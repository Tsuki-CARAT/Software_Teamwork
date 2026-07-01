import { ApiError } from '@/api/client'
import type { QACitation, QAThinkingStep } from '@/lib/types'

type StreamErrorLike = {
  code?: string
  message: string
  requestId?: string
  status?: number
}

type ToolEventKind = 'started' | 'completed' | 'failed'

type ToolStepView = {
  step: QAThinkingStep
  toolCallId?: string
}

const MISSING_REQUEST_ID_TEXT = '响应未包含 requestId，无法关联后端日志'
const NON_GATEWAY_REQUEST_ID_TEXT = '非 Gateway 错误，未包含 requestId'
const BLOCKED_SUMMARY_KEY_PARTS = [
  'apikey',
  'api_key',
  'argument',
  'internalurl',
  'internal_url',
  'objectkey',
  'object_key',
  'prompt',
  'providerraw',
  'provider_raw',
  'raw',
  'secret',
  'storage',
  'token',
  'url',
]
const BLOCKED_SUMMARY_VALUE_PATTERNS = [
  /\bapi[_-]?key\b/i,
  /\bauthorization\b/i,
  /\bbearer\s+[a-z0-9._-]+/i,
  /\b(?:developer|full|hidden|system)\s+prompt\b/i,
  /\b(?:localhost|127\.0\.0\.1|10\.\d{1,3}\.|172\.(?:1[6-9]|2\d|3[01])\.|192\.168\.)/i,
  /\bobject\s*key\b/i,
  /\bprompt\s*[:=]/i,
  /\bprovider\s+raw\b/i,
  /\braw\s+(?:body|error|response|result)\b/i,
  /\bsecret\b/i,
  /\btoken\b/i,
  /\bhttps?:\/\//i,
  /\bminio\b/i,
]
const SAFE_SUMMARY_LABELS: Record<string, string> = {
  chunkCount: '片段数',
  hitCount: '命中数',
  iterationNo: '迭代',
  knowledgeBaseCount: '知识库数',
  queryCount: '查询数',
  rerankTopN: '重排序 TopN',
  resultCount: '结果数',
  topK: 'TopK',
}
const SAFE_STREAM_ERROR_MESSAGES: Record<string, string> = {
  cancelled: '请求已取消',
  dependency_error: '依赖服务暂不可用，当前回复可能已降级',
  forbidden: '权限不足，请联系管理员开通访问权限',
  internal_error: '服务暂时无法完成回复',
  invalid_sse_event: '收到无法解析的流式事件',
  model_error: '模型服务暂不可用',
  network_error: '网络连接中断',
  not_implemented: '后端工作流尚未就绪',
  stream_ended_without_completion: '流式回复未正常完成',
  timeout: '请求超时',
  validation_error: '请求参数无效',
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function getString(record: Record<string, unknown>, key: string): string | undefined {
  const value = record[key]
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function getNumber(record: Record<string, unknown>, key: string): number | undefined {
  const value = record[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined
}

function getRequestIdText(requestId?: string): string {
  return requestId ? `requestId: ${requestId}` : MISSING_REQUEST_ID_TEXT
}

function isNotReady(error: ApiError | StreamErrorLike): boolean {
  return error.status === 501 || error.code === 'not_implemented' || error.code === 'http_501'
}

function isDependencyFailure(error: ApiError | StreamErrorLike): boolean {
  return error.status === 502 || error.code === 'dependency_error'
}

function isForbidden(error: ApiError | StreamErrorLike): boolean {
  return error.status === 403 || error.code === 'forbidden'
}

function formatApiError(error: ApiError | StreamErrorLike, featureName: string): string {
  const requestIdText = getRequestIdText(error.requestId)
  const safeMessage =
    error.code && SAFE_STREAM_ERROR_MESSAGES[error.code]
      ? SAFE_STREAM_ERROR_MESSAGES[error.code]
      : '请稍后重试或联系管理员'

  if (isNotReady(error)) {
    return `${featureName}暂未就绪：Gateway 已暴露契约，但后端工作流尚未就绪。（${requestIdText}）`
  }

  if (isDependencyFailure(error)) {
    return `${featureName}降级：${safeMessage}。（${requestIdText}）`
  }

  if (isForbidden(error)) {
    return `${featureName}权限不足：${safeMessage}。（${requestIdText}）`
  }

  return `${featureName}失败：${safeMessage}。（${requestIdText}）`
}

function isBlockedSummaryKey(key: string): boolean {
  const normalized = key.replaceAll(/[-.\s]/g, '_').toLowerCase()
  return BLOCKED_SUMMARY_KEY_PARTS.some((part) => normalized.includes(part))
}

function isBlockedSummaryValue(value: string): boolean {
  return BLOCKED_SUMMARY_VALUE_PATTERNS.some((pattern) => pattern.test(value))
}

function formatSummaryValue(value: unknown): string | undefined {
  const formatted =
    typeof value === 'string'
      ? value.trim()
      : typeof value === 'number' && Number.isFinite(value)
        ? String(value)
        : typeof value === 'boolean'
          ? value
            ? 'true'
            : 'false'
          : ''

  if (!formatted || isBlockedSummaryValue(formatted)) return undefined
  return formatted
}

function formatAllowedSummaryEntry(key: string, value: unknown): string | undefined {
  const label = SAFE_SUMMARY_LABELS[key]
  if (!label) return undefined
  const formatted = formatSummaryValue(value)
  return formatted ? `${label}: ${formatted}` : undefined
}

function formatSummaryObject(value: unknown): string | undefined {
  if (!isRecord(value)) return undefined

  const parts = Object.entries(value)
    .map(([key, entryValue]) => {
      if (isBlockedSummaryKey(key)) return undefined
      return formatAllowedSummaryEntry(key, entryValue)
    })
    .filter((part): part is string => Boolean(part))
    .slice(0, 4)

  return parts.length > 0 ? parts.join('，') : undefined
}

export function formatQAError(error: unknown, featureName: string): string {
  if (error instanceof ApiError) return formatApiError(error, featureName)
  return `${featureName}失败：请求未能完成。（${NON_GATEWAY_REQUEST_ID_TEXT}）`
}

export function formatQAStreamError(error: StreamErrorLike): string {
  return formatApiError(error, 'QA 流式回复')
}

export function createSafeToolStep(kind: ToolEventKind, payload: unknown): ToolStepView {
  const data = isRecord(payload) ? payload : {}
  const rawToolName = getString(data, 'toolName') ?? getString(data, 'tool')
  const toolName = rawToolName && !isBlockedSummaryValue(rawToolName) ? rawToolName : '工具调用'
  const toolCallId = getString(data, 'toolCallId')
  const latencyMs = getNumber(data, 'latencyMs')
  const summary =
    formatSummaryObject(data.argumentsSummary) ?? formatSummaryObject(data.resultSummary)
  const errorCode = getString(data, 'errorCode')
  const errorMessage = getString(data, 'errorMessage')
  const detailParts = [
    summary,
    kind === 'failed' && errorCode && !isBlockedSummaryValue(errorCode)
      ? `错误码: ${errorCode}`
      : undefined,
    kind === 'failed' && errorMessage && !isBlockedSummaryValue(errorMessage)
      ? `错误: ${errorMessage}`
      : undefined,
    latencyMs != null ? `耗时 ${latencyMs}ms` : undefined,
  ].filter((part): part is string => Boolean(part))

  const statusMap: Record<ToolEventKind, QAThinkingStep['status']> = {
    completed: 'done',
    failed: 'failed',
    started: 'running',
  }
  const labelMap: Record<ToolEventKind, string> = {
    completed: `${toolName} 完成`,
    failed: `${toolName} 失败`,
    started: `${toolName} 执行中`,
  }

  return {
    step: {
      detail: detailParts.join('；') || undefined,
      label: labelMap[kind],
      status: statusMap[kind],
      type: 'tool_call',
    },
    toolCallId,
  }
}

export function getSafeReasoningStep(payload: unknown): QAThinkingStep | undefined {
  if (!isRecord(payload)) return undefined
  const step = isRecord(payload.step) ? payload.step : payload
  const type = getString(step, 'type')
  const status = getString(step, 'status')

  if (
    !type ||
    !status ||
    !['agent_iteration', 'tool_call', 'tool_result', 'generation', 'citation', 'verify'].includes(
      type,
    ) ||
    !['pending', 'running', 'done', 'failed'].includes(status)
  ) {
    return undefined
  }

  const rawLabel = getString(step, 'label')
  const rawDetail = getString(step, 'detail')

  return {
    detail: rawDetail && !isBlockedSummaryValue(rawDetail) ? rawDetail : undefined,
    label: rawLabel && !isBlockedSummaryValue(rawLabel) ? rawLabel : type,
    status: status as QAThinkingStep['status'],
    type: type as QAThinkingStep['type'],
  }
}

export function getCitationDelta(payload: unknown): QACitation | undefined {
  if (!isRecord(payload) || !isRecord(payload.citation)) return undefined
  const citation = payload.citation
  const id = getString(citation, 'id')
  if (!id) return undefined
  return citation as QACitation
}

export function getCitationAvailabilityText(citation: QACitation): string {
  if (citation.isSourceAvailable === false) {
    return '来源详情暂不可用；当前仅展示 QA 保存的引用快照。'
  }

  return '引用详情以后端 citation snapshot 为准；详情接口未就绪时不展示补全文本。'
}
