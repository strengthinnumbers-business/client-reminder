package notion

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
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
	FolderPath   string
	UploadPrompt string
	Status       string
}

type ClientRepository struct {
	api            APIClient
	dataSourceID   string
	dataSourceName string
	fields         FieldMapping
	logger         ports.Logger
}

type Option func(*ClientRepository)

func New(api APIClient, dataSourceID string, fields FieldMapping, options ...Option) *ClientRepository {
	r := &ClientRepository{
		api:          api,
		dataSourceID: dataSourceID,
		fields:       fields.withDefaults(),
		logger:       ports.NoopLogger{},
	}
	for _, option := range options {
		option(r)
	}
	r.logger = ports.EnsureLogger(r.logger)
	return r
}

func NewForDataSourceName(api APIClient, dataSourceName string, fields FieldMapping, options ...Option) *ClientRepository {
	r := &ClientRepository{
		api:            api,
		dataSourceName: dataSourceName,
		fields:         fields.withDefaults(),
		logger:         ports.NoopLogger{},
	}
	for _, option := range options {
		option(r)
	}
	r.logger = ports.EnsureLogger(r.logger)
	return r
}

func WithLogger(logger ports.Logger) Option {
	return func(r *ClientRepository) {
		r.logger = ports.EnsureLogger(logger)
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
	periodType, err := parsePeriodType(page.Properties.Text(r.fields.PeriodType))
	if err != nil {
		return entities.Client{}, fmt.Errorf("%s: %w", r.fields.PeriodType, err)
	}

	gaps, err := parseReminderGaps(page.Properties.Text(r.fields.ReminderGaps))
	if err != nil {
		return entities.Client{}, fmt.Errorf("%s: %w", r.fields.ReminderGaps, err)
	}
	if len(gaps) == 0 {
		gaps = entities.ReminderGapsStandard
	}

	return entities.Client{
		ID:           page.ID,
		Name:         page.Properties.Text(r.fields.Name),
		PeriodType:   periodType,
		ReminderGaps: gaps,
		Region:       entities.ClientRegion(page.Properties.Text(r.fields.Region)),
		Email:        page.Properties.Text(r.fields.Email),
		EmailStyle:   page.Properties.Text(r.fields.EmailStyle),
		Greeting:     page.Properties.Text(r.fields.Greeting),
		FolderURL:    page.Properties.Text(r.fields.FolderURL),
		FolderPath:   page.Properties.Text(r.fields.FolderPath),
		UploadPrompt: page.Properties.Text(r.fields.UploadPrompt),
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
	if m.FolderPath == "" {
		m.FolderPath = "Folder Path"
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
		m.FolderPath,
		m.UploadPrompt,
		m.Status,
	}
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
