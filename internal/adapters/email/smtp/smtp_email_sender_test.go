package smtp

import (
	netsmtp "net/smtp"
	"reflect"
	"strings"
	"testing"
)

func TestEmailSenderSendsOneMessageToAllRecipients(t *testing.T) {
	var gotTo []string
	var gotMessage string
	sender := New("smtp.example.com", "25", "", "", "from@example.com", WithSendMail(func(_ string, _ netsmtp.Auth, _ string, to []string, message []byte) error {
		gotTo = to
		gotMessage = string(message)
		return nil
	}))

	recipients := []string{"ops@example.com", "finance@example.com"}
	if err := sender.SendEmail(recipients, "Subject", "Body"); err != nil {
		t.Fatalf("SendEmail returned error: %v", err)
	}

	if !reflect.DeepEqual(gotTo, recipients) {
		t.Fatalf("unexpected envelope recipients: got %#v want %#v", gotTo, recipients)
	}
	if !strings.Contains(gotMessage, "To: ops@example.com, finance@example.com\r\n") {
		t.Fatalf("missing recipient header in message:\n%s", gotMessage)
	}
}
