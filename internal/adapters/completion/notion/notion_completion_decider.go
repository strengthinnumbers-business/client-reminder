package notion

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

var _ ports.CompletionDecider = (*CompletionDecider)(nil)

type APIClient interface {
	FindDataSourceIDByTitle(ctx context.Context, title string) (string, error)
	QueryDataSource(ctx context.Context, dataSourceID string, query notionapi.QueryDataSourceRequest) ([]notionapi.Page, error)
	UpdatePageSelect(ctx context.Context, pageID string, request notionapi.UpdatePageSelectRequest) (notionapi.Page, error)
}

type FieldMapping struct {
	Title          string
	PeriodKey      string
	ReminderClient string
	Status         string
}

type verdictRecord struct {
	pageID  string
	verdict entities.CompletionVerdict
}

type verdictMap map[string]verdictRecord

type CompletionDecider struct {
	api            APIClient
	dataSourceID   string
	dataSourceName string
	fields         FieldMapping

	mu      sync.Mutex
	loaded  bool
	records verdictMap
}

func New(api APIClient, dataSourceID string, fields FieldMapping) *CompletionDecider {
	return &CompletionDecider{
		api:          api,
		dataSourceID: dataSourceID,
		fields:       fields.withDefaults(),
	}
}

func NewForDataSourceName(api APIClient, dataSourceName string, fields FieldMapping) *CompletionDecider {
	return &CompletionDecider{
		api:            api,
		dataSourceName: dataSourceName,
		fields:         fields.withDefaults(),
	}
}

func (d *CompletionDecider) IsCompleted(c entities.Client, p entities.Period) (entities.CompletionVerdict, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureLoaded(context.Background()); err != nil {
		return entities.CompletionUndecided, err
	}

	record, ok := d.records[stateKey(c.ID, p.ID)]
	if !ok {
		return entities.CompletionVerdictNotRequested, nil
	}

	return record.verdict, nil
}

func (d *CompletionDecider) ResetCompletionVerdict(c entities.Client, p entities.Period) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureLoaded(context.Background()); err != nil {
		return err
	}

	key := stateKey(c.ID, p.ID)
	record, ok := d.records[key]
	if !ok {
		return fmt.Errorf("Notion completion task not found for client %s period %s", c.ID, p.ID)
	}

	_, err := d.api.UpdatePageSelect(context.Background(), record.pageID, notionapi.UpdatePageSelectRequest{
		PropertyName: d.fields.Status,
		SelectName:   "unset",
	})
	if err != nil {
		return fmt.Errorf("reset Notion completion verdict for client %s period %s page %s: %w", c.ID, p.ID, record.pageID, err)
	}

	record.verdict = entities.CompletionVerdictNotRequested
	d.records[key] = record
	return nil
}

func (d *CompletionDecider) ensureLoaded(ctx context.Context) error {
	if d.loaded {
		return nil
	}

	dataSourceID, err := d.resolveDataSourceID(ctx)
	if err != nil {
		return err
	}

	pages, err := d.api.QueryDataSource(ctx, dataSourceID, notionapi.QueryDataSourceRequest{
		FilterProperties: d.fields.filterProperties(),
	})
	if err != nil {
		return fmt.Errorf("query Notion completion tasks: %w", err)
	}

	records, err := d.recordsFromPages(pages)
	if err != nil {
		return err
	}

	d.records = records
	d.loaded = true
	return nil
}

func (d *CompletionDecider) resolveDataSourceID(ctx context.Context) (string, error) {
	if d.dataSourceID != "" {
		return d.dataSourceID, nil
	}
	if d.dataSourceName == "" {
		return "", fmt.Errorf("Notion completion data source ID or name is required")
	}
	id, err := d.api.FindDataSourceIDByTitle(ctx, d.dataSourceName)
	if err != nil {
		return "", fmt.Errorf("resolve Notion completion data source %q: %w", d.dataSourceName, err)
	}
	d.dataSourceID = id
	return id, nil
}

func (d *CompletionDecider) recordsFromPages(pages []notionapi.Page) (verdictMap, error) {
	records := make(verdictMap, len(pages))
	for _, page := range pages {
		periodKey := page.Properties.Text(d.fields.PeriodKey)
		if periodKey == "" {
			return nil, fmt.Errorf("map Notion completion task page %s: missing %q", page.ID, d.fields.PeriodKey)
		}

		clientID, err := clientIDFromRelation(page.Properties[d.fields.ReminderClient])
		if err != nil {
			return nil, fmt.Errorf("map Notion completion task page %s: %s: %w", page.ID, d.fields.ReminderClient, err)
		}

		verdict, err := verdictFromStatus(page.Properties.Text(d.fields.Status))
		if err != nil {
			return nil, fmt.Errorf("map Notion completion task page %s: %s: %w", page.ID, d.fields.Status, err)
		}

		key := stateKey(clientID, periodKey)
		if existing, ok := records[key]; ok {
			return nil, fmt.Errorf("map Notion completion task page %s: duplicate task for client %s period %s already mapped to page %s", page.ID, clientID, periodKey, existing.pageID)
		}

		records[key] = verdictRecord{
			pageID:  page.ID,
			verdict: verdict,
		}
	}
	return records, nil
}

func (m FieldMapping) withDefaults() FieldMapping {
	if m.Title == "" {
		m.Title = "Title"
	}
	if m.PeriodKey == "" {
		m.PeriodKey = "Period Key"
	}
	if m.ReminderClient == "" {
		m.ReminderClient = "Reminder Client"
	}
	if m.Status == "" {
		m.Status = "Status"
	}
	return m
}

func (m FieldMapping) filterProperties() []string {
	return []string{
		m.Title,
		m.PeriodKey,
		m.ReminderClient,
		m.Status,
	}
}

func clientIDFromRelation(property notionapi.Property) (string, error) {
	if property.Type != "relation" {
		return "", fmt.Errorf("expected relation property, got %q", property.Type)
	}
	if property.HasMore {
		return "", fmt.Errorf("relation has more than the supported single page reference")
	}
	if len(property.Relation) != 1 {
		return "", fmt.Errorf("expected one related client page, got %d", len(property.Relation))
	}
	if property.Relation[0].ID == "" {
		return "", fmt.Errorf("related client page ID is empty")
	}
	return property.Relation[0].ID, nil
}

func verdictFromStatus(status string) (entities.CompletionVerdict, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "unset":
		return entities.CompletionVerdictNotRequested, nil
	case "undecided":
		return entities.CompletionUndecided, nil
	case "upload_incomplete":
		return entities.CompletionIncomplete, nil
	case "upload_complete":
		return entities.CompletionComplete, nil
	default:
		return entities.CompletionVerdictNotRequested, fmt.Errorf("unsupported status %q", status)
	}
}

func stateKey(customerID, periodID string) string {
	return customerID + "::" + periodID
}
