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
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
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
	logger   ports.Logger
	mu       sync.Mutex
}

type Option func(*HolidayChecker)

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

func New(cacheDir string, options ...Option) *HolidayChecker {
	c := &HolidayChecker{
		baseURL:  defaultBaseURL,
		cacheDir: cacheDir,
		cacheTTL: defaultCacheTTL,
		client:   &http.Client{Timeout: 10 * time.Second},
		clock:    time.Now,
		logger:   ports.NoopLogger{},
	}
	for _, option := range options {
		option(c)
	}
	c.logger = ports.EnsureLogger(c.logger)
	return c
}

func NewWithOptions(baseURL, cacheDir string, cacheTTL time.Duration, client *http.Client, clock Clock, options ...Option) *HolidayChecker {
	options = append([]Option{WithBaseURL(baseURL), WithCacheTTL(cacheTTL), WithHTTPClient(client), WithClock(clock)}, options...)
	return New(cacheDir, options...)
}

func WithBaseURL(baseURL string) Option {
	return func(c *HolidayChecker) {
		if baseURL != "" {
			c.baseURL = baseURL
		}
	}
}

func WithCacheTTL(cacheTTL time.Duration) Option {
	return func(c *HolidayChecker) {
		if cacheTTL > 0 {
			c.cacheTTL = cacheTTL
		}
	}
}

func WithHTTPClient(client *http.Client) Option {
	return func(c *HolidayChecker) {
		if client != nil {
			c.client = client
		}
	}
}

func WithClock(clock Clock) Option {
	return func(c *HolidayChecker) {
		if clock != nil {
			c.clock = clock
		}
	}
}

func WithLogger(logger ports.Logger) Option {
	return func(c *HolidayChecker) {
		c.logger = ports.EnsureLogger(logger)
	}
}

func (c *HolidayChecker) IsHoliday(date time.Time, region entities.ClientRegion) (bool, error) {
	normalized := normalizeDate(date)
	c.logger.Demo("checking business-day holiday calendar", "date", normalized.Format(time.DateOnly), "region", region)

	holidays, err := c.holidaysForYear(region, normalized.Year())
	if err != nil {
		return false, err
	}

	_, ok := holidays[normalized.Format(time.DateOnly)]
	c.logger.Demo("checked business-day holiday calendar", "date", normalized.Format(time.DateOnly), "region", region, "is_holiday", ok)
	return ok, nil
}

func (c *HolidayChecker) holidaysForYear(region entities.ClientRegion, year int) (map[string]struct{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok, err := c.loadCache(region, year); err != nil {
		return nil, err
	} else if ok {
		c.logger.Demo("using cached holiday calendar", "region", region, "year", year, "holidays", len(cached))
		return cached, nil
	}

	c.logger.Demo("fetching holiday calendar", "region", region, "year", year)
	fresh, err := c.fetchProvinceHolidays(region, year)
	if err != nil {
		return nil, err
	}

	if err := c.storeCache(region, year, fresh); err != nil {
		return nil, err
	}

	c.logger.Demo("loaded fresh holiday calendar", "region", region, "year", year, "holidays", len(fresh))
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
	c.logger.Demo("holiday API responded", "region", region, "year", year, "status", resp.Status)

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
			c.logger.Demo("holiday cache miss", "region", region, "year", year, "path", c.cachePath(region, year))
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read holiday cache: %w", err)
	}

	var cached cachedProvinceHolidays
	if err := json.Unmarshal(bytes, &cached); err != nil {
		return nil, false, fmt.Errorf("decode holiday cache: %w", err)
	}

	if c.clock().UTC().After(cached.FetchedAt.UTC().Add(c.cacheTTL)) {
		c.logger.Demo("holiday cache expired", "region", region, "year", year, "path", c.cachePath(region, year), "fetched_at", cached.FetchedAt.Format(time.RFC3339))
		return nil, false, nil
	}

	holidays := make(map[string]struct{}, len(cached.Holidays))
	for _, date := range cached.Holidays {
		holidays[date] = struct{}{}
	}

	c.logger.Demo("holiday cache hit", "region", region, "year", year, "path", c.cachePath(region, year), "holidays", len(holidays))
	return holidays, true, nil
}

func (c *HolidayChecker) storeCache(region entities.ClientRegion, year int, holidays map[string]struct{}) error {
	if c.cacheDir == "" {
		c.logger.Demo("holiday cache disabled; not storing calendar", "region", region, "year", year)
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
	c.logger.Demo("stored holiday calendar in cache", "region", region, "year", year, "path", c.cachePath(region, year), "holidays", len(holidays))

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
