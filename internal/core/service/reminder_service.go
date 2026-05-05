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

func (s *ReminderService) Run(ctx context.Context) (RunResult, error) {
	_ = ctx

	now := s.clock().UTC()
	s.logger.Demo("starting reminder run", "run_date", now.Format(time.DateOnly), "run_time_utc", now.Format(time.RFC3339))

	clients, err := s.clientRepo.GetAllClients()
	if err != nil {
		return RunResult{}, fmt.Errorf("load clients: %w", err)
	}
	s.logger.Demo("loaded active clients", "count", len(clients))

	result := RunResult{TotalCustomers: len(clients)}

	for _, client := range clients {
		currentPeriod := entities.CurrentPeriod(client.PeriodType, now)
		s.logger.Demo("evaluating client", "client_id", client.ID, "client_name", client.Name, "email", client.Email, "period_type", periodTypeName(client.PeriodType), "current_period", currentPeriod.ID, "region", client.Region, "email_style", client.EmailStyle, "reminder_gaps", client.ReminderGaps.Effective())
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
		s.logger.Demo("loaded previous successful sends for current period", "client_id", client.ID, "period", currentPeriod.ID, "count", len(successfulSends))

		eligibility, ok, err := schedule.NextEligibility(now, successfulSends, s.holidayChecker)
		if err != nil {
			s.logger.Error("determine reminder eligibility", "client_id", client.ID, "period", currentPeriod.ID, "error", err)
			result.Failures++
			continue
		}
		if !ok {
			s.logger.Demo("client is not eligible for a reminder today", "client_id", client.ID, "period", currentPeriod.ID, "successful_send_count", len(successfulSends), "configured_sequence_count", len(client.ReminderGaps.Effective()))
			continue
		}
		s.logger.Demo("client is eligible for a reminder", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "earliest_date", eligibility.EarliestDate.Format(time.DateOnly))

		verdict, err := s.completionDecider.IsCompleted(client, eligibility.Period)
		if err != nil {
			s.logger.Error("decide completion", "client_id", client.ID, "period", eligibility.Period.ID, "error", err)
			result.Failures++
			continue
		}
		s.logger.Demo("loaded upload review verdict", "client_id", client.ID, "period", eligibility.Period.ID, "verdict", completionVerdictName(verdict))

		switch verdict {
		case entities.CompletionComplete:
			s.logger.Demo("skipping reminder because upload is marked complete", "client_id", client.ID, "period", eligibility.Period.ID)
			result.SkippedDone++
			continue
		case entities.CompletionUndecided:
			s.logger.Demo("skipping reminder because upload review is undecided", "client_id", client.ID, "period", eligibility.Period.ID)
			continue
		case entities.CompletionVerdictNotRequested, entities.CompletionIncomplete:
			s.logger.Demo("sending reminder because upload is not complete", "client_id", client.ID, "period", eligibility.Period.ID, "verdict", completionVerdictName(verdict))
			s.sendReminder(client, eligibility, client.EmailStyle, verdict, now, &result)
		}
	}

	s.logger.Demo("finished reminder run", "total_clients", result.TotalCustomers, "sent", result.Sent, "skipped_done", result.SkippedDone, "missed_period_alerts", result.MissedPeriodAlerts, "failures", result.Failures)
	return result, nil
}

func (s *ReminderService) alertMissedPreviousPeriod(client entities.Client, currentPeriod entities.Period, result *RunResult) bool {
	previousPeriod := currentPeriod.Previous()
	if previousPeriod.ID == "" {
		s.logger.Demo("previous period could not be determined", "client_id", client.ID, "current_period", currentPeriod.ID)
		return false
	}
	s.logger.Demo("checking whether previous period was missed", "client_id", client.ID, "previous_period", previousPeriod.ID)

	dealtWith, err := s.periodResolutionRepo.IsDealtWith(client, previousPeriod)
	if err != nil {
		s.logger.Error("check period resolution", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if dealtWith {
		s.logger.Demo("previous period already resolved", "client_id", client.ID, "period", previousPeriod.ID)
		return false
	}

	successfulSends, err := s.reminderSendRepo.ListSuccessfulSends(client, previousPeriod)
	if err != nil {
		s.logger.Error("list successful reminder sends for previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if len(successfulSends) > 0 {
		s.logger.Demo("previous period had successful reminder sends", "client_id", client.ID, "period", previousPeriod.ID, "count", len(successfulSends))
		return false
	}
	s.logger.Demo("previous period had no successful reminder sends", "client_id", client.ID, "period", previousPeriod.ID)

	verdict, err := s.completionDecider.IsCompleted(client, previousPeriod)
	if err != nil {
		s.logger.Error("decide completion for previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	s.logger.Demo("loaded previous-period upload review verdict", "client_id", client.ID, "period", previousPeriod.ID, "verdict", completionVerdictName(verdict))
	if verdict == entities.CompletionComplete {
		reason := "completion complete: no reminder needed"
		s.logger.Demo("marking previous period resolved because upload is complete", "client_id", client.ID, "period", previousPeriod.ID, "reason", reason)
		if err := s.periodResolutionRepo.MarkDealtWith(client, previousPeriod, reason); err != nil {
			s.logger.Error("mark previous period dealt with", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
			result.Failures++
		}
		return false
	}

	reason := "admin alerted: period ended with no successful reminders"
	s.logger.Demo("alerting admin about missed previous period", "client_id", client.ID, "period", previousPeriod.ID, "reason", reason)
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

func (s *ReminderService) sendReminder(client entities.Client, eligibility entities.ReminderEligibility, emailStyle string, verdict entities.CompletionVerdict, now time.Time, result *RunResult) {
	s.logger.Demo("loading reminder email template", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "style", emailStyle)
	subjectTemplate, bodyTemplate, err := s.globalConfig.GetEmailBodyTemplate(eligibility.SequenceIndex, emailStyle)
	if err != nil {
		s.logger.Error("load email template", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "style", emailStyle, "error", err)
		result.Failures++
		return
	}

	subjectLine := RenderEmailTemplate(subjectTemplate, client, eligibility.Period, now)
	body := RenderEmailTemplate(bodyTemplate, client, eligibility.Period, now)
	s.logger.Demo("rendered reminder email", "client_id", client.ID, "period", eligibility.Period.ID, "to", client.Email, "subject", subjectLine, "body_bytes", len(body))
	entry := entities.SendLogEntry{
		ClientID:      client.ID,
		ForPeriod:     eligibility.Period,
		ReminderGaps:  client.ReminderGaps.Effective(),
		SequenceIndex: eligibility.SequenceIndex,
		SentAt:        now,
		Success:       true,
	}

	s.logger.Demo("sending reminder email", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "to", client.Email, "subject", subjectLine)
	if err := s.emailSender.SendEmail(client.Email, subjectLine, body); err != nil {
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
	s.logger.Demo("recording successful reminder send", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex)
	if err := s.reminderSendRepo.RecordSuccessfulSend(client, entry); err != nil {
		s.logger.Error("record successful reminder send", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "error", err)
		result.Failures++
		return
	}

	if verdict == entities.CompletionIncomplete {
		s.logger.Demo("resetting upload review verdict after incomplete-upload reminder", "client_id", client.ID, "period", eligibility.Period.ID)
		if err := s.completionDecider.ResetCompletionVerdict(client, eligibility.Period); err != nil {
			s.logger.Error("reset completion verdict", "client_id", client.ID, "period", eligibility.Period.ID, "error", err)
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

func completionVerdictName(verdict entities.CompletionVerdict) string {
	switch verdict {
	case entities.CompletionVerdictNotRequested:
		return "not_requested"
	case entities.CompletionUndecided:
		return "undecided"
	case entities.CompletionIncomplete:
		return "upload_incomplete"
	case entities.CompletionComplete:
		return "upload_complete"
	default:
		return fmt.Sprintf("unknown_%d", verdict)
	}
}

func periodTypeName(periodType entities.PeriodType) string {
	switch periodType {
	case entities.PeriodWeekly:
		return "weekly"
	case entities.PeriodMonthly:
		return "monthly"
	case entities.PeriodQuarterly:
		return "quarterly"
	default:
		return fmt.Sprintf("unknown_%d", periodType)
	}
}
