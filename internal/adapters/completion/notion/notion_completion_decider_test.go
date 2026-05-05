package notion

import (
	"context"
	"reflect"
	"testing"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestCompletionDeciderQueriesOnceAndMapsVerdicts(t *testing.T) {
	api := &fakeAPI{
		pages: []notionapi.Page{
			{
				ID: "task-page-1",
				Properties: notionapi.Properties{
					"Period Key":      richTextProperty("2026-04"),
					"Reminder Client": relationProperty("client-page-1"),
					"Status":          selectProperty("upload_complete"),
					"Changes Summary": richTextProperty("changed files"),
					"Verdict Reason":  richTextProperty("all present"),
				},
			},
			{
				ID: "task-page-2",
				Properties: notionapi.Properties{
					"Period Key":      richTextProperty("2026-05"),
					"Reminder Client": relationProperty("client-page-1"),
					"Status":          selectProperty("undecided"),
				},
			},
		},
	}

	decider := New(api, "tasks-ds", FieldMapping{})
	client := entities.Client{ID: "client-page-1"}

	task, err := decider.GetVerdict(client, entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"})
	if err != nil {
		t.Fatalf("GetVerdict returned error: %v", err)
	}
	if task.Status != entities.CompletionComplete {
		t.Fatalf("expected CompletionComplete, got %v", task.Status)
	}
	if task.ChangesSummary != "changed files" || task.VerdictReason != "all present" {
		t.Fatalf("unexpected task details: %#v", task)
	}

	task, err = decider.GetVerdict(client, entities.Period{Type: entities.PeriodMonthly, ID: "2026-05"})
	if err != nil {
		t.Fatalf("GetVerdict returned error: %v", err)
	}
	if task.Status != entities.CompletionUndecided {
		t.Fatalf("expected CompletionUndecided, got %v", task.Status)
	}
	if api.queryCalls != 1 {
		t.Fatalf("expected one cached query, got %d", api.queryCalls)
	}

	wantFilterProperties := []string{"Title", "Period Key", "Reminder Client", "Status", "Changes Summary", "Verdict Reason"}
	if !reflect.DeepEqual(api.query.FilterProperties, wantFilterProperties) {
		t.Fatalf("unexpected filter properties: got %#v want %#v", api.query.FilterProperties, wantFilterProperties)
	}
}

func TestCompletionDeciderMissingVerdictDefaultsToNotRequested(t *testing.T) {
	decider := New(&fakeAPI{}, "tasks-ds", FieldMapping{})

	task, err := decider.GetVerdict(
		entities.Client{ID: "client-page-1"},
		entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"},
	)
	if err != nil {
		t.Fatalf("GetVerdict returned error: %v", err)
	}
	if task.Status != entities.CompletionVerdictNotRequested {
		t.Fatalf("expected CompletionVerdictNotRequested, got %v", task.Status)
	}
}

func TestCompletionDeciderRequestNewVerdictTrashesExistingTasksAndCreatesNewTask(t *testing.T) {
	api := &fakeAPI{
		createdPageID: "task-page-2",
		pages: []notionapi.Page{
			{
				ID: "task-page-1",
				Properties: notionapi.Properties{
					"Period Key":      richTextProperty("2026-04"),
					"Reminder Client": relationProperty("client-page-1"),
					"Status":          selectProperty("upload_incomplete"),
				},
			},
		},
	}

	decider := New(api, "tasks-ds", FieldMapping{})
	client := entities.Client{ID: "client-page-1"}
	period := entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"}

	task, err := decider.RequestNewCompletionVerdict(client, period, "new uploads")
	if err != nil {
		t.Fatalf("RequestNewCompletionVerdict returned error: %v", err)
	}

	if !reflect.DeepEqual(api.trashedPageIDs, []string{"task-page-1"}) {
		t.Fatalf("unexpected trashed pages: %#v", api.trashedPageIDs)
	}
	if task.ID != "task-page-2" || task.Status != entities.CompletionVerdictNotRequested || task.ChangesSummary != "new uploads" {
		t.Fatalf("unexpected returned task: %#v", task)
	}
	if api.createRequest.DataSourceID != "tasks-ds" {
		t.Fatalf("unexpected create data source: %q", api.createRequest.DataSourceID)
	}
	if got := api.createRequest.Properties["Changes Summary"].RichText[0].Text.Content; got != "new uploads" {
		t.Fatalf("unexpected changes summary: %q", got)
	}
	if got := api.createRequest.Properties["Status"].Select.Name; got != "unset" {
		t.Fatalf("unexpected status: %q", got)
	}

	task, err = decider.GetVerdict(client, period)
	if err != nil {
		t.Fatalf("GetVerdict returned error: %v", err)
	}
	if task.ID != "task-page-2" || task.Status != entities.CompletionVerdictNotRequested {
		t.Fatalf("expected cached new task, got %#v", task)
	}
	if api.queryCalls != 1 {
		t.Fatalf("expected reset to update cache without requerying, got %d queries", api.queryCalls)
	}
}

func TestCompletionDeciderResolvesDataSourceName(t *testing.T) {
	api := &fakeAPI{dataSourceID: "resolved-ds"}

	_, err := NewForDataSourceName(api, "Test Upload Review Tasks", FieldMapping{}).GetVerdict(
		entities.Client{ID: "client-page-1"},
		entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"},
	)
	if err != nil {
		t.Fatalf("GetVerdict returned error: %v", err)
	}
	if api.searchedTitle != "Test Upload Review Tasks" {
		t.Fatalf("expected data source title search, got %q", api.searchedTitle)
	}
	if api.queriedDataSourceID != "resolved-ds" {
		t.Fatalf("expected query against resolved data source, got %q", api.queriedDataSourceID)
	}
}

type fakeAPI struct {
	dataSourceID        string
	searchedTitle       string
	queriedDataSourceID string
	query               notionapi.QueryDataSourceRequest
	queryCalls          int
	pages               []notionapi.Page

	updatedPageID  string
	updateRequest  notionapi.UpdatePageSelectRequest
	trashedPageIDs []string
	createRequest  notionapi.CreatePageRequest
	createdPageID  string
}

func (f *fakeAPI) FindDataSourceIDByTitle(_ context.Context, title string) (string, error) {
	f.searchedTitle = title
	return f.dataSourceID, nil
}

func (f *fakeAPI) QueryDataSource(_ context.Context, dataSourceID string, query notionapi.QueryDataSourceRequest) ([]notionapi.Page, error) {
	f.queriedDataSourceID = dataSourceID
	f.query = query
	f.queryCalls++
	return f.pages, nil
}

func (f *fakeAPI) UpdatePageSelect(_ context.Context, pageID string, request notionapi.UpdatePageSelectRequest) (notionapi.Page, error) {
	f.updatedPageID = pageID
	f.updateRequest = request
	return notionapi.Page{ID: pageID}, nil
}

func (f *fakeAPI) UpdatePageInTrash(_ context.Context, pageID string, inTrash bool) (notionapi.Page, error) {
	if inTrash {
		f.trashedPageIDs = append(f.trashedPageIDs, pageID)
	}
	return notionapi.Page{ID: pageID, InTrash: inTrash}, nil
}

func (f *fakeAPI) CreatePage(_ context.Context, request notionapi.CreatePageRequest) (notionapi.Page, error) {
	f.createRequest = request
	pageID := f.createdPageID
	if pageID == "" {
		pageID = "created-page"
	}
	return notionapi.Page{ID: pageID, Properties: notionapi.Properties{}}, nil
}

func richTextProperty(value string) notionapi.Property {
	return notionapi.Property{
		Type:     "rich_text",
		RichText: []notionapi.RichTextValue{{PlainText: value}},
	}
}

func selectProperty(value string) notionapi.Property {
	return notionapi.Property{
		Type:   "select",
		Select: &notionapi.NamedValue{Name: value},
	}
}

func relationProperty(pageID string) notionapi.Property {
	return notionapi.Property{
		Type:     "relation",
		Relation: []notionapi.PageReference{{ID: pageID}},
	}
}
