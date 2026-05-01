package notionapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestFindDataSourceIDByTitleSendsAuthVersionAndSearchFilter(t *testing.T) {
	var requests int
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++
		if got, want := r.URL.Path, "/v1/search"; got != want {
			t.Fatalf("unexpected path: got %s want %s", got, want)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer secret"; got != want {
			t.Fatalf("unexpected auth header: got %s want %s", got, want)
		}
		if got, want := r.Header.Get("Notion-Version"), DefaultNotionVersion; got != want {
			t.Fatalf("unexpected Notion version: got %s want %s", got, want)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if got, want := body["query"], "Clients"; got != want {
			t.Fatalf("unexpected query: got %v want %v", got, want)
		}

		return jsonResponse(http.StatusOK, map[string]any{
			"object":      "list",
			"has_more":    false,
			"next_cursor": nil,
			"results": []map[string]any{
				{
					"object": "data_source",
					"id":     "ds-1",
					"title": []map[string]any{
						{"plain_text": "Clients"},
					},
				},
			},
		}), nil
	})}

	id, err := New("secret", WithBaseURL("https://api.notion.test/v1"), WithHTTPClient(httpClient), WithRequestGap(0)).FindDataSourceIDByTitle(context.Background(), "Clients")
	if err != nil {
		t.Fatalf("FindDataSourceIDByTitle returned error: %v", err)
	}
	if id != "ds-1" {
		t.Fatalf("unexpected data source ID: got %s want ds-1", id)
	}
	if requests != 1 {
		t.Fatalf("expected 1 request, got %d", requests)
	}
}

func TestQueryDataSourcePaginatesAndUsesFilterProperties(t *testing.T) {
	var cursors []string
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got, want := r.URL.Path, "/v1/data_sources/ds-1/query"; got != want {
			t.Fatalf("unexpected path: got %s want %s", got, want)
		}
		filterProperties := r.URL.Query()["filter_properties[]"]
		if len(filterProperties) != 2 || filterProperties[0] != "Name" || filterProperties[1] != "Status" {
			t.Fatalf("unexpected filter_properties: %+v", filterProperties)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		cursor, _ := body["start_cursor"].(string)
		cursors = append(cursors, cursor)

		hasMore := cursor == ""
		nextCursor := any(nil)
		if hasMore {
			nextCursor = "next"
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"object":      "list",
			"has_more":    hasMore,
			"next_cursor": nextCursor,
			"results": []map[string]any{
				{"object": "page", "id": "page-" + cursor, "properties": map[string]any{}},
			},
		}), nil
	})}

	pages, err := New("secret", WithBaseURL("https://api.notion.test/v1"), WithHTTPClient(httpClient), WithRequestGap(0)).QueryDataSource(context.Background(), "ds-1", QueryDataSourceRequest{
		Filter: map[string]any{
			"property": "Status",
			"status": map[string]string{
				"equals": "active",
			},
		},
		FilterProperties: []string{"Name", "Status"},
	})
	if err != nil {
		t.Fatalf("QueryDataSource returned error: %v", err)
	}
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %+v", pages)
	}
	if len(cursors) != 2 || cursors[0] != "" || cursors[1] != "next" {
		t.Fatalf("unexpected cursors: %+v", cursors)
	}
}

func TestClientWaitsBetweenRequests(t *testing.T) {
	var requestTimes []time.Time
	httpClient := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestTimes = append(requestTimes, time.Now())
		return jsonResponse(http.StatusOK, map[string]any{
			"object":      "list",
			"has_more":    false,
			"next_cursor": nil,
			"results":     []map[string]any{},
		}), nil
	})}

	client := New("secret", WithBaseURL("https://api.notion.test/v1"), WithHTTPClient(httpClient), WithRequestGap(20*time.Millisecond))
	if _, err := client.QueryDataSource(context.Background(), "ds-1", QueryDataSourceRequest{}); err != nil {
		t.Fatalf("first query returned error: %v", err)
	}
	if _, err := client.QueryDataSource(context.Background(), "ds-1", QueryDataSourceRequest{}); err != nil {
		t.Fatalf("second query returned error: %v", err)
	}

	if len(requestTimes) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(requestTimes))
	}
	if elapsed := requestTimes[1].Sub(requestTimes[0]); elapsed < 20*time.Millisecond {
		t.Fatalf("expected request gap of at least 20ms, got %s", elapsed)
	}
}

func TestPropertiesTextExtractsSupportedPropertyValues(t *testing.T) {
	number := 12.5
	wholeNumber := 12.0
	checked := true
	formulaNumber := 7.0

	properties := Properties{
		"title":        {Type: "title", Title: []RichTextValue{{PlainText: "Acme"}, {PlainText: " Corp"}}},
		"rich_text":    {Type: "rich_text", RichText: []RichTextValue{{PlainText: "Hello"}}},
		"email":        {Type: "email", Email: "ops@acme.example"},
		"url":          {Type: "url", URL: "https://files.example.com/acme"},
		"select":       {Type: "select", Select: &NamedValue{Name: "monthly"}},
		"status":       {Type: "status", Status: &NamedValue{Name: "active"}},
		"multi_select": {Type: "multi_select", MultiSelect: []NamedValue{{Name: "one"}, {Name: "two"}}},
		"number":       {Type: "number", Number: &number},
		"whole_number": {Type: "number", Number: &wholeNumber},
		"checkbox":     {Type: "checkbox", Checkbox: &checked},
		"formula":      {Type: "formula", Formula: &FormulaValue{Type: "number", Number: &formulaNumber}},
	}

	tests := map[string]string{
		"title":        "Acme Corp",
		"rich_text":    "Hello",
		"email":        "ops@acme.example",
		"url":          "https://files.example.com/acme",
		"select":       "monthly",
		"status":       "active",
		"multi_select": "one,two",
		"number":       "12.5",
		"whole_number": "12",
		"checkbox":     "true",
		"formula":      "7",
		"missing":      "",
	}

	for name, want := range tests {
		if got := properties.Text(name); got != want {
			t.Fatalf("Properties.Text(%q) = %q, want %q", name, got, want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func jsonResponse(statusCode int, payload any) *http.Response {
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(payload); err != nil {
		panic(err)
	}
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Header:     make(http.Header),
		Body:       io.NopCloser(&body),
	}
}
