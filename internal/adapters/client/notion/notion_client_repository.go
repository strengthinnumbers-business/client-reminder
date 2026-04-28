package notion

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

type APIClient interface {
	FindDataSourceIDByTitle(ctx context.Context, title string) (string, error)
	QueryDataSource(ctx context.Context, dataSourceID string, query notionapi.QueryDataSourceRequest) ([]notionapi.Page, error)
}

type FieldMapping struct {
	ID           string
	Name         string
	PeriodType   string
	ReminderGaps string
	Region       string
	Email        string
	EmailStyle   string
	Greeting     string
	FolderURL    string
	UploadPrompt string
	Status       string
}

type ClientRepository struct {
	api            APIClient
	dataSourceID   string
	dataSourceName string
	fields         FieldMapping
}

func New(api APIClient, dataSourceID string, fields FieldMapping) *ClientRepository {
	return &ClientRepository{
		api:          api,
		dataSourceID: dataSourceID,
		fields:       fields.withDefaults(),
	}
}

func NewForDataSourceName(api APIClient, dataSourceName string, fields FieldMapping) *ClientRepository {
	return &ClientRepository{
		api:            api,
		dataSourceName: dataSourceName,
		fields:         fields.withDefaults(),
	}
}

func (r *ClientRepository) GetAllClients() ([]entities.Client, error) {
	dataSourceID, err := r.resolveDataSourceID(context.Background())
	if err != nil {
		return nil, err
	}

	pages, err := r.api.QueryDataSource(context.Background(), dataSourceID, notionapi.QueryDataSourceRequest{
		Filter: map[string]any{
			"property": r.fields.Status,
			"select": map[string]string{
				"equals": "active",
			},
		},
		FilterProperties: r.fields.filterProperties(),
	})
	if err != nil {
		return nil, fmt.Errorf("query active Notion clients: %w", err)
	}

	clients := make([]entities.Client, 0, len(pages))
	for _, page := range pages {
		client, err := r.clientFromPage(page)
		if err != nil {
			return nil, fmt.Errorf("map Notion client page %s: %w", page.ID, err)
		}
		clients = append(clients, client)
	}

	return clients, nil
}

func (r *ClientRepository) resolveDataSourceID(ctx context.Context) (string, error) {
	if r.dataSourceID != "" {
		return r.dataSourceID, nil
	}
	if r.dataSourceName == "" {
		return "", fmt.Errorf("Notion data source ID or name is required")
	}
	id, err := r.api.FindDataSourceIDByTitle(ctx, r.dataSourceName)
	if err != nil {
		return "", fmt.Errorf("resolve Notion data source %q: %w", r.dataSourceName, err)
	}
	r.dataSourceID = id
	return id, nil
}

func (r *ClientRepository) clientFromPage(page notionapi.Page) (entities.Client, error) {
	periodType, err := parsePeriodType(propertyText(page.Properties, r.fields.PeriodType))
	if err != nil {
		return entities.Client{}, fmt.Errorf("%s: %w", r.fields.PeriodType, err)
	}

	gaps, err := parseReminderGaps(propertyText(page.Properties, r.fields.ReminderGaps))
	if err != nil {
		return entities.Client{}, fmt.Errorf("%s: %w", r.fields.ReminderGaps, err)
	}
	if len(gaps) == 0 {
		gaps = entities.ReminderGapsStandard
	}

	return entities.Client{
		ID:           page.ID,
		Name:         propertyText(page.Properties, r.fields.Name),
		PeriodType:   periodType,
		ReminderGaps: gaps,
		Region:       entities.ClientRegion(propertyText(page.Properties, r.fields.Region)),
		Email:        propertyText(page.Properties, r.fields.Email),
		EmailStyle:   propertyText(page.Properties, r.fields.EmailStyle),
		Greeting:     propertyText(page.Properties, r.fields.Greeting),
		FolderURL:    propertyText(page.Properties, r.fields.FolderURL),
		UploadPrompt: propertyText(page.Properties, r.fields.UploadPrompt),
	}, nil
}

func (m FieldMapping) withDefaults() FieldMapping {
	if m.ID == "" {
		m.ID = "ID"
	}
	if m.Name == "" {
		m.Name = "Name"
	}
	if m.PeriodType == "" {
		m.PeriodType = "Period Type"
	}
	if m.ReminderGaps == "" {
		m.ReminderGaps = "Schedule Preset"
	}
	if m.Region == "" {
		m.Region = "Region"
	}
	if m.Email == "" {
		m.Email = "Contact Email"
	}
	if m.EmailStyle == "" {
		m.EmailStyle = "Email Style"
	}
	if m.Greeting == "" {
		m.Greeting = "Greeting"
	}
	if m.FolderURL == "" {
		m.FolderURL = "Folder URL"
	}
	if m.UploadPrompt == "" {
		m.UploadPrompt = "Prompt"
	}
	if m.Status == "" {
		m.Status = "Status"
	}
	return m
}

func (m FieldMapping) filterProperties() []string {
	return []string{
		m.Name,
		m.PeriodType,
		m.ReminderGaps,
		m.Region,
		m.Email,
		m.EmailStyle,
		m.Greeting,
		m.FolderURL,
		m.UploadPrompt,
		m.Status,
	}
}

func propertyText(properties map[string]notionapi.Property, name string) string {
	property, ok := properties[name]
	if !ok {
		return ""
	}

	switch property.Type {
	case "title":
		return richText(property.Title)
	case "rich_text":
		return richText(property.RichText)
	case "email":
		return property.Email
	case "url":
		return property.URL
	case "select":
		if property.Select != nil {
			return property.Select.Name
		}
	case "status":
		if property.Status != nil {
			return property.Status.Name
		}
	case "multi_select":
		values := make([]string, 0, len(property.MultiSelect))
		for _, value := range property.MultiSelect {
			values = append(values, value.Name)
		}
		return strings.Join(values, ",")
	case "number":
		if property.Number != nil {
			if *property.Number == float64(int(*property.Number)) {
				return strconv.Itoa(int(*property.Number))
			}
			return strconv.FormatFloat(*property.Number, 'f', -1, 64)
		}
	case "checkbox":
		if property.Checkbox != nil {
			return strconv.FormatBool(*property.Checkbox)
		}
	case "formula":
		if property.Formula != nil {
			return formulaText(*property.Formula)
		}
	}

	return ""
}

func richText(values []notionapi.RichTextValue) string {
	var out strings.Builder
	for _, value := range values {
		out.WriteString(value.PlainText)
	}
	return out.String()
}

func formulaText(value notionapi.FormulaValue) string {
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

func parsePeriodType(value string) (entities.PeriodType, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "0", "weekly", "week":
		return entities.PeriodWeekly, nil
	case "1", "monthly", "month":
		return entities.PeriodMonthly, nil
	case "2", "quarterly", "quarter":
		return entities.PeriodQuarterly, nil
	default:
		return entities.PeriodMonthly, fmt.Errorf("unsupported period type %q", value)
	}
}

func parseReminderGaps(value string) (entities.MinimumBusinessDayGaps, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	if value == "standard" {
		return entities.MinimumBusinessDayGaps{0, 2, 2}, nil
	}

	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t' || r == ' '
	})
	gaps := make(entities.MinimumBusinessDayGaps, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		gap, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("parse gap %q: %w", part, err)
		}
		gaps = append(gaps, gap)
	}
	return gaps, nil
}
