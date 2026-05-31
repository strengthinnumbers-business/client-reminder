package demolog

import "testing"

func TestSourceMapKeyUsesStableShortHash(t *testing.T) {
	key := SourceMapKey(Source{
		Path:     "internal/core/service/reminder_service.go",
		Line:     99,
		Function: "github.com/strengthinnumbers-business/client-reminder/internal/core/service.(*ReminderService).Run",
		Message:  "loaded active clients",
	})

	if key != "8c3f797e8c15" {
		t.Fatalf("expected stable short key, got %q", key)
	}
}

func TestSourceMapKeyChangesWhenSourceChanges(t *testing.T) {
	first := SourceMapKey(Source{
		Path:     "internal/core/service/reminder_service.go",
		Line:     99,
		Function: "Run",
		Message:  "loaded active clients",
	})
	second := SourceMapKey(Source{
		Path:     "internal/core/service/reminder_service.go",
		Line:     100,
		Function: "Run",
		Message:  "loaded active clients",
	})

	if first == second {
		t.Fatalf("expected line changes to affect source map key %q", first)
	}
}
