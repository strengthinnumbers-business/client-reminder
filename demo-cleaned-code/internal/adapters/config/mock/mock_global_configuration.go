package mock

type GlobalConfiguration struct {
	SubjectTemplate string
	Template        string
	Calls           []TemplateCall
	Error           error
}

type TemplateCall struct {
	SequenceIndex int
	Style         string
}

func (m *GlobalConfiguration) GetEmailBodyTemplate(sequenceIndex int, style string) (string, string, error) {
	m.Calls = append(m.Calls, TemplateCall{SequenceIndex: sequenceIndex, Style: style})
	if m.Error != nil {
		return "", "", m.Error
	}
	subject := m.SubjectTemplate
	if subject == "" {
		subject = "Reminder to upload your data"
	}
	return subject, m.Template, nil
}
