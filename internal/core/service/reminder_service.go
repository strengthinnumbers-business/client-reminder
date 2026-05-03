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

	clients, err := s.clientRepo.GetAllClients()
	if err != nil {
		return RunResult{}, fmt.Errorf("load clients: %w", err)
	}

	now := s.clock().UTC()
	result := RunResult{TotalCustomers: len(clients)}

	for _, client := range clients {
		currentPeriod := entities.CurrentPeriod(client.PeriodType, now)
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

		eligibility, ok, err := schedule.NextEligibility(now, successfulSends, s.holidayChecker)
		if err != nil {
			s.logger.Error("determine reminder eligibility", "client_id", client.ID, "period", currentPeriod.ID, "error", err)
			result.Failures++
			continue
		}
		if !ok {
			continue
		}
		s.logger.Debug("client reminder eligible", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "earliest_date", eligibility.EarliestDate.Format(time.DateOnly))

		verdict, err := s.completionDecider.IsCompleted(client, eligibility.Period)
		if err != nil {
			s.logger.Error("decide completion", "client_id", client.ID, "period", eligibility.Period.ID, "error", err)
			result.Failures++
			continue
		}

		switch verdict {
		case entities.CompletionComplete:
			result.SkippedDone++
			continue
		case entities.CompletionUndecided:
			continue
		case entities.CompletionVerdictNotRequested, entities.CompletionIncomplete:
			s.sendReminder(client, eligibility, client.EmailStyle, verdict, now, &result)
		}
	}

	return result, nil
}

func (s *ReminderService) alertMissedPreviousPeriod(client entities.Client, currentPeriod entities.Period, result *RunResult) bool {
	previousPeriod := currentPeriod.Previous()
	if previousPeriod.ID == "" {
		return false
	}

	dealtWith, err := s.periodResolutionRepo.IsDealtWith(client, previousPeriod)
	if err != nil {
		s.logger.Error("check period resolution", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if dealtWith {
		return false
	}

	successfulSends, err := s.reminderSendRepo.ListSuccessfulSends(client, previousPeriod)
	if err != nil {
		s.logger.Error("list successful reminder sends for previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if len(successfulSends) > 0 {
		return false
	}

	verdict, err := s.completionDecider.IsCompleted(client, previousPeriod)
	if err != nil {
		s.logger.Error("decide completion for previous period", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
		result.Failures++
		return false
	}
	if verdict == entities.CompletionComplete {
		reason := "completion complete: no reminder needed"
		if err := s.periodResolutionRepo.MarkDealtWith(client, previousPeriod, reason); err != nil {
			s.logger.Error("mark previous period dealt with", "client_id", client.ID, "period", previousPeriod.ID, "error", err)
			result.Failures++
		}
		return false
	}

	reason := "admin alerted: period ended with no successful reminders"
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
	subjectTemplate, bodyTemplate, err := s.globalConfig.GetEmailBodyTemplate(eligibility.SequenceIndex, emailStyle)
	if err != nil {
		s.logger.Error("load email template", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "style", emailStyle, "error", err)
		result.Failures++
		return
	}

	subjectLine := RenderEmailTemplate(subjectTemplate, client, eligibility.Period, now)
	body := RenderEmailTemplate(bodyTemplate, client, eligibility.Period, now)
	entry := entities.SendLogEntry{
		ClientID:      client.ID,
		ForPeriod:     eligibility.Period,
		ReminderGaps:  client.ReminderGaps.Effective(),
		SequenceIndex: eligibility.SequenceIndex,
		SentAt:        now,
		Success:       true,
	}

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
	if err := s.reminderSendRepo.RecordSuccessfulSend(client, entry); err != nil {
		s.logger.Error("record successful reminder send", "client_id", client.ID, "period", eligibility.Period.ID, "sequence_index", eligibility.SequenceIndex, "error", err)
		result.Failures++
		return
	}

	if verdict == entities.CompletionIncomplete {
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
