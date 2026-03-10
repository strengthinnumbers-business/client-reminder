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
	emailSender       ports.EmailSender
	clientRepo        ports.ClientRepository
	globalConfig      ports.GlobalConfiguration
	completionDecider ports.CompletionDecider
	clock             Clock
}

type RunResult struct {
	TotalCustomers int
	SkippedDone    int
	Sent           int
	Failures       int
}

func NewReminderService(
	emailSender ports.EmailSender,
	clientRepo ports.ClientRepository,
	globalConfig ports.GlobalConfiguration,
	completionDecider ports.CompletionDecider,
	clock Clock,
) *ReminderService {
	if clock == nil {
		clock = time.Now
	}

	return &ReminderService{
		emailSender:       emailSender,
		clientRepo:        clientRepo,
		globalConfig:      globalConfig,
		completionDecider: completionDecider,
		clock:             clock,
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
		period := entities.CurrentPeriod(client.PeriodType, now)
		verdict, err := s.completionDecider.IsCompleted(client, period)
		if err != nil {
			result.Failures++
			continue
		}

		if verdict == entities.CompletionIncomplete {
			body := RenderEmailTemplate(template, client, period, now)
			if err := s.emailSender.SendEmail(client.Email, body); err != nil {
				result.Failures++
				continue
			}
			result.Sent++
			if err := s.completionDecider.ResetCompletionVerdict(client, period); err != nil {
				result.Failures++
			}
		}

		if verdict == entities.CompletionComplete {
			result.SkippedDone++
			continue
		}

	}

	return result, nil
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
