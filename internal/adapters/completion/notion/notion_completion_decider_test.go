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

	verdict, err := decider.IsCompleted(client, entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"})
	if err != nil {
		t.Fatalf("IsCompleted returned error: %v", err)
	}
	if verdict != entities.CompletionComplete {
		t.Fatalf("expected CompletionComplete, got %v", verdict)
	}

	verdict, err = decider.IsCompleted(client, entities.Period{Type: entities.PeriodMonthly, ID: "2026-05"})
	if err != nil {
		t.Fatalf("IsCompleted returned error: %v", err)
	}
	if verdict != entities.CompletionUndecided {
		t.Fatalf("expected CompletionUndecided, got %v", verdict)
	}
	if api.queryCalls != 1 {
		t.Fatalf("expected one cached query, got %d", api.queryCalls)
	}

	wantFilterProperties := []string{"Title", "Period Key", "Reminder Client", "Status"}
	if !reflect.DeepEqual(api.query.FilterProperties, wantFilterProperties) {
		t.Fatalf("unexpected filter properties: got %#v want %#v", api.query.FilterProperties, wantFilterProperties)
	}
}

func TestCompletionDeciderMissingVerdictDefaultsToNotRequested(t *testing.T) {
	decider := New(&fakeAPI{}, "tasks-ds", FieldMapping{})

	verdict, err := decider.IsCompleted(
		entities.Client{ID: "client-page-1"},
		entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"},
	)
	if err != nil {
		t.Fatalf("IsCompleted returned error: %v", err)
	}
	if verdict != entities.CompletionVerdictNotRequested {
		t.Fatalf("expected CompletionVerdictNotRequested, got %v", verdict)
	}
}

func TestCompletionDeciderResetUpdatesNotionAndCache(t *testing.T) {
	api := &fakeAPI{
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

	if err := decider.ResetCompletionVerdict(client, period); err != nil {
		t.Fatalf("ResetCompletionVerdict returned error: %v", err)
	}

	if api.updatedPageID != "task-page-1" {
		t.Fatalf("expected update for task-page-1, got %q", api.updatedPageID)
	}
	if got, want := api.updateRequest.PropertyName, "Status"; got != want {
		t.Fatalf("unexpected updated property: got %q want %q", got, want)
	}
	if got, want := api.updateRequest.SelectName, "unset"; got != want {
		t.Fatalf("unexpected updated select: got %q want %q", got, want)
	}

	verdict, err := decider.IsCompleted(client, period)
	if err != nil {
		t.Fatalf("IsCompleted returned error: %v", err)
	}
	if verdict != entities.CompletionVerdictNotRequested {
		t.Fatalf("expected cached CompletionVerdictNotRequested, got %v", verdict)
	}
	if api.queryCalls != 1 {
		t.Fatalf("expected reset to update cache without requerying, got %d queries", api.queryCalls)
	}
}

func TestCompletionDeciderResolvesDataSourceName(t *testing.T) {
	api := &fakeAPI{dataSourceID: "resolved-ds"}

	_, err := NewForDataSourceName(api, "Test Upload Review Tasks", FieldMapping{}).IsCompleted(
		entities.Client{ID: "client-page-1"},
		entities.Period{Type: entities.PeriodMonthly, ID: "2026-04"},
	)
	if err != nil {
		t.Fatalf("IsCompleted returned error: %v", err)
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

	updatedPageID string
	updateRequest notionapi.UpdatePageSelectRequest
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
