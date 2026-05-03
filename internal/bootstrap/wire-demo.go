package bootstrap

import (
	"fmt"
	"os"
	"time"

	adminalertemail "github.com/strengthinnumbers-business/client-reminder/internal/adapters/adminalert/email"
	clientnotion "github.com/strengthinnumbers-business/client-reminder/internal/adapters/client/notion"
	completionnotion "github.com/strengthinnumbers-business/client-reminder/internal/adapters/completion/notion"
	configenv "github.com/strengthinnumbers-business/client-reminder/internal/adapters/config/env"
	emailsmtp "github.com/strengthinnumbers-business/client-reminder/internal/adapters/email/smtp"
	holidaycanada "github.com/strengthinnumbers-business/client-reminder/internal/adapters/holiday/canadaholidaysapi"
	notionapi "github.com/strengthinnumbers-business/client-reminder/internal/adapters/notionapi"
	periodresolutionjson "github.com/strengthinnumbers-business/client-reminder/internal/adapters/periodresolution/jsonfile"
	remindersendjson "github.com/strengthinnumbers-business/client-reminder/internal/adapters/remindersend/jsonfile"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/service"
)

func BuildServiceForDemo() (*service.ReminderService, error) {
	notionAPIKey := envOrDefault("NOTION_API_KEY", "")
	notionDataSourceIDClients := envOrDefault("NOTION_CLIENTS_DATA_SOURCE_ID", "")
	notionDataSourceIDTasks := envOrDefault("NOTION_TASKS_DATA_SOURCE_ID", "")
	templatePath := os.Getenv("EMAIL_TEMPLATE_PATH")
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
	notionClient := notionapi.New(notionAPIKey)
	clientRepo := clientnotion.New(notionClient, notionDataSourceIDClients, clientnotion.FieldMapping{})
	completionDecider := completionnotion.New(notionClient, notionDataSourceIDTasks, completionnotion.FieldMapping{})
	config := configenv.New(templatePath)
	holidayChecker := holidaycanada.New(holidayCacheDir)
	reminderSendRepo := remindersendjson.New(reminderSendStatePath)
	periodResolutionRepo := periodresolutionjson.New(periodResolutionStatePath)
	adminAlerter := adminalertemail.New(emailSender, adminEmail)

	demoNow, err := parseEnvDate(os.Getenv("NOW"))
	if err != nil {
		return nil, fmt.Errorf("parse demo NOW env var: %w", err)
	}

	demoClock := func() time.Time {
		return demoNow
	}

	return service.NewReminderService(
		emailSender,
		clientRepo,
		config,
		completionDecider,
		holidayChecker,
		reminderSendRepo,
		periodResolutionRepo,
		adminAlerter,
		demoClock,
	), nil
}

func parseEnvDate(raw string) (time.Time, error) {
	formats := []string{time.DateOnly, time.RFC3339}
	for _, format := range formats {
		if parsed, err := time.Parse(format, raw); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("parse date from env var: unsupported format")
}
