/**
 * Citation endpoints — API doc sections 7.1 & 7.2.
 *
 * getCitation        GET  /api/citations/:chunk_id
 * batchGetCitations  POST /api/citations/batch
 */

import { doRequest } from './client'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface CitationDetail {
  chunk_id: string
  doc_id: string
  doc_name: string
  text: string
  context_before: string
  context_after: string
  page_number: number
  score: number
}

interface BatchCitationsRequest {
  chunk_ids: string[]
}

// ---------------------------------------------------------------------------
// 7.1  Single citation
// ---------------------------------------------------------------------------

/**
 * Retrieve full citation detail (including surrounding context) for a
 * single chunk.
 */
export async function getCitation(chunkId: string): Promise<CitationDetail> {
  return doRequest<CitationDetail>(`/citations/${encodeURIComponent(chunkId)}`)
}

// ---------------------------------------------------------------------------
// 7.2  Batch citations
// ---------------------------------------------------------------------------

/**
 * Retrieve citation details for multiple chunks at once.
 */
export async function batchGetCitations(chunkIds: string[]): Promise<CitationDetail[]> {
  return doRequest<CitationDetail[]>('/citations/batch', {
    method: 'POST',
    body: JSON.stringify({ chunk_ids: chunkIds } satisfies BatchCitationsRequest),
  })
}
