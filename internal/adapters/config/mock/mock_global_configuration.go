package mock

type GlobalConfiguration struct {
	Template string
	Error    error
}

func (m *GlobalConfiguration) GetEmailBodyTemplate() (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.Template, nil
}
