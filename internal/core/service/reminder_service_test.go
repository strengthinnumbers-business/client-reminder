package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	adminmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/adminalert/mock"
	clientmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/client/mock"
	completionmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/completion/mock"
	configmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/config/mock"
	emailmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/email/mock"
	holidaymock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/mock"
	resolutionmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/periodresolution/mock"
	sendmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/remindersend/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/service"
)

func TestReminderServiceRun_SendsOnlyForCustomersReadyForReminder(t *testing.T) {
	now := time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)
	customers := []entities.Client{
		testClient("c1"),
		testClient("c2"),
		testClient("c3"),
		testClient("c4"),
	}

	emailSender := &emailmock.EmailSender{}
	clientRepo := &clientmock.ClientRepository{Clients: customers}
	completionDecider := &completionmock.CompletionDecider{}
	period := entities.CurrentPeriod(entities.PeriodMonthly, now)
	for _, customer := range customers {
		completionDecider.SetVerdict(customer.ID, period.ID, customerVerdict(customer))
	}

	svc := newTestService(now, emailSender, clientRepo, completionDecider, &sendmock.ReminderSendRepository{}, dealtWithPrevious(customers, now), &adminmock.AdminAlerter{})

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.Sent != 2 {
		t.Fatalf("expected sent=2, got %d", result.Sent)
	}
	if result.SkippedDone != 1 {
		t.Fatalf("expected skipped_done=1, got %d", result.SkippedDone)
	}
	if len(emailSender.Sent) != 2 {
		t.Fatalf("expected 2 sent emails, got %d", len(emailSender.Sent))
	}
	if emailSender.Sent[0].Subject != "c1 data request for 2026-02" {
		t.Fatalf("unexpected subject: %q", emailSender.Sent[0].Subject)
	}
}

func TestReminderServiceRun_UsesActualPreviousSendForNextGap(t *testing.T) {
	now := time.Date(2026, time.February, 9, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	sendRepo := &sendmock.ReminderSendRepository{
		SuccessfulSends: []sendmock.RecordedSend{
			{
				ClientID: customer.ID,
				Entry: entities.SendLogEntry{
					ForPeriod:     entities.CurrentPeriod(entities.PeriodMonthly, now),
					ReminderGaps:  customer.ReminderGaps,
					SequenceIndex: 0,
					SentAt:        time.Date(2026, time.February, 4, 8, 0, 0, 0, time.UTC),
					Success:       true,
				},
			},
		},
	}
	emailSender := &emailmock.EmailSender{}

	svc := newTestService(now, emailSender, &clientmock.ClientRepository{Clients: []entities.Client{customer}}, &completionmock.CompletionDecider{}, sendRepo, dealtWithPrevious([]entities.Client{customer}, now), &adminmock.AdminAlerter{})

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.Sent != 1 {
		t.Fatalf("expected sent=1, got %d", result.Sent)
	}
	successfulSends, err := sendRepo.ListSuccessfulSends(customer, entities.CurrentPeriod(customer.PeriodType, now))
	if err != nil {
		t.Fatalf("ListSuccessfulSends returned error: %v", err)
	}
	if len(successfulSends) != 2 || successfulSends[1].SequenceIndex != 1 {
		t.Fatalf("expected second successful send at index 1, got %+v", successfulSends)
	}
}

func TestReminderServiceRun_LoadsTemplateForSequenceIndexAndEmailStyle(t *testing.T) {
	now := time.Date(2026, time.February, 9, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	customer.EmailStyle = "brief"
	currentPeriod := entities.CurrentPeriod(customer.PeriodType, now)
	config := &configmock.GlobalConfiguration{
		SubjectTemplate: "Reminder {{PeriodID}}",
		Template:        "{{Greeting}} {{PeriodID}}",
	}
	sendRepo := &sendmock.ReminderSendRepository{
		SuccessfulSends: []sendmock.RecordedSend{
			{
				ClientID: customer.ID,
				Entry: entities.SendLogEntry{
					ForPeriod:     currentPeriod,
					ReminderGaps:  customer.ReminderGaps,
					SequenceIndex: 0,
					SentAt:        time.Date(2026, time.February, 4, 8, 0, 0, 0, time.UTC),
					Success:       true,
				},
			},
		},
	}
	emailSender := &emailmock.EmailSender{}

	svc := service.NewReminderService(
		emailSender,
		&clientmock.ClientRepository{Clients: []entities.Client{customer}},
		config,
		&completionmock.CompletionDecider{},
		&holidaymock.HolidayChecker{},
		sendRepo,
		dealtWithPrevious([]entities.Client{customer}, now),
		&adminmock.AdminAlerter{},
		func() time.Time { return now },
	)

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.Sent != 1 {
		t.Fatalf("expected sent=1, got %d", result.Sent)
	}
	if len(config.Calls) != 1 {
		t.Fatalf("expected one template lookup, got %+v", config.Calls)
	}
	if config.Calls[0].SequenceIndex != 1 || config.Calls[0].Style != "brief" {
		t.Fatalf("expected sequence/style lookup 1/brief, got %+v", config.Calls[0])
	}
	if len(emailSender.Sent) != 1 || emailSender.Sent[0].Subject != "Reminder 2026-02" {
		t.Fatalf("unexpected sent email: %+v", emailSender.Sent)
	}
}

func TestReminderServiceRun_DoesNotSendMoreThanOneCatchUpReminderPerRun(t *testing.T) {
	now := time.Date(2026, time.February, 20, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	sendRepo := &sendmock.ReminderSendRepository{}
	emailSender := &emailmock.EmailSender{}

	svc := newTestService(now, emailSender, &clientmock.ClientRepository{Clients: []entities.Client{customer}}, &completionmock.CompletionDecider{}, sendRepo, dealtWithPrevious([]entities.Client{customer}, now), &adminmock.AdminAlerter{})

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.Sent != 1 {
		t.Fatalf("expected sent=1, got %d", result.Sent)
	}
	if len(sendRepo.SuccessfulSends) != 1 || sendRepo.SuccessfulSends[0].Entry.SequenceIndex != 0 {
		t.Fatalf("expected only first catch-up reminder to be recorded, got %+v", sendRepo.SuccessfulSends)
	}
}

func TestReminderServiceRun_LogsFailedSendWithoutAdvancingSequence(t *testing.T) {
	now := time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	sendRepo := &sendmock.ReminderSendRepository{}
	emailSender := &emailmock.EmailSender{Error: errors.New("smtp down")}

	svc := newTestService(now, emailSender, &clientmock.ClientRepository{Clients: []entities.Client{customer}}, &completionmock.CompletionDecider{}, sendRepo, dealtWithPrevious([]entities.Client{customer}, now), &adminmock.AdminAlerter{})

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.Failures != 1 {
		t.Fatalf("expected failures=1, got %d", result.Failures)
	}
	if len(sendRepo.SuccessfulSends) != 0 {
		t.Fatalf("expected no successful sends, got %+v", sendRepo.SuccessfulSends)
	}
	if len(sendRepo.FailedSends) != 1 {
		t.Fatalf("expected one failed send, got %+v", sendRepo.FailedSends)
	}
}

func TestReminderServiceRun_AlertsAndMarksMissedPreviousPeriod(t *testing.T) {
	now := time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	resolutionRepo := &resolutionmock.PeriodResolutionRepository{}
	adminAlerter := &adminmock.AdminAlerter{}
	completionDecider := &completionmock.CompletionDecider{}
	completionDecider.SetVerdict(customer.ID, entities.CurrentPeriod(customer.PeriodType, now).ID, entities.CompletionComplete)

	svc := newTestService(now, &emailmock.EmailSender{}, &clientmock.ClientRepository{Clients: []entities.Client{customer}}, completionDecider, &sendmock.ReminderSendRepository{}, resolutionRepo, adminAlerter)

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.MissedPeriodAlerts != 1 {
		t.Fatalf("expected missed_period_alerts=1, got %d", result.MissedPeriodAlerts)
	}
	if len(adminAlerter.Alerts) != 1 {
		t.Fatalf("expected one admin alert, got %+v", adminAlerter.Alerts)
	}
	previous := entities.CurrentPeriod(customer.PeriodType, now).Previous()
	dealtWith, err := resolutionRepo.IsDealtWith(customer, previous)
	if err != nil {
		t.Fatalf("IsDealtWith returned error: %v", err)
	}
	if !dealtWith {
		t.Fatalf("expected previous period to be marked dealt with")
	}
}

func TestReminderServiceRun_DoesNotAlertAlreadyDealtWithPreviousPeriod(t *testing.T) {
	now := time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	resolutionRepo := dealtWithPrevious([]entities.Client{customer}, now)
	adminAlerter := &adminmock.AdminAlerter{}
	completionDecider := &completionmock.CompletionDecider{}
	completionDecider.SetVerdict(customer.ID, entities.CurrentPeriod(customer.PeriodType, now).ID, entities.CompletionComplete)

	svc := newTestService(now, &emailmock.EmailSender{}, &clientmock.ClientRepository{Clients: []entities.Client{customer}}, completionDecider, &sendmock.ReminderSendRepository{}, resolutionRepo, adminAlerter)

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.MissedPeriodAlerts != 0 {
		t.Fatalf("expected missed_period_alerts=0, got %d", result.MissedPeriodAlerts)
	}
	if len(adminAlerter.Alerts) != 0 {
		t.Fatalf("expected no admin alerts, got %+v", adminAlerter.Alerts)
	}
}

func TestReminderServiceRun_DoesNotAlertPreviousPeriodThatWasComplete(t *testing.T) {
	now := time.Date(2026, time.February, 2, 8, 0, 0, 0, time.UTC)
	customer := testClient("c1")
	resolutionRepo := &resolutionmock.PeriodResolutionRepository{}
	adminAlerter := &adminmock.AdminAlerter{}
	completionDecider := &completionmock.CompletionDecider{}
	current := entities.CurrentPeriod(customer.PeriodType, now)
	completionDecider.SetVerdict(customer.ID, current.Previous().ID, entities.CompletionComplete)
	completionDecider.SetVerdict(customer.ID, current.ID, entities.CompletionComplete)

	svc := newTestService(now, &emailmock.EmailSender{}, &clientmock.ClientRepository{Clients: []entities.Client{customer}}, completionDecider, &sendmock.ReminderSendRepository{}, resolutionRepo, adminAlerter)

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}

	if result.MissedPeriodAlerts != 0 {
		t.Fatalf("expected missed_period_alerts=0, got %d", result.MissedPeriodAlerts)
	}
	if len(adminAlerter.Alerts) != 0 {
		t.Fatalf("expected no admin alerts, got %+v", adminAlerter.Alerts)
	}
	dealtWith, err := resolutionRepo.IsDealtWith(customer, current.Previous())
	if err != nil {
		t.Fatalf("IsDealtWith returned error: %v", err)
	}
	if !dealtWith {
		t.Fatalf("expected complete previous period to be marked dealt with")
	}
}

func TestCurrentPeriod(t *testing.T) {
	fixedNow := time.Date(2026, time.February, 10, 8, 0, 0, 0, time.UTC)

	weekly := entities.CurrentPeriod(entities.PeriodWeekly, fixedNow)
	if weekly.ID != "2026-W07" {
		t.Fatalf("unexpected weekly period: %s", weekly.ID)
	}
	if got := weekly.Start(); !got.Equal(time.Date(2026, time.February, 9, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected weekly start: %s", got.Format(time.DateOnly))
	}
	if got := weekly.Previous(); got.ID != "2026-W06" {
		t.Fatalf("unexpected previous weekly period: %s", got.ID)
	}

	monthly := entities.CurrentPeriod(entities.PeriodMonthly, fixedNow)
	if monthly.ID != "2026-02" {
		t.Fatalf("unexpected monthly period: %s", monthly.ID)
	}
	if got := monthly.Start(); !got.Equal(time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected monthly start: %s", got.Format(time.DateOnly))
	}
	if got := monthly.Previous(); got.ID != "2026-01" {
		t.Fatalf("unexpected previous monthly period: %s", got.ID)
	}

	quarterly := entities.CurrentPeriod(entities.PeriodQuarterly, fixedNow)
	if quarterly.ID != "2026-Q1" {
		t.Fatalf("unexpected quarterly period: %s", quarterly.ID)
	}
	if got := quarterly.Start(); !got.Equal(time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected quarterly start: %s", got.Format(time.DateOnly))
	}
	if got := quarterly.Previous(); got.ID != "2025-Q4" {
		t.Fatalf("unexpected previous quarterly period: %s", got.ID)
	}
}

func testClient(id string) entities.Client {
	return entities.Client{
		ID:           id,
		Name:         id,
		PeriodType:   entities.PeriodMonthly,
		ReminderGaps: entities.MinimumBusinessDayGaps{0, 3, 2, 2},
		Region:       entities.RegionOntario,
		Email:        id + "@example.com",
		EmailStyle:   "standard",
		Greeting:     "Hello,",
		FolderURL:    "https://files/" + id,
		UploadPrompt: "Upload your files",
	}
}

func customerVerdict(customer entities.Client) entities.CompletionVerdict {
	switch customer.ID {
	case "c1":
		return entities.CompletionIncomplete
	case "c2":
		return entities.CompletionComplete
	case "c3":
		return entities.CompletionUndecided
	default:
		return entities.CompletionVerdictNotRequested
	}
}

func newTestService(
	now time.Time,
	emailSender *emailmock.EmailSender,
	clientRepo *clientmock.ClientRepository,
	completionDecider *completionmock.CompletionDecider,
	sendRepo *sendmock.ReminderSendRepository,
	resolutionRepo *resolutionmock.PeriodResolutionRepository,
	adminAlerter *adminmock.AdminAlerter,
) *service.ReminderService {
	return service.NewReminderService(
		emailSender,
		clientRepo,
		&configmock.GlobalConfiguration{
			SubjectTemplate: "{{ClientName}} data request for {{PeriodID}}",
			Template:        "{{Greeting}} {{PeriodID}} {{FolderURL}}",
		},
		completionDecider,
		&holidaymock.HolidayChecker{},
		sendRepo,
		resolutionRepo,
		adminAlerter,
		func() time.Time { return now },
	)
}

func dealtWithPrevious(customers []entities.Client, now time.Time) *resolutionmock.PeriodResolutionRepository {
	repo := &resolutionmock.PeriodResolutionRepository{}
	for _, customer := range customers {
		previous := entities.CurrentPeriod(customer.PeriodType, now).Previous()
		if err := repo.MarkDealtWith(customer, previous, "onboarding baseline: client added after this period"); err != nil {
			panic(err)
		}
	}
	return repo
}
