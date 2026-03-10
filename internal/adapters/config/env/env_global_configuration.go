package env

import (
	"fmt"
	"os"
)

type GlobalConfiguration struct {
	templatePath string
}

func New(templatePath string) *GlobalConfiguration {
	return &GlobalConfiguration{templatePath: templatePath}
}

func (c *GlobalConfiguration) GetEmailBodyTemplate() (string, error) {
	if c.templatePath != "" {
		bytes, err := os.ReadFile(c.templatePath)
		if err != nil {
			return "", fmt.Errorf("read template file: %w", err)
		}
		return string(bytes), nil
	}

	tpl := os.Getenv("EMAIL_BODY_TEMPLATE")
	if tpl == "" {
		return "", fmt.Errorf("email template is empty: set EMAIL_BODY_TEMPLATE or EMAIL_TEMPLATE_PATH")
	}
	return tpl, nil
}
