package canadaholidaysapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

const (
	defaultBaseURL  = "https://canada-holidays.ca/api/v1"
	defaultCacheTTL = 28 * 24 * time.Hour
)

type Clock func() time.Time

type HolidayChecker struct {
	baseURL  string
	cacheDir string
	cacheTTL time.Duration
	client   *http.Client
	clock    Clock
	mu       sync.Mutex
}

type cachedProvinceHolidays struct {
	FetchedAt time.Time `json:"fetchedAt"`
	Holidays  []string  `json:"holidays"`
}

type provinceResponse struct {
	Province province `json:"province"`
}

type province struct {
	Holidays []holiday `json:"holidays"`
}

type holiday struct {
	Date         string `json:"date"`
	ObservedDate string `json:"observedDate"`
}

func New(cacheDir string) *HolidayChecker {
	return NewWithOptions(defaultBaseURL, cacheDir, defaultCacheTTL, nil, nil)
}

func NewWithOptions(baseURL, cacheDir string, cacheTTL time.Duration, client *http.Client, clock Clock) *HolidayChecker {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	if cacheTTL <= 0 {
		cacheTTL = defaultCacheTTL
	}
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if clock == nil {
		clock = time.Now
	}

	return &HolidayChecker{
		baseURL:  baseURL,
		cacheDir: cacheDir,
		cacheTTL: cacheTTL,
		client:   client,
		clock:    clock,
	}
}

func (c *HolidayChecker) IsHoliday(date time.Time, region entities.ClientRegion) (bool, error) {
	normalized := normalizeDate(date)

	holidays, err := c.holidaysForYear(region, normalized.Year())
	if err != nil {
		return false, err
	}

	_, ok := holidays[normalized.Format(time.DateOnly)]
	return ok, nil
}

func (c *HolidayChecker) holidaysForYear(region entities.ClientRegion, year int) (map[string]struct{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok, err := c.loadCache(region, year); err != nil {
		return nil, err
	} else if ok {
		return cached, nil
	}

	fresh, err := c.fetchProvinceHolidays(region, year)
	if err != nil {
		return nil, err
	}

	if err := c.storeCache(region, year, fresh); err != nil {
		return nil, err
	}

	return fresh, nil
}

func (c *HolidayChecker) fetchProvinceHolidays(region entities.ClientRegion, year int) (map[string]struct{}, error) {
	endpoint, err := url.Parse(fmt.Sprintf("%s/provinces/%s", c.baseURL, region))
	if err != nil {
		return nil, fmt.Errorf("build holiday endpoint: %w", err)
	}

	query := endpoint.Query()
	query.Set("year", fmt.Sprintf("%d", year))
	query.Set("optional", "false")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build holiday request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch holidays for %s %d: %w", region, year, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch holidays for %s %d: unexpected status %s", region, year, resp.Status)
	}

	var payload provinceResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode holidays for %s %d: %w", region, year, err)
	}

	holidays := make(map[string]struct{}, len(payload.Province.Holidays))
	for _, h := range payload.Province.Holidays {
		if normalized, ok := normalizeAPIDate(h.ObservedDate); ok {
			holidays[normalized] = struct{}{}
			continue
		}
		if normalized, ok := normalizeAPIDate(h.Date); ok {
			holidays[normalized] = struct{}{}
		}
	}

	return holidays, nil
}

func (c *HolidayChecker) loadCache(region entities.ClientRegion, year int) (map[string]struct{}, bool, error) {
	if c.cacheDir == "" {
		return nil, false, nil
	}

	bytes, err := os.ReadFile(c.cachePath(region, year))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read holiday cache: %w", err)
	}

	var cached cachedProvinceHolidays
	if err := json.Unmarshal(bytes, &cached); err != nil {
		return nil, false, fmt.Errorf("decode holiday cache: %w", err)
	}

	if c.clock().UTC().After(cached.FetchedAt.UTC().Add(c.cacheTTL)) {
		return nil, false, nil
	}

	holidays := make(map[string]struct{}, len(cached.Holidays))
	for _, date := range cached.Holidays {
		holidays[date] = struct{}{}
	}

	return holidays, true, nil
}

func (c *HolidayChecker) storeCache(region entities.ClientRegion, year int, holidays map[string]struct{}) error {
	if c.cacheDir == "" {
		return nil
	}

	if err := os.MkdirAll(c.cacheDir, 0o755); err != nil {
		return fmt.Errorf("create holiday cache directory: %w", err)
	}

	dates := make([]string, 0, len(holidays))
	for date := range holidays {
		dates = append(dates, date)
	}

	payload := cachedProvinceHolidays{
		FetchedAt: c.clock().UTC(),
		Holidays:  dates,
	}

	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode holiday cache: %w", err)
	}

	if err := os.WriteFile(c.cachePath(region, year), bytes, 0o644); err != nil {
		return fmt.Errorf("write holiday cache: %w", err)
	}

	return nil
}

func (c *HolidayChecker) cachePath(region entities.ClientRegion, year int) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%s-%d.json", region, year))
}

func normalizeAPIDate(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}

	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		return normalizeDate(parsed).Format(time.DateOnly), true
	}
	if parsed, err := time.Parse(time.DateOnly, raw); err == nil {
		return normalizeDate(parsed).Format(time.DateOnly), true
	}

	return "", false
}

func normalizeDate(date time.Time) time.Time {
	year, month, day := date.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}
