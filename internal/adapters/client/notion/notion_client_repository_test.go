package notion

import (
	"context"
	"reflect"
	"testing"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestClientRepositoryQueriesOnlyActiveClientsAndMapsFields(t *testing.T) {
	api := &fakeAPI{
		dataSourceID: "ds-1",
		pages: []notionapi.Page{
			{
				ID: "page-1",
				Properties: notionapi.Properties{
					"ID":              richTextProperty("customer-a"),
					"Name":            titleProperty("Acme Corp"),
					"Period Type":     selectProperty("monthly"),
					"Schedule Preset": selectProperty("standard"),
					"Region":          selectProperty("AB"),
					"Contact Email":   emailProperty("ops@acme.example"),
					"Email Style":     selectProperty("standard"),
					"Greeting":        richTextProperty("Hello Acme Team,"),
					"Folder URL":      urlProperty("https://files.example.com/acme"),
					"Prompt":          richTextProperty("Please upload the latest monthly data exports."),
				},
			},
		},
	}

	clients, err := New(api, "ds-1", FieldMapping{}).GetAllClients()
	if err != nil {
		t.Fatalf("GetAllClients returned error: %v", err)
	}

	if api.queriedDataSourceID != "ds-1" {
		t.Fatalf("expected data source ds-1, got %q", api.queriedDataSourceID)
	}
	wantFilter := map[string]any{
		"property": "Status",
		"select": map[string]string{
			"equals": "active",
		},
	}
	if !reflect.DeepEqual(api.query.Filter, wantFilter) {
		t.Fatalf("unexpected filter: got %#v want %#v", api.query.Filter, wantFilter)
	}

	want := []entities.Client{
		{
			ID:           "page-1",
			Name:         "Acme Corp",
			PeriodType:   entities.PeriodMonthly,
			ReminderGaps: entities.MinimumBusinessDayGaps{0, 2, 2},
			Region:       entities.RegionAlberta,
			Email:        "ops@acme.example",
			EmailStyle:   "standard",
			Greeting:     "Hello Acme Team,",
			FolderURL:    "https://files.example.com/acme",
			UploadPrompt: "Please upload the latest monthly data exports.",
		},
	}
	if !reflect.DeepEqual(clients, want) {
		t.Fatalf("unexpected clients:\ngot  %#v\nwant %#v", clients, want)
	}
}

func TestClientRepositoryResolvesDataSourceNameAndDefaultsReminderGaps(t *testing.T) {
	api := &fakeAPI{
		dataSourceID: "resolved-ds",
		pages: []notionapi.Page{
			{
				ID: "page-1",
				Properties: notionapi.Properties{
					"Name":        titleProperty("Acme Corp"),
					"Period Type": numberProperty(2),
				},
			},
		},
	}

	clients, err := NewForDataSourceName(api, "Clients", FieldMapping{}).GetAllClients()
	if err != nil {
		t.Fatalf("GetAllClients returned error: %v", err)
	}

	if api.searchedTitle != "Clients" {
		t.Fatalf("expected data source title search, got %q", api.searchedTitle)
	}
	if clients[0].ID != "page-1" {
		t.Fatalf("expected page ID fallback, got %q", clients[0].ID)
	}
	if !reflect.DeepEqual(clients[0].ReminderGaps, entities.ReminderGapsStandard) {
		t.Fatalf("expected standard gaps, got %+v", clients[0].ReminderGaps)
	}
	if clients[0].PeriodType != entities.PeriodQuarterly {
		t.Fatalf("expected quarterly period, got %v", clients[0].PeriodType)
	}
}

type fakeAPI struct {
	dataSourceID        string
	searchedTitle       string
	queriedDataSourceID string
	query               notionapi.QueryDataSourceRequest
	pages               []notionapi.Page
}

func (f *fakeAPI) FindDataSourceIDByTitle(_ context.Context, title string) (string, error) {
	f.searchedTitle = title
	return f.dataSourceID, nil
}

func (f *fakeAPI) QueryDataSource(_ context.Context, dataSourceID string, query notionapi.QueryDataSourceRequest) ([]notionapi.Page, error) {
	f.queriedDataSourceID = dataSourceID
	f.query = query
	return f.pages, nil
}

func titleProperty(value string) notionapi.Property {
	return notionapi.Property{
		Type:  "title",
		Title: []notionapi.RichTextValue{{PlainText: value}},
	}
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

func multiSelectProperty(values ...string) notionapi.Property {
	options := make([]notionapi.NamedValue, 0, len(values))
	for _, value := range values {
		options = append(options, notionapi.NamedValue{Name: value})
	}
	return notionapi.Property{
		Type:        "multi_select",
		MultiSelect: options,
	}
}

func emailProperty(value string) notionapi.Property {
	return notionapi.Property{Type: "email", Email: value}
}

func urlProperty(value string) notionapi.Property {
	return notionapi.Property{Type: "url", URL: value}
}

func numberProperty(value float64) notionapi.Property {
	return notionapi.Property{Type: "number", Number: &value}
}
