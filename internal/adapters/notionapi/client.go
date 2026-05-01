package notionapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	APIKeyEnvVar         = "NOTION_API_KEY"
	DefaultBaseURL       = "https://api.notion.com/v1"
	DefaultNotionVersion = "2026-03-11"
	defaultRequestGap    = 333 * time.Millisecond
	defaultPageSize      = 100
)

type Client struct {
	apiKey        string
	baseURL       string
	notionVersion string
	httpClient    *http.Client
	requestGap    time.Duration

	mu              sync.Mutex
	previousCallEnd time.Time
}

type Option func(*Client)

type QueryDataSourceRequest struct {
	Filter           any
	Sorts            []any
	FilterProperties []string
	PageSize         int
}

type RetrievePageRequest struct {
	FilterProperties []string
}

type CreatePageRequest struct {
	DataSourceID string
	Properties   PagePropertyUpdates
}

type UpdatePageSelectRequest struct {
	PropertyName string
	SelectName   string
}

type SearchResult struct {
	Object string          `json:"object"`
	ID     string          `json:"id"`
	Title  []RichTextValue `json:"title"`
}

type Page struct {
	Object         string     `json:"object"`
	ID             string     `json:"id"`
	Properties     Properties `json:"properties"`
	CreatedTime    string     `json:"created_time"`
	LastEditedTime string     `json:"last_edited_time"`
}

type Properties map[string]Property

func (p Properties) Text(name string) string {
	property, ok := p[name]
	if ok {
		return property.Text(name)
	}

	return ""
}

type Property struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Title       []RichTextValue `json:"title"`
	RichText    []RichTextValue `json:"rich_text"`
	Email       string          `json:"email"`
	URL         string          `json:"url"`
	Number      *float64        `json:"number"`
	Select      *NamedValue     `json:"select"`
	Status      *NamedValue     `json:"status"`
	MultiSelect []NamedValue    `json:"multi_select"`
	Checkbox    *bool           `json:"checkbox"`
	Formula     *FormulaValue   `json:"formula"`
	People      []User          `json:"people"`
	Relation    []PageReference `json:"relation"`
	HasMore     bool            `json:"has_more"`
	Raw         map[string]any  `json:"-"`
}

func (p Property) Text(name string) string {
	switch p.Type {
	case "title":
		return plainText(p.Title)
	case "rich_text":
		return plainText(p.RichText)
	case "email":
		return p.Email
	case "url":
		return p.URL
	case "select":
		if p.Select != nil {
			return p.Select.Name
		}
	case "status":
		if p.Status != nil {
			return p.Status.Name
		}
	case "multi_select":
		values := make([]string, 0, len(p.MultiSelect))
		for _, value := range p.MultiSelect {
			values = append(values, value.Name)
		}
		return strings.Join(values, ",")
	case "number":
		if p.Number != nil {
			if *p.Number == float64(int(*p.Number)) {
				return strconv.Itoa(int(*p.Number))
			}
			return strconv.FormatFloat(*p.Number, 'f', -1, 64)
		}
	case "checkbox":
		if p.Checkbox != nil {
			return strconv.FormatBool(*p.Checkbox)
		}
	case "formula":
		if p.Formula != nil {
			return formulaText(*p.Formula)
		}
	}

	return ""
}

type RichTextValue struct {
	PlainText string `json:"plain_text"`
}

type NamedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FormulaValue struct {
	Type   string      `json:"type"`
	String string      `json:"string"`
	Number *float64    `json:"number"`
	Bool   *bool       `json:"boolean"`
	Raw    interface{} `json:"-"`
}

type User struct {
	Object    string       `json:"object"`
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	AvatarURL string       `json:"avatar_url"`
	Type      string       `json:"type"`
	Person    *PersonValue `json:"person"`
}

type PersonValue struct {
	Email string `json:"email"`
}

type PageReference struct {
	ID string `json:"id"`
}

type PagePropertyUpdates map[string]PagePropertyUpdate

type PagePropertyUpdate struct {
	Title    []RichTextInput `json:"title,omitempty"`
	RichText []RichTextInput `json:"rich_text,omitempty"`
	Relation []PageReference `json:"relation,omitempty"`
	Select   *SelectInput    `json:"select,omitempty"`
}

type RichTextInput struct {
	Text TextInput `json:"text"`
}

type TextInput struct {
	Content string `json:"content"`
}

type SelectInput struct {
	Name string `json:"name,omitempty"`
	ID   string `json:"id,omitempty"`
}

func TitleProperty(content string) PagePropertyUpdate {
	return PagePropertyUpdate{Title: []RichTextInput{{Text: TextInput{Content: content}}}}
}

func RichTextProperty(content string) PagePropertyUpdate {
	return PagePropertyUpdate{RichText: []RichTextInput{{Text: TextInput{Content: content}}}}
}

func RelationProperty(pageIDs ...string) PagePropertyUpdate {
	references := make([]PageReference, 0, len(pageIDs))
	for _, pageID := range pageIDs {
		references = append(references, PageReference{ID: pageID})
	}
	return PagePropertyUpdate{Relation: references}
}

func SelectProperty(name string) PagePropertyUpdate {
	return PagePropertyUpdate{Select: &SelectInput{Name: name}}
}

type listResponse[T any] struct {
	Object     string `json:"object"`
	Results    []T    `json:"results"`
	NextCursor string `json:"next_cursor"`
	HasMore    bool   `json:"has_more"`
}

func NewFromEnv(options ...Option) (*Client, error) {
	apiKey := os.Getenv(APIKeyEnvVar)
	if apiKey == "" {
		return nil, fmt.Errorf("%s is not set", APIKeyEnvVar)
	}
	return New(apiKey, options...), nil
}

func New(apiKey string, options ...Option) *Client {
	c := &Client{
		apiKey:        apiKey,
		baseURL:       DefaultBaseURL,
		notionVersion: DefaultNotionVersion,
		httpClient:    &http.Client{Timeout: 15 * time.Second},
		requestGap:    defaultRequestGap,
	}
	for _, option := range options {
		option(c)
	}
	return c
}

func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

func WithRequestGap(gap time.Duration) Option {
	return func(c *Client) {
		if gap >= 0 {
			c.requestGap = gap
		}
	}
}

func WithNotionVersion(version string) Option {
	return func(c *Client) {
		if version != "" {
			c.notionVersion = version
		}
	}
}

func (c *Client) FindDataSourceIDByTitle(ctx context.Context, title string) (string, error) {
	var cursor string
	for {
		body := map[string]any{
			"query":     title,
			"page_size": defaultPageSize,
			"filter": map[string]string{
				"property": "object",
				"value":    "data_source",
			},
		}
		if cursor != "" {
			body["start_cursor"] = cursor
		}

		var payload listResponse[SearchResult]
		if err := c.doJSON(ctx, http.MethodPost, "/search", nil, body, &payload); err != nil {
			return "", fmt.Errorf("search Notion data source %q: %w", title, err)
		}

		for _, result := range payload.Results {
			if result.Object == "data_source" && plainText(result.Title) == title {
				return result.ID, nil
			}
		}
		if !payload.HasMore {
			return "", fmt.Errorf("Notion data source %q not found", title)
		}
		cursor = payload.NextCursor
	}
}

func (c *Client) QueryDataSource(ctx context.Context, dataSourceID string, query QueryDataSourceRequest) ([]Page, error) {
	var pages []Page
	var cursor string
	for {
		body := map[string]any{
			"page_size": effectivePageSize(query.PageSize),
		}
		if query.Filter != nil {
			body["filter"] = query.Filter
		}
		if len(query.Sorts) > 0 {
			body["sorts"] = query.Sorts
		}
		if cursor != "" {
			body["start_cursor"] = cursor
		}

		queryParams := url.Values{}
		for _, property := range query.FilterProperties {
			queryParams.Add("filter_properties[]", property)
		}

		var payload listResponse[Page]
		path := fmt.Sprintf("/data_sources/%s/query", url.PathEscape(dataSourceID))
		if err := c.doJSON(ctx, http.MethodPost, path, queryParams, body, &payload); err != nil {
			return nil, fmt.Errorf("query Notion data source %s: %w", dataSourceID, err)
		}

		pages = append(pages, payload.Results...)
		if !payload.HasMore {
			return pages, nil
		}
		cursor = payload.NextCursor
	}
}

func (c *Client) RetrievePage(ctx context.Context, pageID string, request RetrievePageRequest) (Page, error) {
	queryParams := url.Values{}
	for _, property := range request.FilterProperties {
		queryParams.Add("filter_properties[]", property)
	}

	var page Page
	path := fmt.Sprintf("/pages/%s", url.PathEscape(pageID))
	if err := c.doJSON(ctx, http.MethodGet, path, queryParams, nil, &page); err != nil {
		return Page{}, fmt.Errorf("retrieve Notion page %s: %w", pageID, err)
	}
	return page, nil
}

func (c *Client) CreatePage(ctx context.Context, request CreatePageRequest) (Page, error) {
	body := map[string]any{
		"parent": map[string]string{
			"type":           "data_source_id",
			"data_source_id": request.DataSourceID,
		},
		"properties": request.Properties,
	}

	var page Page
	if err := c.doJSON(ctx, http.MethodPost, "/pages", nil, body, &page); err != nil {
		return Page{}, fmt.Errorf("create Notion page in data source %s: %w", request.DataSourceID, err)
	}
	return page, nil
}

func (c *Client) UpdatePageSelect(ctx context.Context, pageID string, request UpdatePageSelectRequest) (Page, error) {
	body := map[string]any{
		"properties": PagePropertyUpdates{
			request.PropertyName: SelectProperty(request.SelectName),
		},
	}

	var page Page
	path := fmt.Sprintf("/pages/%s", url.PathEscape(pageID))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, body, &page); err != nil {
		return Page{}, fmt.Errorf("update Notion page %s select property %q: %w", pageID, request.PropertyName, err)
	}
	return page, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, requestBody any, responseBody any) error {
	var requestBytes []byte
	if requestBody != nil {
		payload, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("encode Notion request: %w", err)
		}
		requestBytes = payload
	}

	endpoint, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("build Notion URL: %w", err)
	}
	if len(query) > 0 {
		endpoint.RawQuery = query.Encode()
	}

	for attempt := 0; attempt < 2; attempt++ {
		var body io.Reader
		if requestBytes != nil {
			body = bytes.NewReader(requestBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
		if err != nil {
			return fmt.Errorf("build Notion request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Notion-Version", c.notionVersion)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		if err := c.waitForRateLimit(ctx); err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			c.recordCallEnd()
			return fmt.Errorf("call Notion API: %w", err)
		}

		if resp.StatusCode == http.StatusTooManyRequests && attempt == 0 {
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			c.recordCallEnd()
			if err := sleepContext(ctx, retryAfter); err != nil {
				return err
			}
			continue
		}

		responseBytes, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		c.recordCallEnd()
		if readErr != nil {
			return fmt.Errorf("read Notion response: %w", readErr)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("unexpected Notion status %s: %s", resp.Status, string(responseBytes))
		}
		if responseBody == nil {
			return nil
		}
		if err := json.Unmarshal(responseBytes, responseBody); err != nil {
			return fmt.Errorf("decode Notion response: %w", err)
		}
		return nil
	}

	return errors.New("Notion request retry loop exhausted")
}

func (c *Client) waitForRateLimit(ctx context.Context) error {
	c.mu.Lock()
	wait := time.Duration(0)
	if !c.previousCallEnd.IsZero() {
		nextAllowed := c.previousCallEnd.Add(c.requestGap)
		wait = time.Until(nextAllowed)
	}
	c.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	return sleepContext(ctx, wait)
}

func (c *Client) recordCallEnd() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.previousCallEnd = time.Now()
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseRetryAfter(value string) time.Duration {
	t, err := http.ParseTime(value)
	if err == nil {
		return time.Until(t)
	}
	seconds, err := strconv.Atoi(value)
	if err != nil || seconds < 0 {
		return time.Second
	}
	return time.Duration(seconds) * time.Second
}

func effectivePageSize(pageSize int) int {
	if pageSize <= 0 || pageSize > defaultPageSize {
		return defaultPageSize
	}
	return pageSize
}

func plainText(values []RichTextValue) string {
	var out string
	for _, value := range values {
		out += value.PlainText
	}
	return out
}

func formulaText(value FormulaValue) string {
	switch value.Type {
	case "string":
		return value.String
	case "number":
		if value.Number != nil {
			return strconv.FormatFloat(*value.Number, 'f', -1, 64)
		}
	case "boolean":
		if value.Bool != nil {
			return strconv.FormatBool(*value.Bool)
		}
	}
	return ""
}
