package service_test

import (
	"context"
	"testing"
	"time"

	clientmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/client/mock"
	completionmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/completion/mock"
	configmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/config/mock"
	emailmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/email/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/service"
)

func TestReminderServiceRun_SendsOnlyForIncompleteCustomers(t *testing.T) {
	customers := []entities.Client{
		{
			ID:           "c1",
			Name:         "Acme",
			PeriodType:   entities.PeriodMonthly,
			Email:        "ops@acme.example",
			Greeting:     "Hello,",
			FolderURL:    "https://files/acme",
			UploadPrompt: "Upload your files",
		},
		{
			ID:           "c2",
			Name:         "Globex",
			PeriodType:   entities.PeriodMonthly,
			Email:        "ops@globex.example",
			Greeting:     "Hi,",
			FolderURL:    "https://files/globex",
			UploadPrompt: "Upload data",
		},
	}

	emailSender := &emailmock.EmailSender{}
	clientRepo := &clientmock.ClientRepository{Clients: customers}
	globalConfig := &configmock.GlobalConfiguration{Template: "{{Greeting}} {{PeriodID}} {{FolderURL}}"}
	completionDecider := &completionmock.CompletionDecider{}

	now := time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC)
	period := entities.CurrentPeriod(entities.PeriodMonthly, now)
	completionDecider.SetVerdict(customers[1].ID, period.ID, entities.CompletionComplete)

	svc := service.NewReminderService(emailSender, clientRepo, globalConfig, completionDecider, func() time.Time {
		return now
	})

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.TotalCustomers != 2 {
		t.Fatalf("expected total=2, got %d", result.TotalCustomers)
	}
	if result.Sent != 1 {
		t.Fatalf("expected sent=1, got %d", result.Sent)
	}
	if result.SkippedDone != 1 {
		t.Fatalf("expected skipped_done=1, got %d", result.SkippedDone)
	}
	if len(emailSender.Sent) != 1 {
		t.Fatalf("expected 1 sent email, got %d", len(emailSender.Sent))
	}
}

func TestCurrentPeriod(t *testing.T) {
	fixedNow := time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC)

	weekly := entities.CurrentPeriod(entities.PeriodWeekly, fixedNow)
	if weekly.ID != "2026-W07" {
		t.Fatalf("unexpected weekly period: %s", weekly.ID)
	}

	monthly := entities.CurrentPeriod(entities.PeriodMonthly, fixedNow)
	if monthly.ID != "2026-02" {
		t.Fatalf("unexpected monthly period: %s", monthly.ID)
	}

	quarterly := entities.CurrentPeriod(entities.PeriodQuarterly, fixedNow)
	if quarterly.ID != "2026-Q1" {
		t.Fatalf("unexpected quarterly period: %s", quarterly.ID)
	}
}
