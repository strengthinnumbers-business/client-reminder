package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type Clock func() time.Time

type ReminderService struct {
	emailSender          ports.EmailSender
	clientRepo           ports.ClientRepository
	globalConfig         ports.GlobalConfiguration
	completionDecider    ports.CompletionDecider
	holidayChecker       ports.HolidayChecker
	reminderSendRepo     ports.ReminderSendRepository
	periodResolutionRepo ports.PeriodResolutionRepository
	adminAlerter         ports.AdminAlerter
	uploadSnapshotter    ports.UploadSnapshotter
	logger               ports.Logger
	clock                Clock
}

type RunResult struct {
	TotalCustomers     int
	SkippedDone        int
	Sent               int
	MissedPeriodAlerts int
	Failures           int
}

type Option func(*ReminderService)

func NewReminderService(
	emailSender ports.EmailSender,
	clientRepo ports.ClientRepository,
	globalConfig ports.GlobalConfiguration,
	completionDecider ports.CompletionDecider,
	holidayChecker ports.HolidayChecker,
	reminderSendRepo ports.ReminderSendRepository,
	periodResolutionRepo ports.PeriodResolutionRepository,
	adminAlerter ports.AdminAlerter,
	clock Clock,
	options ...Option,
) *ReminderService {
	if clock == nil {
		clock = time.Now
	}

	s := &ReminderService{
		emailSender:          emailSender,
		clientRepo:           clientRepo,
		globalConfig:         globalConfig,
		completionDecider:    completionDecider,
		holidayChecker:       holidayChecker,
		reminderSendRepo:     reminderSendRepo,
		periodResolutionRepo: periodResolutionRepo,
		adminAlerter:         adminAlerter,
		uploadSnapshotter:    ports.NoopUploadSnapshotter{},
		logger:               ports.NoopLogger{},
		clock:                clock,
	}
	for _, option := range options {
		option(s)
	}
	s.logger = ports.EnsureLogger(s.logger)
	return s
}

func WithLogger(logger ports.Logger) Option {
	return func(s *ReminderService) {
		s.logger = ports.EnsureLogger(logger)
	}
}

func WithUploadSnapshotter(uploadSnapshotter ports.UploadSnapshotter) Option {
	return func(s *ReminderService) {
		if uploadSnapshotter != nil {
			s.uploadSnapshotter = uploadSnapshotter
		}
	}
}

func (s *ReminderService) Run(ctx context.Context) (RunResult, error) {
	_ = ctx

	now := s.clock().UTC()
	s.logger.DemoBelow("starting reminder run", "run_date", now.Format(time.DateOnly), "run_time_utc", now.Format(time.RFC3339))

	clients, err := s.clientRepo.GetAllClients()
	if err != nil {
		return RunResult{}, fmt.Errorf("load clients: %w", err)
	}
	s.logger.DemoAbove("loaded active clients", "count", len(clients))

	result := RunResult{TotalCustomers: len(clients)}
	previousSnapshot, currentSnapshot, err := s.uploadSnapshotter.GetPreviousAndCurrentSnapshot()
	if err != nil {
		return RunResult{}, fmt.Errorf("snapshot uploads: %w", err)
	}
	s.logger.DemoAbove("loaded upload snapshots", "previous_files", len(previousSnapshot), "current_files", len(currentSnapshot))

	for _, client := range clients {
		currentPeriod := entities.CurrentPeriod(client.PeriodType, now)
		s.logger.DemoSurrounding("evaluating client", "client_id", client.ID, "client_name", client.Name, "emails", client.Emails, "period_type", client.PeriodType.Name(), "current_period", currentPeriod.ID, "region", client.Region, "email_style", client.EmailStyle, "reminder_gaps", client.ReminderGaps.Effective())

		changes := entities.DiffSnapshots(previousSnapshot.Filter(client.FolderPath), currentSnapshot.Filter(client.FolderPath))
		if changes.Any() {
			changesSummary := changes.Summary()
			s.logger.DemoBelow("requesting upload review verdict for changed files", "client_id", client.ID, "period", currentPeriod.ID, "added", len(changes.Added), "changed", len(changes.Changed), "deleted", len(changes.Deleted))
			if _, err := s.completionDecider.RequestNewCompletionVerdict(client, currentPeriod, changesSummary); err != nil {
				s.logger.Error("request new completion verdict for upload changes", "client_id", client.ID, "period", currentPeriod.ID, "error", err)
				result.Failures++
				continue
			}
		}

		if s.alertMissedPreviousPeriod(client, currentPeriod, &result) {
			result.MissedPeriodAlerts++
		}

		schedule := client.ReminderSchedule()
		successfulSends, err := s.reminderSendRepo.ListSuccessfulSends(client, currentPeriod)
		if err != nil {
			s.logger.Error("list successful reminder sends", "client_id", client.ID, "period", currentPeriod.ID, "error", err)
			result.Failures++
			continue
		}
		s.logger.DemoAbove("loaded previous successful sends for current period", "client_id", client.ID, "period", currentPeriod.ID, "count", len(successfulSends))

		eligibility, ok, err := schedule.NextEligibility(now, successfulSends, s.holidayChecker)
		if err != nil {
			s.logger.Error("determine reminder eligibility", "client_id", client.ID, "period", currentPeriod.ID, "error", err)
			result.Failures++
			continue
		}
		if !ok {
			s.logger.DemoSurrounding("client is not eligible for a reminder today", "client_id", client.ID, "period", currentPeriod.ID, "successful_send_count", len(successfulSends), "configured_sequence_count", len(client.ReminderGaps.Effective()))
			continue
		}
		s.logger.DemoAbove("client is eligible for a reminder", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "earliest_date", eligibility.EarliestDate.Format(time.DateOnly))

		verdictTask, err := s.completionDecider.GetVerdict(client, eligibility.Period)
		if err != nil {
			s.logger.Error("decide completion", "client_id", client.ID, "period", eligibility.Period.ID, "error", err)
			result.Failures++
			continue
		}
		verdict := verdictTask.Status
		s.logger.DemoAbove("loaded upload review verdict", "client_id", client.ID, "period", eligibility.Period.ID, "verdict", verdict.Name())

		switch verdict {
		case entities.CompletionComplete:
			s.logger.DemoSurrounding("skipping reminder because upload is marked complete", "client_id", client.ID, "period", eligibility.Period.ID)
			result.SkippedDone++
			continue
		case entities.CompletionUndecided:
			s.logger.DemoSurrounding("skipping reminder because upload review is undecided", "client_id", client.ID, "period", eligibility.Period.ID)
			continue
		case entities.CompletionVerdictNotRequested, entities.CompletionIncomplete:
			s.logger.DemoSurrounding("sending reminder because upload is not complete", "client_id", client.ID, "period", eligibility.Period.ID, "verdict", verdict.Name())
			s.sendReminder(client, eligibility, client.EmailStyle, verdict, now, &result)
		}
	}

	s.logger.DemoAbove("finished reminder run", "total_clients", result.TotalCustomers, "sent", result.Sent, "skipped_done", result.SkippedDone, "missed_period_alerts", result.MissedPeriodAlerts, "failures", result.Failures)
	return result, nil
}

func (s *ReminderService) alertMissedPreviousPeriod(client entities.Client, currentPeriod entities.Period, result *RunResult) bool {
	previousPeriod := currentPeriod.Previous()
	if previousPeriod.ID == "" {
		s.logger.DemoSurrounding("previous period could not be determined", "client_id", client.ID, "current_period", currentPeriod.ID)
		return false
	}
	s.logger.DemoBelow("checking whether previous period was missed", "client_id", client.ID, "previous_period", previousPeriod.ID)

	dealtWith, err := s.periodResolutionRepo.IsDealtWith(client, previousPeriod)
	if err != nil {
		s.logger.Error("check period resolution", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if dealtWith {
		s.logger.DemoSurrounding("previous period already resolved", "client_id", client.ID, "period", previousPeriod.ID)
		return false
	}

	successfulSends, err := s.reminderSendRepo.ListSuccessfulSends(client, previousPeriod)
	if err != nil {
		s.logger.Error("list successful reminder sends for previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if len(successfulSends) > 0 {
		s.logger.DemoSurrounding("previous period had successful reminder sends", "client_id", client.ID, "period", previousPeriod.ID, "count", len(successfulSends))
		return false
	}
	s.logger.DemoAbove("previous period had no successful reminder sends", "client_id", client.ID, "period", previousPeriod.ID)

	verdictTask, err := s.completionDecider.GetVerdict(client, previousPeriod)
	if err != nil {
		s.logger.Error("decide completion for previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	verdict := verdictTask.Status
	s.logger.DemoAbove("loaded previous-period upload review verdict", "client_id", client.ID, "period", previousPeriod.ID, "verdict", verdict.Name())
	if verdict == entities.CompletionComplete {
		reason := "completion complete: no reminder needed"
		s.logger.DemoSurrounding("marking previous period resolved because upload is complete", "client_id", client.ID, "period", previousPeriod.ID, "reason", reason)
		if err := s.periodResolutionRepo.MarkDealtWith(client, previousPeriod, reason); err != nil {
			s.logger.Error("mark previous period dealt with", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
			result.Failures++
		}
		return false
	}

	reason := "admin alerted: period ended with no successful reminders"
	s.logger.DemoBelow("alerting admin about missed previous period", "client_id", client.ID, "period", previousPeriod.ID, "reason", reason)
	if err := s.adminAlerter.AlertMissedPeriod(client, previousPeriod, reason); err != nil {
		s.logger.Error("alert missed previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if err := s.periodResolutionRepo.MarkDealtWith(client, previousPeriod, reason); err != nil {
		s.logger.Error("mark previous period dealt with", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}

	return true
}

func (s *ReminderService) sendReminder(client entities.Client, eligibility entities.ReminderEligibility, emailStyle string, verdict entities.CompletionVerdictStatus, now time.Time, result *RunResult) {
	s.logger.DemoBelow("loading reminder email template", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "style", emailStyle)
	subjectTemplate, bodyTemplate, err := s.globalConfig.GetEmailBodyTemplate(eligibility.SequenceIndex, emailStyle)
	if err != nil {
		s.logger.Error("load email template", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "style", emailStyle, "error", err)
		result.Failures++
		return
	}

	subjectLine := RenderEmailTemplate(subjectTemplate, client, eligibility.Period, now)
	body := RenderEmailTemplate(bodyTemplate, client, eligibility.Period, now)
	s.logger.DemoAbove("rendered reminder email", "client_id", client.ID, "period", eligibility.Period.ID, "to", client.Emails, "subject", subjectLine, "body_bytes", len(body))
	entry := entities.SendLogEntry{
		ClientID:      client.ID,
		ForPeriod:     eligibility.Period,
		ReminderGaps:  client.ReminderGaps.Effective(),
		SequenceIndex: eligibility.SequenceIndex,
		SentAt:        now,
		Success:       true,
	}

	s.logger.DemoBelow("sending reminder email", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "to", client.Emails, "subject", subjectLine)
	if err := s.emailSender.SendEmail(client.Emails, subjectLine, body); err != nil {
		s.logger.Error("send reminder email", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "error", err)
		entry.Success = false
		entry.ErrorMessage = err.Error()
		if recordErr := s.reminderSendRepo.RecordFailedSend(client, entry); recordErr != nil {
			s.logger.Error("record failed reminder send", "client_id", client.ID, "error", recordErr)
		}
		result.Failures++
		return
	}

	result.Sent++
	s.logger.DemoBelow("recording successful reminder send", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex)
	if err := s.reminderSendRepo.RecordSuccessfulSend(client, entry); err != nil {
		s.logger.Error("record successful reminder send", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "error", err)
		result.Failures++
		return
	}

	if verdict == entities.CompletionIncomplete {
		s.logger.DemoBelow("requesting new upload review verdict after incomplete-upload reminder", "client_id", client.ID, "period", eligibility.Period.ID)
		if _, err := s.completionDecider.RequestNewCompletionVerdict(client, eligibility.Period, ""); err != nil {
			s.logger.Error("request new completion verdict", "client_id", client.ID, "period", eligibility.Period.ID, "error", err)
			result.Failures++
		}
	}
}

func RenderEmailTemplate(template string, client entities.Client, period entities.Period, now time.Time) string {
	replacer := strings.NewReplacer(
		"{{ClientName}}", client.Name,
		"{{Greeting}}", client.Greeting,
		"{{FolderURL}}", client.FolderURL,
		"{{UploadPrompt}}", client.UploadPrompt,
		"{{PeriodID}}", period.ID,
		"{{RunDate}}", now.Format("2006-01-02"),
	)
	return replacer.Replace(template)
}
