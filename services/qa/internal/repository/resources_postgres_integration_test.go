package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Sakayori-Iroha-168/Software_Teamwork/services/qa/internal/service"
)

func TestDocumentedResourceRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("QA_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("QA_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	repo, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	now := time.Now().UTC()
	suffix := uint64(now.UnixNano()) & 0xffffffffffff
	conversationID := integrationUUID(suffix)
	userMessageID := integrationUUID(suffix + 1)
	assistantMessageID := integrationUUID(suffix + 2)
	conversation := service.Conversation{ID: conversationID, OwnerUserID: "integration-user", Title: "contract", Status: "active", CreatedAt: now, UpdatedAt: now}
	if _, err = repo.CreateConversation(ctx, conversation); err != nil {
		t.Fatal(err)
	}
	run, err := repo.AppendMessages(ctx, "integration-user", conversationID, service.Message{ID: userMessageID, ConversationID: conversationID, Role: "user", Content: "question", Status: "completed", CreatedAt: now}, service.Message{ID: assistantMessageID, ConversationID: conversationID, Role: "assistant", Status: "streaming", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	events := []service.StreamEvent{{EventSeq: 1, EventType: "agent.iteration.started", Payload: map[string]any{"iterationNo": 1}, CreatedAt: now}, {EventSeq: 2, EventType: "tool.started", Payload: map[string]any{"iterationNo": 1, "toolCallId": "call-1", "tool": "search_knowledge"}, CreatedAt: now}, {EventSeq: 3, EventType: "tool.completed", Payload: map[string]any{"iterationNo": 1, "toolCallId": "call-1", "tool": "search_knowledge"}, CreatedAt: now.Add(time.Millisecond)}}
	if err = repo.SaveStreamEvents(ctx, "integration-user", run.ID, events); err != nil {
		t.Fatal(err)
	}
	invocationID, err := repo.SaveModelInvocation(ctx, "integration-user", service.ModelInvocation{
		ResponseRunID: run.ID, IterationNo: 1, Provider: "ai-gateway", ProfileID: "default",
		ModelName: "deepseek-v4-pro", FinishReason: "stop", Status: "completed",
		StartedAt: now, FinishedAt: ptrTime(now.Add(time.Millisecond)), LatencyMS: 1,
	})
	if err != nil || invocationID == "" {
		t.Fatalf("invocation=%q err=%v", invocationID, err)
	}
	rows, err := repo.queries.ListModelInvocationsByRun(ctx, run.ID, "integration-user")
	if err != nil || len(rows) != 1 || rows[0].Status != "completed" {
		t.Fatalf("invocations=%+v err=%v", rows, err)
	}
	replayed, err := repo.ListStreamEvents(ctx, "integration-user", conversationID, run.ID, 0)
	if err != nil || len(replayed) != 3 {
		t.Fatalf("events=%d err=%v", len(replayed), err)
	}
	calls, err := repo.ListToolCalls(ctx, "integration-user", run.ID)
	if err != nil || len(calls) != 1 || calls[0].Status != "completed" {
		t.Fatalf("calls=%+v err=%v", calls, err)
	}
	cancelled, err := repo.CancelResponseRun(ctx, "integration-user", run.ID)
	if err != nil || cancelled.Status != "cancelled" {
		t.Fatalf("run=%+v err=%v", cancelled, err)
	}
	qaConfig, err := repo.CreateQAConfigVersionResource(ctx, "integration-user", service.CreateQAConfigVersionInput{TopK: 7, MaxIterations: 6, KnowledgeBases: []service.ConfigKnowledgeBase{{ID: "kb-1"}}})
	if err != nil || qaConfig.Retrieval.TopK != 7 || qaConfig.MaxIterations != 6 || qaConfig.Agent.MaxIterations != 6 {
		t.Fatalf("qa config=%+v err=%v", qaConfig, err)
	}
	llmConfig, err := repo.CreateLLMConfigVersionResource(ctx, "integration-user", service.CreateLLMConfigVersionInput{Provider: "ai-gateway", ProfileID: "profile-chat", ModelName: "model", TimeoutSeconds: 30, MaxTokens: 512})
	if err != nil || llmConfig.ProfileID != "profile-chat" {
		t.Fatalf("llm config=%+v err=%v", llmConfig, err)
	}
	retrieval, err := repo.SaveRetrievalTestRun(ctx, "integration-user", service.RetrievalTestInput{Question: "query"}, []service.RetrievalTestResult{{KnowledgeBaseID: "kb-1", DocumentID: "doc-1", ChunkID: "chunk-1", VectorScore: .9, ContentPreview: "preview", Metadata: map[string]any{}}}, time.Millisecond, nil)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := repo.GetRetrievalTestRun(ctx, "integration-user", retrieval.ID)
	if err != nil || len(loaded.Results) != 1 {
		t.Fatalf("retrieval=%+v err=%v", loaded, err)
	}
	if _, err = repo.GetMetricsOverview(ctx, 1); err != nil {
		t.Fatal(err)
	}
}

func integrationUUID(value uint64) string { return fmt.Sprintf("00000000-0000-4000-8000-%012x", value) }

func ptrTime(value time.Time) *time.Time { return &value }
