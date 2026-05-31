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
	uploadsnapshotlocalfs "github.com/strengthinnumbers-business/client-reminder/internal/adapters/uploadsnapshot/localfs"
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
	uploadDir := envOrDefault("UPLOAD_DIR", "state/upload-mirror")
	uploadSnapshotDir := envOrDefault("UPLOAD_SNAPSHOT_DIR", "state/upload-snapshots")

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

	logger := loggerFromEnv()
	logger.DemoBelow("building demo reminder service", "clients_data_source_id_set", notionDataSourceIDClients != "", "tasks_data_source_id_set", notionDataSourceIDTasks != "", "template_path", templatePath, "reminder_send_state_path", reminderSendStatePath, "period_resolution_state_path", periodResolutionStatePath, "holiday_cache_dir", holidayCacheDir, "upload_dir", uploadDir, "upload_snapshot_dir", uploadSnapshotDir, "smtp_host", smtpHost, "smtp_port", smtpPort, "smtp_from", smtpFrom, "admin_email", adminEmail)
	emailSender := emailsmtp.New(smtpHost, smtpPort, smtpUsername, smtpPassword, smtpFrom, emailsmtp.WithLogger(logger))
	notionClient := notionapi.New(notionAPIKey, notionapi.WithLogger(logger))
	clientRepo := clientnotion.New(notionClient, notionDataSourceIDClients, clientnotion.FieldMapping{}, clientnotion.WithLogger(logger))
	completionDecider := completionnotion.New(notionClient, notionDataSourceIDTasks, completionnotion.FieldMapping{}, completionnotion.WithLogger(logger))
	config := configenv.New(templatePath, configenv.WithLogger(logger))
	holidayChecker := holidaycanada.New(holidayCacheDir, holidaycanada.WithCacheTTL(5*365*24*time.Hour), holidaycanada.WithLogger(logger))
	reminderSendRepo := remindersendjson.New(reminderSendStatePath, remindersendjson.WithLogger(logger))
	periodResolutionRepo := periodresolutionjson.New(periodResolutionStatePath, periodresolutionjson.WithLogger(logger))
	adminAlerter := adminalertemail.New(emailSender, adminEmail, adminalertemail.WithLogger(logger))

	demoNow, err := parseEnvDate(os.Getenv("NOW"))
	if err != nil {
		return nil, fmt.Errorf("parse demo NOW env var: %w", err)
	}
	logger.DemoAbove("using injected demo clock", "now", demoNow.UTC().Format(time.RFC3339))

	demoClock := func() time.Time {
		return demoNow
	}
	uploadSnapshotter := uploadsnapshotlocalfs.New(uploadDir, uploadSnapshotDir, uploadsnapshotlocalfs.WithClock(demoClock), uploadsnapshotlocalfs.WithLogger(logger))

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
		service.WithLogger(logger),
		service.WithUploadSnapshotter(uploadSnapshotter),
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
