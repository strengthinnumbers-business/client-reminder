package bootstrap

import (
	"fmt"
	"os"

	adminalertemail "github.com/strengthinnumbers-business/client-reminder/internal/adapters/adminalert/email"
	clientjson "github.com/strengthinnumbers-business/client-reminder/internal/adapters/client/jsonfile"
	completionjson "github.com/strengthinnumbers-business/client-reminder/internal/adapters/completion/jsonfile"
	configenv "github.com/strengthinnumbers-business/client-reminder/internal/adapters/config/env"
	emailsmtp "github.com/strengthinnumbers-business/client-reminder/internal/adapters/email/smtp"
	holidaycanada "github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/canadaholidaysapi"
	periodresolutionjson "github.com/strengthinnumbers-business/client-reminder/internal/adapters/periodresolution/jsonfile"
	remindersendjson "github.com/strengthinnumbers-business/client-reminder/internal/adapters/remindersend/jsonfile"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/service"
)

func BuildServiceFromEnv() (*service.ReminderService, error) {
	clientsPath := envOrDefault("CLIENTS_JSON_PATH", "configs/clients.json")
	templatePath := os.Getenv("EMAIL_TEMPLATE_PATH")
	completionStatePath := envOrDefault("COMPLETION_STATE_PATH", "state/completion-verdicts.json")
	reminderSendStatePath := envOrDefault("REMINDER_SEND_STATE_PATH", "state/reminder-sends.json")
	periodResolutionStatePath := envOrDefault("PERIOD_RESOLUTION_STATE_PATH", "state/period-resolutions.json")
	holidayCacheDir := envOrDefault("HOLIDAY_CACHE_DIR", "state/holiday-cache")

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPort := envOrDefault("SMTP_PORT", "25")
	smtpUsername := os.Getenv("SMTP_USERNAME")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	smtpFrom := os.Getenv("SMTP_FROM")
	adminEmail := os.Getenv("ADMIN_EMAIL")

	if smtpHost == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}
	if smtpFrom == "" {
		return nil, fmt.Errorf("SMTP_FROM is required")
	}
	if adminEmail == "" {
		return nil, fmt.Errorf("ADMIN_EMAIL is required")
	}

	emailSender := emailsmtp.New(smtpHost, smtpPort, smtpUsername, smtpPassword, smtpFrom)
	clientRepo := clientjson.New(clientsPath)
	config := configenv.New(templatePath)
	completionDecider := completionjson.New(completionStatePath)
	holidayChecker := holidaycanada.New(holidayCacheDir)
	reminderSendRepo := remindersendjson.New(reminderSendStatePath)
	periodResolutionRepo := periodresolutionjson.New(periodResolutionStatePath)
	adminAlerter := adminalertemail.New(emailSender, adminEmail)

	return service.NewReminderService(
		emailSender,
		clientRepo,
		config,
		completionDecider,
		holidayChecker,
		reminderSendRepo,
		periodResolutionRepo,
		adminAlerter,
		nil,
	), nil
}

func envOrDefault(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
