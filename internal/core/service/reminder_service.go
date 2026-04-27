package service

import (
	"context"
	"fmt"
	"log"
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
	clock                Clock
}

type RunResult struct {
	TotalCustomers     int
	SkippedDone        int
	Sent               int
	MissedPeriodAlerts int
	Failures           int
}

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
) *ReminderService {
	if clock == nil {
		clock = time.Now
	}

	return &ReminderService{
		emailSender:          emailSender,
		clientRepo:           clientRepo,
		globalConfig:         globalConfig,
		completionDecider:    completionDecider,
		holidayChecker:       holidayChecker,
		reminderSendRepo:     reminderSendRepo,
		periodResolutionRepo: periodResolutionRepo,
		adminAlerter:         adminAlerter,
		clock:                clock,
	}
}

func (s *ReminderService) Run(ctx context.Context) (RunResult, error) {
	_ = ctx

	clients, err := s.clientRepo.GetAllClients()
	if err != nil {
		return RunResult{}, fmt.Errorf("load clients: %w", err)
	}

	template, err := s.globalConfig.GetEmailBodyTemplate()
	if err != nil {
		return RunResult{}, fmt.Errorf("load email template: %w", err)
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
			log.Printf("list successful reminder sends for client %s period %s: %v", client.ID, currentPeriod.ID, err)
			result.Failures++
			continue
		}

		eligibility, ok, err := schedule.NextEligibility(now, successfulSends, s.holidayChecker)
		if err != nil {
			log.Printf("determine reminder eligibility for client %s period %s: %v", client.ID, currentPeriod.ID, err)
			result.Failures++
			continue
		}
		if !ok {
			continue
		}
		log.Printf(
			"client %s reminder eligible: period=%s sequence_index=%d earliest_date=%s",
			client.ID,
			eligibility.Period.ID,
			eligibility.SequenceIndex,
			eligibility.EarliestDate.Format(time.DateOnly),
		)

		verdict, err := s.completionDecider.IsCompleted(client, eligibility.Period)
		if err != nil {
			log.Printf("decide completion for client %s period %s: %v", client.ID, eligibility.Period.ID, err)
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
			s.sendReminder(template, client, eligibility, verdict, now, &result)
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
		log.Printf("check period resolution for client %s period %s: %v", client.ID, previousPeriod.ID, err)
		result.Failures++
		return false
	}
	if dealtWith {
		return false
	}

	successfulSends, err := s.reminderSendRepo.ListSuccessfulSends(client, previousPeriod)
	if err != nil {
		log.Printf("list successful reminder sends for client %s previous period %s: %v", client.ID, previousPeriod.ID, err)
		result.Failures++
		return false
	}
	if len(successfulSends) > 0 {
		return false
	}

	verdict, err := s.completionDecider.IsCompleted(client, previousPeriod)
	if err != nil {
		log.Printf("decide completion for client %s previous period %s: %v", client.ID, previousPeriod.ID, err)
		result.Failures++
		return false
	}
	if verdict == entities.CompletionComplete {
		reason := "completion complete: no reminder needed"
		if err := s.periodResolutionRepo.MarkDealtWith(client, previousPeriod, reason); err != nil {
			log.Printf("mark period dealt with for client %s previous period %s: %v", client.ID, previousPeriod.ID, err)
			result.Failures++
		}
		return false
	}

	reason := "admin alerted: period ended with no successful reminders"
	if err := s.adminAlerter.AlertMissedPeriod(client, previousPeriod, reason); err != nil {
		log.Printf("alert missed period for client %s previous period %s: %v", client.ID, previousPeriod.ID, err)
		result.Failures++
		return false
	}
	if err := s.periodResolutionRepo.MarkDealtWith(client, previousPeriod, reason); err != nil {
		log.Printf("mark period dealt with for client %s previous period %s: %v", client.ID, previousPeriod.ID, err)
		result.Failures++
		return false
	}

	return true
}

func (s *ReminderService) sendReminder(template string, client entities.Client, eligibility entities.ReminderEligibility, verdict entities.CompletionVerdict, now time.Time, result *RunResult) {
	body := RenderEmailTemplate(template, client, eligibility.Period, now)
	entry := entities.SendLogEntry{
		ForPeriod:     eligibility.Period,
		ReminderGaps:  client.ReminderGaps.Effective(),
		SequenceIndex: eligibility.SequenceIndex,
		SentAt:        now,
		Success:       true,
	}

	if err := s.emailSender.SendEmail(client.Email, body); err != nil {
		log.Printf("send reminder email for client %s period %s sequence_index=%d: %v", client.ID, eligibility.Period.ID, eligibility.SequenceIndex, err)
		entry.Success = false
		entry.ErrorMessage = err.Error()
		if recordErr := s.reminderSendRepo.RecordFailedSend(client, entry); recordErr != nil {
			log.Printf("record failed reminder send for client %s: %v", client.ID, recordErr)
		}
		result.Failures++
		return
	}

	result.Sent++
	if err := s.reminderSendRepo.RecordSuccessfulSend(client, entry); err != nil {
		log.Printf("record successful reminder send for client %s period %s sequence_index=%d: %v", client.ID, eligibility.Period.ID, eligibility.SequenceIndex, err)
		result.Failures++
		return
	}

	if verdict == entities.CompletionIncomplete {
		if err := s.completionDecider.ResetCompletionVerdict(client, eligibility.Period); err != nil {
			log.Printf("reset completion verdict for client %s period %s: %v", client.ID, eligibility.Period.ID, err)
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
