package canadaholidaysapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/canadaholidaysapi"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestHolidayCheckerUsesObservedDate(t *testing.T) {
	var requests int

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++

		if got, want := r.URL.Path, "/provinces/ON"; got != want {
			t.Fatalf("unexpected path: got %s want %s", got, want)
		}
		if got, want := r.URL.Query().Get("year"), "2026"; got != want {
			t.Fatalf("unexpected year: got %s want %s", got, want)
		}
		if got, want := r.URL.Query().Get("optional"), "false"; got != want {
			t.Fatalf("unexpected optional flag: got %s want %s", got, want)
		}

		body, err := json.Marshal(map[string]any{
			"province": map[string]any{
				"holidays": []map[string]any{
					{
						"date":         "2026-12-26T00:00:00.000Z",
						"observedDate": "2026-12-28T00:00:00.000Z",
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	checker := canadaholidaysapi.NewWithOptions("https://example.test", "", time.Hour, client, func() time.Time {
		return time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	})

	isHoliday, err := checker.IsHoliday(time.Date(2026, time.December, 28, 12, 0, 0, 0, time.UTC), entities.RegionOntario)
	if err != nil {
		t.Fatalf("IsHoliday returned error: %v", err)
	}
	if !isHoliday {
		t.Fatalf("expected observed holiday date to be treated as holiday")
	}
	if requests != 1 {
		t.Fatalf("expected 1 request, got %d", requests)
	}
}

func TestHolidayCheckerCachesProvinceYearLookups(t *testing.T) {
	cacheDir := t.TempDir()
	var requests int

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requests++

		body, err := json.Marshal(map[string]any{
			"province": map[string]any{
				"holidays": []map[string]any{
					{
						"date":         "2026-07-01T00:00:00.000Z",
						"observedDate": "2026-07-01T00:00:00.000Z",
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("encode response: %v", err)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     make(http.Header),
		}, nil
	})}

	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	checker := canadaholidaysapi.NewWithOptions("https://example.test", cacheDir, 31*24*time.Hour, client, func() time.Time {
		return now
	})

	for i := 0; i < 2; i++ {
		isHoliday, err := checker.IsHoliday(time.Date(2026, time.July, 1, 8, 0, 0, 0, time.UTC), entities.RegionOntario)
		if err != nil {
			t.Fatalf("IsHoliday returned error: %v", err)
		}
		if !isHoliday {
			t.Fatalf("expected cached holiday lookup to return true")
		}
	}

	if requests != 1 {
		t.Fatalf("expected 1 upstream request, got %d", requests)
	}

	cachePath := filepath.Join(cacheDir, "ON-2026.json")
	if _, err := os.Stat(cachePath); err != nil {
		t.Fatalf("expected cache file %s: %v", cachePath, err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
