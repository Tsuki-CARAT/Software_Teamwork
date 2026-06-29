/**
 * Admin API — API doc sections 6 (config), 7 (stats), 8 (system mgmt).
 */

import type {
  KnowledgeBaseConfig,
  LLMConfig,
  RAGDefaults,
  RAGSearchResult,
  StatsOverview,
  TopQuery,
  TrendPoint,
} from '@/lib/types'

import { apiClient, ApiError, doRequest } from './client'

// ---------------------------------------------------------------------------
// Local response types not yet in @/lib/types
// ---------------------------------------------------------------------------

export interface LLMConfigResponse extends LLMConfig {
  extra_headers: Record<string, string>
  updated_at: string
}

export interface LLMTestRequest {
  api_url: string
  api_key: string
  model_name: string
}

export interface LLMTestResponse {
  success: boolean
  latency_ms: number
  model: string
  tested_at: string
}

export interface KnowledgeConfigResponse {
  knowledge_bases: KnowledgeBaseConfig[]
  defaults: RAGDefaults
}

export interface RAGTestRequest {
  query: string
  knowledge_bases_override?: string[] | null
  top_k_override?: number | null
  similarity_threshold_override?: number | null
}

export interface RAGTestResponse {
  query: string
  mode: string
  results: RAGSearchResult[]
  total_hits: number
  took_ms: number
}

export interface StatsTrendResponse {
  days: number
  points: TrendPoint[]
}

export interface TopQueriesResponse {
  items: TopQuery[]
}

export interface IntentDistributionItem {
  intent: string
  label: string
  count: number
  percent: number
}

export interface IntentDistributionResponse {
  items: IntentDistributionItem[]
}

export interface CreateKnowledgeBaseRequest {
  name: string
  description?: string
}

// Stub types for user/role management (API doc section 8, reserved)
export interface AdminUser {
  id: string
  username: string
  role: string
  created_at: string
}

export interface AdminRole {
  id: string
  name: string
  permissions: string[]
}

// =========================================================================
// 6.1  LLM Configuration
// =========================================================================

/** GET /api/admin/llm-config */
export function getLLMConfig(): Promise<LLMConfigResponse> {
  return doRequest<LLMConfigResponse>('/admin/llm-config')
}

/** PUT /api/admin/llm-config */
export async function updateLLMConfig(config: Partial<LLMConfig>): Promise<LLMConfigResponse> {
  return doRequest<LLMConfigResponse>('/admin/llm-config', {
    method: 'PUT',
    body: JSON.stringify(config),
  })
}

/** POST /api/admin/llm-config/test */
export async function testLLMConnection(params: LLMTestRequest): Promise<LLMTestResponse> {
  return doRequest<LLMTestResponse>('/admin/llm-config/test', {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

// =========================================================================
// 6.2  Knowledge Configuration
// =========================================================================

/** GET /api/admin/knowledge-config */
export function getKnowledgeConfig(): Promise<KnowledgeConfigResponse> {
  return doRequest<KnowledgeConfigResponse>('/admin/knowledge-config')
}

/** PUT /api/admin/knowledge-config */
export async function updateKnowledgeConfig(
  defaults: Partial<RAGDefaults>,
): Promise<KnowledgeConfigResponse> {
  return doRequest<KnowledgeConfigResponse>('/admin/knowledge-config', {
    method: 'PUT',
    body: JSON.stringify({ defaults }),
  })
}

// =========================================================================
// 6.3  RAG Test (admin)
// =========================================================================

/** POST /api/admin/rag/test */
export async function ragTest(params: RAGTestRequest): Promise<RAGTestResponse> {
  return doRequest<RAGTestResponse>('/admin/rag/test', {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

// =========================================================================
// 7  Statistics
// =========================================================================

/** GET /api/admin/stats/overview */
export function getStatsOverview(): Promise<StatsOverview> {
  return doRequest<StatsOverview>('/admin/stats/overview')
}

/** GET /api/admin/stats/trend?days=N */
export async function getStatsTrend(days = 30): Promise<StatsTrendResponse> {
  const res = await fetch(`${apiClient.baseUrl}/admin/stats/trend?days=${days}`)
  if (!res.ok) throw new ApiError(res.status, '获取趋势数据失败')
  const json: { code: number; message: string; data: StatsTrendResponse } = await res.json()
  if (json.code !== 0) throw new ApiError(json.code, json.message)
  return json.data
}

/** GET /api/admin/stats/top-queries?limit=N&days=N */
export async function getTopQueries(limit = 10, daysParam = 7): Promise<TopQueriesResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
    days: String(daysParam),
  })
  const res = await fetch(`${apiClient.baseUrl}/admin/stats/top-queries?${params}`)
  if (!res.ok) throw new ApiError(res.status, '获取热门问题失败')
  const json: { code: number; message: string; data: TopQueriesResponse } = await res.json()
  if (json.code !== 0) throw new ApiError(json.code, json.message)
  return json.data
}

/** GET /api/admin/stats/intent-distribution?days=N */
export async function getIntentDistribution(daysParam = 7): Promise<IntentDistributionResponse> {
  const res = await fetch(`${apiClient.baseUrl}/admin/stats/intent-distribution?days=${daysParam}`)
  if (!res.ok) throw new ApiError(res.status, '获取意图分布失败')
  const json: {
    code: number
    message: string
    data: IntentDistributionResponse
  } = await res.json()
  if (json.code !== 0) throw new ApiError(json.code, json.message)
  return json.data
}

// =========================================================================
// 8.1  Knowledge Base CRUD
// =========================================================================

/** GET /api/admin/knowledge-bases */
export function listKnowledgeBases(): Promise<KnowledgeBaseConfig[]> {
  return doRequest<KnowledgeBaseConfig[]>('/admin/knowledge-bases')
}

/** POST /api/admin/knowledge-bases */
export async function createKnowledgeBase(
  params: CreateKnowledgeBaseRequest,
): Promise<KnowledgeBaseConfig> {
  return doRequest<KnowledgeBaseConfig>('/admin/knowledge-bases', {
    method: 'POST',
    body: JSON.stringify(params),
  })
}

/** DELETE /api/admin/knowledge-bases/:id */
export async function deleteKnowledgeBase(id: string): Promise<void> {
  await doRequest<void>(`/admin/knowledge-bases/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// =========================================================================
// 8.2  User Management (reserved — stubs)
// =========================================================================

export function listUsers(): Promise<AdminUser[]> {
  return doRequest<AdminUser[]>('/admin/users')
}

export async function createUser(body: Record<string, unknown>): Promise<AdminUser> {
  return doRequest<AdminUser>('/admin/users', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export async function updateUser(id: string, body: Record<string, unknown>): Promise<AdminUser> {
  return doRequest<AdminUser>(`/admin/users/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export async function deleteUser(id: string): Promise<void> {
  await doRequest<void>(`/admin/users/${encodeURIComponent(id)}`, { method: 'DELETE' })
}

// =========================================================================
// 8.3  Role Management (reserved — stubs)
// =========================================================================

export function listRoles(): Promise<AdminRole[]> {
  return doRequest<AdminRole[]>('/admin/roles')
}

export async function createRole(body: Record<string, unknown>): Promise<AdminRole> {
  return doRequest<AdminRole>('/admin/roles', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}

export async function updateRole(id: string, body: Record<string, unknown>): Promise<AdminRole> {
  return doRequest<AdminRole>(`/admin/roles/${encodeURIComponent(id)}`, {
    method: 'PUT',
    body: JSON.stringify(body),
  })
}

export async function deleteRole(id: string): Promise<void> {
  await doRequest<void>(`/admin/roles/${encodeURIComponent(id)}`, { method: 'DELETE' })
}
