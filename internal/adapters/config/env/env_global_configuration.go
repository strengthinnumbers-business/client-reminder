package env

import (
	"fmt"
	"os"

	"github.com/strengthinnumbers-business/client-reminder/internal/core/ports"
)

type GlobalConfiguration struct {
	templatePath string
	logger       ports.Logger
}

type Option func(*GlobalConfiguration)

func New(templatePath string, options ...Option) *GlobalConfiguration {
	c := &GlobalConfiguration{templatePath: templatePath, logger: ports.NoopLogger{}}
	for _, option := range options {
		option(c)
	}
	c.logger = ports.EnsureLogger(c.logger)
	return c
}

func WithLogger(logger ports.Logger) Option {
	return func(c *GlobalConfiguration) {
		c.logger = ports.EnsureLogger(logger)
	}
}

func (c *GlobalConfiguration) GetEmailBodyTemplate(sequenceIndex int, style string) (string, string, error) {
	// TODO: use sequenceIndex and style to select different templates if needed
	c.logger.Demo("loading email template configuration", "sequence_index", sequenceIndex, "style", style)

	subject := os.Getenv("EMAIL_SUBJECT_TEMPLATE")
	if subject == "" {
		subject = "Reminder to upload your data"
		c.logger.Demo("using default email subject template", "subject", subject)
	} else {
		c.logger.Demo("using email subject template from environment", "subject", subject)
	}

	if c.templatePath != "" {
		c.logger.Demo("loading email body template from file", "path", c.templatePath)
		bytes, err := os.ReadFile(c.templatePath)
		if err != nil {
			return "", "", fmt.Errorf("read template file: %w", err)
		}
		c.logger.Demo("loaded email body template from file", "path", c.templatePath, "body_bytes", len(bytes))
		return subject, string(bytes), nil
	}

	tpl := os.Getenv("EMAIL_BODY_TEMPLATE")
	if tpl == "" {
		return "", "", fmt.Errorf("email template is empty: set EMAIL_BODY_TEMPLATE or EMAIL_TEMPLATE_PATH")
	}
	c.logger.Demo("loaded email body template from environment", "body_bytes", len(tpl))
	return subject, tpl, nil
}
