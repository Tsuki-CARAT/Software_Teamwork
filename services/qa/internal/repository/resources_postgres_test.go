package repository

import (
	"math"
	"strings"
	"testing"
)

func TestStreamEventSeqInt32RejectsInvalidValues(t *testing.T) {
	if _, err := streamEventSeqInt32(-1); err == nil {
		t.Fatal("expected negative cursor to fail")
	}
	if _, err := streamEventSeqInt32(math.MaxInt32 + 1); err == nil {
		t.Fatal("expected overflow cursor to fail")
	}
	if got, err := streamEventSeqInt32(math.MaxInt32); err != nil || got != math.MaxInt32 {
		t.Fatalf("streamEventSeqInt32(MaxInt32) = %d, %v", got, err)
	}
}

func TestMessageCitationLegacySelectDoesNotRequireSnapshotMigrationColumns(t *testing.T) {
	for _, column := range []string{
		"ci.response_run_id",
		"ci.content_preview",
		"ci.is_source_available",
		"ci.source_unavailable_reason",
	} {
		if strings.Contains(messageCitationLegacySelect, column) {
			t.Fatalf("legacy message citation query should not require migration 0006 column %q: %s", column, messageCitationLegacySelect)
		}
	}
	if strings.Contains(messageCitationLegacySelect, "FALSE AS is_source_available") {
		t.Fatalf("legacy message citation query should not hard-code source availability to false: %s", messageCitationLegacySelect)
	}
}
