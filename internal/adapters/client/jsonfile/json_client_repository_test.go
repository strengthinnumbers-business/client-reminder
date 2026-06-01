package jsonfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestClientRepositoryLoadsEmailsArrayWithoutNormalizingValues(t *testing.T) {
	path := writeClientsFile(t, `[
  {
    "ID": "client-a",
    "Emails": [" ops@example.com ", "finance@example.com,other@example.com"]
  }
]`)

	clients, err := New(path).GetAllClients()
	if err != nil {
		t.Fatalf("GetAllClients returned error: %v", err)
	}

	encoded, err := json.Marshal(clients[0])
	if err != nil {
		t.Fatalf("marshal loaded client: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("decode loaded client: %v", err)
	}

	got := fields["Emails"]
	want := []any{" ops@example.com ", "finance@example.com,other@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected Emails: got %#v want %#v", got, want)
	}
}

func TestClientRepositoryRejectsClientWithoutEmails(t *testing.T) {
	path := writeClientsFile(t, `[{"ID":"client-a","Emails":[]}]`)

	_, err := New(path).GetAllClients()
	if err == nil || !strings.Contains(err.Error(), "client client-a has no email recipients") {
		t.Fatalf("expected missing recipients error, got %v", err)
	}
}

func writeClientsFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "clients.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write clients file: %v", err)
	}
	return path
}
