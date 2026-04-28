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

type SearchResult struct {
	Object string          `json:"object"`
	ID     string          `json:"id"`
	Title  []RichTextValue `json:"title"`
}

type Page struct {
	Object     string              `json:"object"`
	ID         string              `json:"id"`
	Properties map[string]Property `json:"properties"`
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
	Raw         map[string]any  `json:"-"`
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
