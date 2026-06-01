package email

import (
	"reflect"
	"testing"

	emailmock "github.com/strengthinnumbers-business/client-reminder/internal/adapters/email/mock"
	"github.com/strengthinnumbers-business/client-reminder/internal/core/entities"
)

func TestAdminAlerterSendsToConfiguredAdminRecipient(t *testing.T) {
	sender := &emailmock.EmailSender{}
	alerter := New(sender, "admin@example.com")

	if err := alerter.AlertMissedPeriod(entities.Client{ID: "client-a", Name: "Acme"}, entities.Period{ID: "2026-04"}, "missing reminder"); err != nil {
		t.Fatalf("AlertMissedPeriod returned error: %v", err)
	}

	if len(sender.Sent) != 1 {
		t.Fatalf("expected one email, got %+v", sender.Sent)
	}
	if got, want := sender.Sent[0].To, []string{"admin@example.com"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected recipients: got %#v want %#v", got, want)
	}
}
