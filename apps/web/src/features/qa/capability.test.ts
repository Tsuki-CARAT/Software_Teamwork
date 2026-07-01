import { describe, expect, it } from 'vitest'

import { ApiError } from '@/api/client'
import type { QACitation } from '@/lib/types'

import {
  createSafeToolStep,
  formatQAError,
  formatQAStreamError,
  getCitationAvailabilityText,
  getCitationDelta,
  getSafeReasoningStep,
} from './capability'

describe('QA capability helpers', () => {
  it('formats readiness and dependency errors with request id state', () => {
    expect(
      formatQAError(
        new ApiError({
          code: 'not_implemented',
          message: 'route pending',
          requestId: 'req-501',
          status: 501,
        }),
        'RAG 检索',
      ),
    ).toContain('requestId: req-501')

    expect(
      formatQAStreamError({
        code: 'dependency_error',
        message: 'knowledge unavailable',
        status: 502,
      }),
    ).toContain('响应未包含 requestId')
  })

  it('formats forbidden errors as permission denials', () => {
    const formatted = formatQAError(
      new ApiError({
        code: 'forbidden',
        message: 'not allowed',
        requestId: 'req-403',
        status: 403,
      }),
      'QA 会话列表',
    )

    expect(formatted).toContain('权限不足')
    expect(formatted).toContain('requestId: req-403')
    expect(formatted).not.toContain('稍后重试')
  })

  it('does not expose backend raw error messages in user-visible text', () => {
    const formatted = formatQAStreamError({
      code: 'dependency_error',
      message: 'provider raw error includes http://10.0.0.2/minio/private-object',
      requestId: 'req-safe',
      status: 502,
    })

    expect(formatted).toContain('依赖服务暂不可用')
    expect(formatted).toContain('requestId: req-safe')
    expect(formatted).not.toContain('provider raw')
    expect(formatted).not.toContain('10.0.0.2')
    expect(formatted).not.toContain('private-object')
  })

  it('builds tool steps from sanitized summary fields without dumping raw payloads', () => {
    const view = createSafeToolStep('completed', {
      argumentsSummary: {
        bucket: 'qa-prod-files',
        internalPreview: 'http://10.0.0.5/minio/private/object',
        objectKey: 'secret/minio/key',
        prompt: 'full hidden prompt',
        queryCount: 3,
        sourcePath: 'tenant-a/private/doc.pdf',
      },
      latencyMs: 120,
      rawResult: 'provider raw response',
      resultSummary: { documentUri: 's3://qa-prod-files/private/doc.pdf', hitCount: 2 },
      toolCallId: 'tool-1',
      toolName: 'search_knowledge',
    })

    expect(view.toolCallId).toBe('tool-1')
    expect(view.step).toMatchObject({
      label: 'search_knowledge 完成',
      status: 'done',
      type: 'tool_call',
    })
    expect(view.step.detail).toContain('查询数: 3')
    expect(view.step.detail).not.toContain('queryCount')
    expect(view.step.detail).not.toContain('qa-prod-files')
    expect(view.step.detail).not.toContain('tenant-a/private')
    expect(view.step.detail).not.toContain('s3://')
    expect(view.step.detail).not.toContain('secret/minio/key')
    expect(view.step.detail).not.toContain('full hidden prompt')
    expect(view.step.detail).not.toContain('http://10.0.0.5')
    expect(view.step.detail).not.toContain('provider raw response')
  })

  it('does not display free-text tool summaries that may leak sensitive details', () => {
    const view = createSafeToolStep('failed', {
      errorCode: 'dependency_error',
      errorMessage: 'provider raw error body includes http://10.0.0.2/internal',
      latencyMs: 30,
      summary: 'prompt: hidden system prompt http://10.0.0.1/minio/bucket/object',
      toolSummary: 'safe-looking but unstructured text from backend',
      toolName: 'search_knowledge',
    })

    expect(view.step.detail).toContain('dependency_error')
    expect(view.step.detail).toContain('30ms')
    expect(view.step.detail).not.toContain('hidden system prompt')
    expect(view.step.detail).not.toContain('10.0.0.1')
    expect(view.step.detail).not.toContain('10.0.0.2')
    expect(view.step.detail).not.toContain('safe-looking but unstructured')
    expect(view.step.detail).not.toContain('provider raw error body')
  })

  it('accepts current backend tool and flat reasoning event fields', () => {
    const toolView = createSafeToolStep('started', {
      argumentsSummary: { queryCount: 2 },
      tool: 'search_knowledge',
      toolCallId: 'tool-2',
    })

    expect(toolView.toolCallId).toBe('tool-2')
    expect(toolView.step).toMatchObject({
      label: expect.stringContaining('search_knowledge'),
      status: 'running',
      type: 'tool_call',
    })
    expect(toolView.step.detail).toContain('查询数: 2')

    expect(
      getSafeReasoningStep({
        detail: 'using retrieved citation snapshots',
        label: '整理引用',
        status: 'done',
        type: 'citation',
      }),
    ).toMatchObject({
      detail: 'using retrieved citation snapshots',
      label: '整理引用',
      status: 'done',
      type: 'citation',
    })
  })

  it('sanitizes reasoning label and detail before display', () => {
    expect(
      getSafeReasoningStep({
        detail: 'provider raw error includes http://10.0.0.2/internal',
        label: 'system prompt: hidden chain',
        status: 'done',
        type: 'generation',
      }),
    ).toMatchObject({
      detail: undefined,
      label: 'generation',
      status: 'done',
      type: 'generation',
    })
  })

  it('accepts only structured reasoning and citation payloads', () => {
    expect(
      getSafeReasoningStep({
        step: {
          detail: '已生成脱敏摘要',
          label: '检索摘要',
          status: 'done',
          type: 'citation',
        },
      }),
    ).toMatchObject({ detail: '已生成脱敏摘要', status: 'done', type: 'citation' })

    expect(
      getSafeReasoningStep({ step: { status: 'done', type: 'private_chain_of_thought' } }),
    ).toBeUndefined()

    const citation = getCitationDelta({
      citation: {
        id: 'cit-1',
        citationNo: 1,
        docId: 'DOC-001',
        docName: '电力变压器巡检手册.pdf',
        isSourceAvailable: false,
        score: 0.96,
        text: '变压器外壳应保持清洁...',
      },
    })

    expect(citation).toMatchObject({
      citationNo: 1,
      docId: 'DOC-001',
      id: 'cit-1',
      text: '变压器外壳应保持清洁...',
    })
    expect(getCitationDelta({ citation: { messageId: 'msg-1' } })).toBeUndefined()
  })

  it('keeps citation detail readiness explicit', () => {
    const citation: QACitation = {
      id: 'cit-1',
      isSourceAvailable: false,
      messageId: 'msg-1',
    }

    expect(getCitationAvailabilityText(citation)).toContain('仅展示 QA 保存的引用快照')
  })
})
