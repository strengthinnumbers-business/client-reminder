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
	CreatePage(ctx context.Context, request notionapi.CreatePageRequest) (notionapi.Page, error)
	UpdatePageSelect(ctx context.Context, pageID string, request notionapi.UpdatePageSelectRequest) (notionapi.Page, error)
	UpdatePageInTrash(ctx context.Context, pageID string, inTrash bool) (notionapi.Page, error)
}

type FieldMapping struct {
	Title          string
	PeriodKey      string
	ReminderClient string
	Status         string
	ChangesSummary string
	VerdictReason  string
}

type verdictRecord struct {
	task entities.CompletionVerdictTask
}

type verdictMap map[string][]verdictRecord

type CompletionDecider struct {
	api            APIClient
	dataSourceID   string
	dataSourceName string
	fields         FieldMapping
	logger         ports.Logger

	mu      sync.Mutex
	loaded  bool
	records verdictMap
}

type Option func(*CompletionDecider)

func New(api APIClient, dataSourceID string, fields FieldMapping, options ...Option) *CompletionDecider {
	d := &CompletionDecider{
		api:          api,
		dataSourceID: dataSourceID,
		fields:       fields.withDefaults(),
		logger:       ports.NoopLogger{},
	}
	for _, option := range options {
		option(d)
	}
	d.logger = ports.EnsureLogger(d.logger)
	return d
}

func NewForDataSourceName(api APIClient, dataSourceName string, fields FieldMapping, options ...Option) *CompletionDecider {
	d := &CompletionDecider{
		api:            api,
		dataSourceName: dataSourceName,
		fields:         fields.withDefaults(),
		logger:         ports.NoopLogger{},
	}
	for _, option := range options {
		option(d)
	}
	d.logger = ports.EnsureLogger(d.logger)
	return d
}

func WithLogger(logger ports.Logger) Option {
	return func(d *CompletionDecider) {
		d.logger = ports.EnsureLogger(logger)
	}
}

func (d *CompletionDecider) GetVerdict(c entities.Client, p entities.Period) (entities.CompletionVerdictTask, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.ensureLoaded(context.Background()); err != nil {
		return entities.CompletionVerdictTask{Status: entities.CompletionUndecided}, err
	}

	records := d.records[stateKey(c.ID, p.ID)]
	if len(records) == 0 {
		d.logger.Demo("no Notion upload review task found", "client_id", c.ID, "client_name", c.Name, "period", p.ID, "verdict", "not_requested")
		return entities.CompletionVerdictTask{Status: entities.CompletionVerdictNotRequested}, nil
	}
	if len(records) > 1 {
		return entities.CompletionVerdictTask{}, fmt.Errorf("multiple Notion completion tasks found for client %s period %s", c.ID, p.ID)
	}

	task := records[0].task
	d.logger.Demo("matched Notion upload review task", "client_id", c.ID, "client_name", c.Name, "period", p.ID, "task_page_id", task.ID, "verdict", verdictName(task.Status))
	return task, nil
}

func (d *CompletionDecider) RequestNewCompletionVerdict(c entities.Client, p entities.Period, changesSummary string) (entities.CompletionVerdictTask, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	ctx := context.Background()

	if err := d.ensureLoaded(ctx); err != nil {
		return entities.CompletionVerdictTask{}, err
	}

	key := stateKey(c.ID, p.ID)
	for _, record := range d.records[key] {
		d.logger.Demo("trashing existing Notion upload review task", "client_id", c.ID, "client_name", c.Name, "period", p.ID, "task_page_id", record.task.ID)
		if _, err := d.api.UpdatePageInTrash(ctx, record.task.ID, true); err != nil {
			return entities.CompletionVerdictTask{}, fmt.Errorf("trash Notion completion task for client %s period %s page %s: %w", c.ID, p.ID, record.task.ID, err)
		}
	}

	dataSourceID, err := d.resolveDataSourceID(ctx)
	if err != nil {
		return entities.CompletionVerdictTask{}, err
	}

	d.logger.Demo("creating new Notion upload review task", "client_id", c.ID, "client_name", c.Name, "period", p.ID, "data_source_id", dataSourceID)
	page, err := d.api.CreatePage(ctx, notionapi.CreatePageRequest{
		DataSourceID: dataSourceID,
		Properties: notionapi.PagePropertyUpdates{
			d.fields.Title:          notionapi.TitleProperty(completionTaskTitle(c, p)),
			d.fields.PeriodKey:      notionapi.RichTextProperty(p.ID),
			d.fields.ReminderClient: notionapi.RelationProperty(c.ID),
			d.fields.Status:         notionapi.SelectProperty("undecided"),
			d.fields.ChangesSummary: notionapi.RichTextProperty(changesSummary),
			d.fields.VerdictReason:  notionapi.RichTextProperty(""),
		},
	})
	if err != nil {
		return entities.CompletionVerdictTask{}, fmt.Errorf("create Notion completion task for client %s period %s: %w", c.ID, p.ID, err)
	}

	task := entities.CompletionVerdictTask{
		ID:             page.ID,
		Status:         entities.CompletionVerdictNotRequested,
		ChangesSummary: changesSummary,
	}
	d.records[key] = []verdictRecord{{task: task}}
	d.logger.Demo("created new Notion upload review task", "client_id", c.ID, "client_name", c.Name, "period", p.ID, "task_page_id", task.ID, "verdict", "not_requested")
	return task, nil
}

func (d *CompletionDecider) ensureLoaded(ctx context.Context) error {
	if d.loaded {
		d.logger.Demo("using cached Notion upload review task snapshot", "records", len(d.records))
		return nil
	}

	dataSourceID, err := d.resolveDataSourceID(ctx)
	if err != nil {
		return err
	}

	d.logger.Demo("querying upload review tasks from Notion", "data_source_id", dataSourceID)
	pages, err := d.api.QueryDataSource(ctx, dataSourceID, notionapi.QueryDataSourceRequest{
		FilterProperties: d.fields.filterProperties(),
	})
	if err != nil {
		return fmt.Errorf("query Notion completion tasks: %w", err)
	}
	d.logger.Demo("Notion returned upload review task pages", "data_source_id", dataSourceID, "count", len(pages))

	records, err := d.recordsFromPages(pages)
	if err != nil {
		return err
	}

	d.records = records
	d.loaded = true
	d.logger.Demo("cached upload review task snapshot for this run", "records", len(records))
	return nil
}

func (d *CompletionDecider) resolveDataSourceID(ctx context.Context) (string, error) {
	if d.dataSourceID != "" {
		d.logger.Demo("using configured Notion upload review task data source", "data_source_id", d.dataSourceID)
		return d.dataSourceID, nil
	}
	if d.dataSourceName == "" {
		return "", fmt.Errorf("Notion completion data source ID or name is required")
	}
	d.logger.Demo("resolving Notion upload review task data source by title", "title", d.dataSourceName)
	id, err := d.api.FindDataSourceIDByTitle(ctx, d.dataSourceName)
	if err != nil {
		return "", fmt.Errorf("resolve Notion completion data source %q: %w", d.dataSourceName, err)
	}
	d.dataSourceID = id
	d.logger.Demo("resolved Notion upload review task data source", "title", d.dataSourceName, "data_source_id", id)
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
		task := entities.CompletionVerdictTask{
			ID:             page.ID,
			Status:         verdict,
			ChangesSummary: page.Properties.Text(d.fields.ChangesSummary),
			VerdictReason:  page.Properties.Text(d.fields.VerdictReason),
		}
		records[key] = append(records[key], verdictRecord{task: task})
		d.logger.Demo("mapped Notion upload review task page", "page_id", page.ID, "client_id", clientID, "period", periodKey, "status", page.Properties.Text(d.fields.Status), "verdict", verdictName(verdict))
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
	if m.ChangesSummary == "" {
		m.ChangesSummary = "Changes Summary"
	}
	if m.VerdictReason == "" {
		m.VerdictReason = "Verdict Reason"
	}
	return m
}

func (m FieldMapping) filterProperties() []string {
	return []string{
		m.Title,
		m.PeriodKey,
		m.ReminderClient,
		m.Status,
		m.ChangesSummary,
		m.VerdictReason,
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

func verdictFromStatus(status string) (entities.CompletionVerdictStatus, error) {
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

func completionTaskTitle(c entities.Client, p entities.Period) string {
	if c.Name == "" {
		return p.ID
	}
	return c.Name + " " + p.ID
}

func verdictName(verdict entities.CompletionVerdictStatus) string {
	switch verdict {
	case entities.CompletionVerdictNotRequested:
		return "not_requested"
	case entities.CompletionUndecided:
		return "undecided"
	case entities.CompletionIncomplete:
		return "upload_incomplete"
	case entities.CompletionComplete:
		return "upload_complete"
	default:
		return fmt.Sprintf("unknown_%d", verdict)
	}
}
