package repository

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/repository/sqlc"
	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

func TestConversationFromRow(t *testing.T) {
	lastMessageAt := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	row := sqlc.ConversationSummaryRow{
		ID:                 "session-1",
		Title:              "demo",
		ExternalUserID:     "user-1",
		Status:             "active",
		CreatedAt:          lastMessageAt.Add(-time.Hour),
		UpdatedAt:          lastMessageAt,
		LastMessageAt:      pgtype.Timestamptz{Time: lastMessageAt, Valid: true},
		MessageCount:       3,
		LastMessagePreview: "hello",
	}
	conversation := conversationFromRow(row)
	if conversation.ID != row.ID || conversation.OwnerUserID != row.ExternalUserID {
		t.Fatalf("unexpected conversation mapping: %+v", conversation)
	}
	if conversation.LastMessageAt == nil || !conversation.LastMessageAt.Equal(lastMessageAt) {
		t.Fatalf("expected last message timestamp, got %+v", conversation.LastMessageAt)
	}
	if conversation.MessageCount != 3 || conversation.LastMessagePreview != "hello" {
		t.Fatalf("unexpected summary fields: %+v", conversation)
	}
}

func TestListConversationsParams(t *testing.T) {
	params, err := listConversationsParams("user-1", service.ConversationListOptions{
		Page:     2,
		PageSize: 10,
		Status:   "active",
		Sort:     "-updatedAt",
	})
	if err != nil {
		t.Fatal(err)
	}
	if params.ExternalUserID != "user-1" || params.Status != "active" {
		t.Fatalf("unexpected params: %+v", params)
	}
	if params.PageSize != 10 || params.PageOffset != 10 {
		t.Fatalf("unexpected pagination: %+v", params)
	}
}

func TestListConversationsParamsRejectsInvalidPagination(t *testing.T) {
	cases := []service.ConversationListOptions{
		{Page: 0, PageSize: 10},
		{Page: 1, PageSize: 0},
		{Page: 214748366, PageSize: 10},
	}
	for _, options := range cases {
		if _, err := listConversationsParams("user-1", options); err == nil {
			t.Fatalf("listConversationsParams(%+v) error = nil, want validation error", options)
		}
	}
}
