package entities_test

import (
	"testing"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestPeriodName(t *testing.T) {
	tests := []struct {
		name   string
		period entities.Period
		want   string
	}{
		{
			name:   "weekly",
			period: entities.Period{Type: entities.PeriodWeekly, ID: "2026-W07"},
			want:   "Week 7 of 2026",
		},
		{
			name:   "monthly",
			period: entities.Period{Type: entities.PeriodMonthly, ID: "2026-02"},
			want:   "February 2026",
		},
		{
			name:   "quarterly",
			period: entities.Period{Type: entities.PeriodQuarterly, ID: "2026-Q3"},
			want:   "Q3 2026",
		},
		{
			name:   "invalid id",
			period: entities.Period{Type: entities.PeriodMonthly, ID: "invalid"},
			want:   "",
		},
		{
			name:   "weekly id with trailing text",
			period: entities.Period{Type: entities.PeriodWeekly, ID: "2026-W07-extra"},
			want:   "",
		},
		{
			name:   "weekly id with invalid iso week",
			period: entities.Period{Type: entities.PeriodWeekly, ID: "2026-W99"},
			want:   "",
		},
		{
			name:   "quarterly id with trailing text",
			period: entities.Period{Type: entities.PeriodQuarterly, ID: "2026-Q3-extra"},
			want:   "",
		},
		{
			name:   "unsupported type",
			period: entities.Period{Type: entities.PeriodType(99), ID: "2026-02"},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.period.Name(); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
