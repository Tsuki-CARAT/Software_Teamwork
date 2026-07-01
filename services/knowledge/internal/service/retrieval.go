package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	maxKnowledgeQueryLength        = 2000
	maxRetrievalTopK               = 100
	retrievalKnowledgeBasePageSize = 200
	retrievalCandidateMultiplier   = 5
	maxVectorCandidateLimit        = 500
	defaultContentPreview          = 500
)

type KnowledgeQueryInput struct {
	Query            string
	KnowledgeBaseIDs []string
	TopK             int
	ScoreThreshold   *float64
	Tags             []string
	MetadataFilter   map[string]string
	Rerank           bool
	RerankTopN       *int
}

type KnowledgeQuerySummary struct {
	ID      string
	Query   string
	Results []KnowledgeQueryResult
	Trace   KnowledgeQueryTrace
}

type KnowledgeQueryResult struct {
	Score           float64
	KnowledgeBaseID string
	DocumentID      string
	ChunkID         string
	DocumentName    string
	SectionPath     *string
	ChunkIndex      *int
	ChunkType       *string
	ContentPreview  string
	Tags            []string
	rerankContent   string
}

type KnowledgeQueryTrace struct {
	EmbeddingProvider  string
	EmbeddingModel     string
	EmbeddingDimension int
	QdrantCollection   string
	SearchTopK         int
	ScoreThreshold     float64
	HitCount           int
	Rerank             bool
	RerankTopN         *int
}

func (s *Service) CreateKnowledgeQuery(ctx context.Context, reqCtx RequestContext, input KnowledgeQueryInput) (KnowledgeQuerySummary, error) {
	scope, err := readScope(reqCtx)
	if err != nil {
		return KnowledgeQuerySummary{}, err
	}

	query := strings.TrimSpace(input.Query)
	fields := map[string]string{}
	if query == "" {
		fields["query"] = "is required"
	} else if len(query) > maxKnowledgeQueryLength {
		fields["query"] = fmt.Sprintf("must be at most %d characters", maxKnowledgeQueryLength)
	}
	topK := input.TopK
	if topK == 0 {
		topK = s.retrievalTopK
	}
	if topK < 1 || topK > maxRetrievalTopK {
		fields["topK"] = fmt.Sprintf("must be between 1 and %d", maxRetrievalTopK)
	}
	scoreThreshold := s.scoreThreshold
	if input.ScoreThreshold != nil {
		scoreThreshold = *input.ScoreThreshold
	}
	if scoreThreshold < 0 {
		fields["scoreThreshold"] = "must be non-negative"
	}
	rerankTopN := cloneIntPointer(input.RerankTopN)
	if rerankTopN != nil && (*rerankTopN < 1 || *rerankTopN > topK) {
		fields["rerankTopN"] = "must be between 1 and topK"
	}
	tags, tagFields := normalizeTags(input.Tags)
	for key, value := range tagFields {
		fields[key] = value
	}
	metadataFilter := normalizeMetadataFilter(input.MetadataFilter)
	if len(fields) > 0 {
		return KnowledgeQuerySummary{}, ValidationError("request validation failed", fields)
	}
	if s.embedder == nil || s.vectorIndex == nil {
		return KnowledgeQuerySummary{}, DependencyError("retrieval pipeline is not configured", nil)
	}

	allowedKnowledgeBaseIDs, err := s.resolveRetrievalKnowledgeBases(ctx, scope, input.KnowledgeBaseIDs)
	if err != nil {
		return KnowledgeQuerySummary{}, err
	}
	if len(allowedKnowledgeBaseIDs) == 0 {
		queryID := s.newID("kq")
		candidateLimit := retrievalCandidateLimit(topK)
		return KnowledgeQuerySummary{
			ID:      queryID,
			Query:   query,
			Results: []KnowledgeQueryResult{},
			Trace:   s.baseKnowledgeQueryTrace(candidateLimit, scoreThreshold, input.Rerank, rerankTopN),
		}, nil
	}

	embedding, err := s.embedder.Embed(ctx, EmbeddingRequest{
		Texts:     []string{query},
		UserID:    strings.TrimSpace(reqCtx.UserID),
		RequestID: strings.TrimSpace(reqCtx.RequestID),
	})
	if err != nil {
		return KnowledgeQuerySummary{}, DependencyError("knowledge query embedding failed", err)
	}
	if len(embedding.Vectors) != 1 {
		return KnowledgeQuerySummary{}, DependencyError("knowledge query embedding failed", nil)
	}

	candidateLimit := retrievalCandidateLimit(topK)
	hits, err := s.vectorIndex.Search(ctx, VectorSearchRequest{
		Vector:           embedding.Vectors[0],
		KnowledgeBaseIDs: allowedKnowledgeBaseIDs,
		Tags:             tags,
		MetadataFilter:   metadataFilter,
		Limit:            candidateLimit,
		ScoreThreshold:   scoreThreshold,
	})
	if err != nil {
		return KnowledgeQuerySummary{}, DependencyError("knowledge vector search failed", err)
	}

	results, err := s.hydrateRetrievalResults(ctx, reqCtx, hits, scoreThreshold, tags, metadataFilter)
	if err != nil {
		return KnowledgeQuerySummary{}, err
	}
	if len(results) > topK {
		results = results[:topK]
	}
	if input.Rerank {
		results, err = s.rerankRetrievalResults(ctx, reqCtx, query, results, rerankTopN)
		if err != nil {
			return KnowledgeQuerySummary{}, err
		}
	}

	queryID := s.newID("kq")
	trace := s.baseKnowledgeQueryTrace(candidateLimit, scoreThreshold, input.Rerank, rerankTopN)
	trace.EmbeddingProvider = strings.TrimSpace(embedding.Provider)
	trace.EmbeddingModel = strings.TrimSpace(embedding.Model)
	trace.EmbeddingDimension = embedding.Dimension
	trace.HitCount = len(results)
	trace = normalizeKnowledgeQueryTrace(trace)
	return KnowledgeQuerySummary{
		ID:      queryID,
		Query:   query,
		Results: results,
		Trace:   trace,
	}, nil
}

func (s *Service) baseKnowledgeQueryTrace(topK int, scoreThreshold float64, rerank bool, rerankTopN *int) KnowledgeQueryTrace {
	trace := KnowledgeQueryTrace{
		EmbeddingProvider:  "configured",
		EmbeddingModel:     "configured",
		EmbeddingDimension: 1,
		QdrantCollection:   strings.TrimSpace(s.vectorCollection),
		SearchTopK:         topK,
		ScoreThreshold:     scoreThreshold,
		HitCount:           0,
		Rerank:             rerank,
		RerankTopN:         cloneIntPointer(rerankTopN),
	}
	return normalizeKnowledgeQueryTrace(trace)
}

func retrievalCandidateLimit(topK int) int {
	limit := topK * retrievalCandidateMultiplier
	if limit < topK {
		limit = topK
	}
	if limit > maxVectorCandidateLimit {
		limit = maxVectorCandidateLimit
	}
	return limit
}

func normalizeKnowledgeQueryTrace(trace KnowledgeQueryTrace) KnowledgeQueryTrace {
	if strings.TrimSpace(trace.EmbeddingProvider) == "" {
		trace.EmbeddingProvider = "configured"
	}
	if strings.TrimSpace(trace.EmbeddingModel) == "" {
		trace.EmbeddingModel = "configured"
	}
	if trace.EmbeddingDimension < 1 {
		trace.EmbeddingDimension = 1
	}
	if strings.TrimSpace(trace.QdrantCollection) == "" {
		trace.QdrantCollection = DefaultVectorCollection
	}
	return trace
}

func (s *Service) rerankRetrievalResults(ctx context.Context, reqCtx RequestContext, query string, results []KnowledgeQueryResult, topN *int) ([]KnowledgeQueryResult, error) {
	limit := len(results)
	if topN != nil && *topN < limit {
		limit = *topN
	}
	if limit > maxRetrievalTopK {
		limit = maxRetrievalTopK
	}
	if limit == 0 {
		return []KnowledgeQueryResult{}, nil
	}
	// When the adapter is not configured, a nil Reranker is an explicit no-op
	// fallback: preserve vector order and never require provider credentials.
	if s.reranker == nil {
		return append([]KnowledgeQueryResult(nil), results[:limit]...), nil
	}

	documents := make([]RerankDocument, 0, len(results))
	byID := make(map[string]KnowledgeQueryResult, len(results))
	for _, result := range results {
		documents = append(documents, RerankDocument{ID: result.ChunkID, Text: result.rerankContent})
		byID[result.ChunkID] = result
	}
	reranked, err := s.reranker.Rerank(ctx, RerankRequest{
		Query:     query,
		Documents: documents,
		TopN:      limit,
		UserID:    strings.TrimSpace(reqCtx.UserID),
		RequestID: strings.TrimSpace(reqCtx.RequestID),
	})
	if err != nil {
		return nil, DependencyError("knowledge query reranking failed", err)
	}

	ordered := make([]KnowledgeQueryResult, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, item := range reranked {
		result, ok := byID[strings.TrimSpace(item.DocumentID)]
		if !ok {
			continue
		}
		if _, duplicate := seen[result.ChunkID]; duplicate {
			continue
		}
		result.Score = item.Score
		ordered = append(ordered, result)
		seen[result.ChunkID] = struct{}{}
		if len(ordered) == limit {
			break
		}
	}
	for _, result := range results {
		if len(ordered) == limit {
			break
		}
		if _, alreadyRanked := seen[result.ChunkID]; alreadyRanked {
			continue
		}
		ordered = append(ordered, result)
		seen[result.ChunkID] = struct{}{}
	}
	return ordered, nil
}

func (s *Service) resolveRetrievalKnowledgeBases(ctx context.Context, scope AccessScope, ids []string) ([]string, error) {
	normalized := make([]string, 0, len(ids))
	seen := map[string]struct{}{}
	for _, raw := range ids {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		_, err := s.repo.GetKnowledgeBase(ctx, id, scope)
		if err != nil {
			return nil, repositoryError(err)
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	if len(normalized) > 0 {
		return normalized, nil
	}

	for page := 1; ; page++ {
		list, err := s.repo.ListKnowledgeBases(ctx, scope, PageInput{Page: page, PageSize: retrievalKnowledgeBasePageSize})
		if err != nil {
			return nil, DependencyError("knowledge base metadata access failed", err)
		}
		for _, base := range list.Items {
			if _, exists := seen[base.ID]; exists {
				continue
			}
			seen[base.ID] = struct{}{}
			normalized = append(normalized, base.ID)
		}
		if len(list.Items) == 0 {
			break
		}
		if len(list.Items) < retrievalKnowledgeBasePageSize {
			break
		}
	}
	return normalized, nil
}

func (s *Service) hydrateRetrievalResults(ctx context.Context, reqCtx RequestContext, hits []VectorSearchHit, scoreThreshold float64, tags []string, metadataFilter map[string]string) ([]KnowledgeQueryResult, error) {
	if len(hits) == 0 {
		return []KnowledgeQueryResult{}, nil
	}
	chunkIDs := make([]string, 0, len(hits))
	for _, hit := range hits {
		if chunkID := stringPayload(hit.Payload, "chunk_id"); chunkID != "" {
			chunkIDs = append(chunkIDs, chunkID)
		}
	}
	chunks, err := s.repo.FindChunksByIDs(ctx, chunkIDs)
	if err != nil {
		return nil, DependencyError("document chunks access failed", err)
	}
	chunksByID := make(map[string]DocumentChunk, len(chunks))
	for _, chunk := range chunks {
		chunksByID[chunk.ID] = chunk
	}

	results := make([]KnowledgeQueryResult, 0, len(hits))
	for _, hit := range hits {
		if hit.Score < scoreThreshold {
			continue
		}
		chunkID := stringPayload(hit.Payload, "chunk_id")
		chunk, ok := chunksByID[chunkID]
		if !ok {
			continue
		}
		scope, scopeErr := readScope(reqCtx)
		if scopeErr != nil {
			return nil, scopeErr
		}
		doc, err := s.repo.GetDocument(ctx, chunk.DocumentID, scope)
		if err != nil {
			if errorsIsNotFound(err) {
				continue
			}
			return nil, repositoryError(err)
		}
		if doc.Status != DocumentStatusReady || !containsAllTags(doc.Tags, tags) || !matchesChunkMetadata(chunk.Metadata, metadataFilter) {
			continue
		}
		chunkIndex := int(chunk.ChunkIndex)
		results = append(results, KnowledgeQueryResult{
			Score:           hit.Score,
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			DocumentID:      chunk.DocumentID,
			ChunkID:         chunk.ID,
			DocumentName:    doc.Name,
			SectionPath:     cloneStringPointer(chunk.SectionPath),
			ChunkIndex:      &chunkIndex,
			ChunkType:       cloneStringPointer(chunk.ChunkType),
			ContentPreview:  contentPreview(chunk.Content, defaultContentPreview),
			Tags:            append([]string(nil), doc.Tags...),
			rerankContent:   chunk.Content,
		})
	}
	return results, nil
}

func containsAllTags(documentTags []string, required []string) bool {
	if len(required) == 0 {
		return true
	}
	available := make(map[string]struct{}, len(documentTags))
	for _, tag := range documentTags {
		available[strings.TrimSpace(tag)] = struct{}{}
	}
	for _, tag := range required {
		if _, ok := available[tag]; !ok {
			return false
		}
	}
	return true
}

func matchesChunkMetadata(metadata map[string]any, filter map[string]string) bool {
	for key, expected := range filter {
		actual, ok := metadata[key]
		if !ok || strings.TrimSpace(fmt.Sprint(actual)) != expected {
			return false
		}
	}
	return true
}

func normalizeMetadataFilter(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	filter := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		filter[key] = value
	}
	return filter
}

func stringPayload(payload map[string]any, key string) string {
	value, _ := payload[key].(string)
	return strings.TrimSpace(value)
}

func contentPreview(content string, limit int) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= limit {
		return content
	}
	return string(runes[:limit])
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneIntPointer(value *int) *int {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
