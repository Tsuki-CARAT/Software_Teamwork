import { AlertCircle, CheckCircle2, FlaskConical, RefreshCw, Rocket } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { z } from 'zod'

import { ApiError } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  useCreateQAConfigVersionMutation,
  useCreateQALLMConfigVersionMutation,
  useCreateQALLMConnectionTestMutation,
  useQASettingsQueries,
} from '@/features/qa-settings/qa-settings.queries'
import type {
  CreateQAConfigVersionRequest,
  CreateQALLMConfigVersionRequest,
  QAConfigKnowledgeBase,
  QAConfigVersion,
  QALLMConfigVersion,
  QALLMConnectionTest,
} from '@/features/qa-settings/qa-settings.types'

interface QAFormState {
  topK: string
  scoreThreshold: string
  enableRerank: boolean
  rerankThreshold: string
  rerankTopN: string
  maxIterations: string
  toolTimeoutSeconds: string
  modelTimeoutSeconds: string
  overallTimeoutSeconds: string
  enabledToolNames: string
  defaultKnowledgeBaseIds: string
  knowledgeBases: string
  activate: boolean
}

interface LLMFormState {
  profileId: string
  modelName: string
  timeoutSeconds: string
  temperature: string
  maxTokens: string
  activate: boolean
}

interface FormErrors {
  qa?: string
  llm?: string
}

const knowledgeBaseSchema = z.object({
  id: z.string().min(1, '知识库 id 不能为空'),
  type: z.string().optional(),
  displayName: z.string().optional(),
  sortOrder: z.number().int().min(0).optional(),
})

const qaPayloadSchema = z.object({
  defaultKnowledgeBaseIds: z.array(z.string()).optional(),
  knowledgeBases: z.array(knowledgeBaseSchema).optional(),
  retrieval: z.object({
    topK: z.number().int().min(1).optional(),
    scoreThreshold: z.number().min(0).optional(),
    enableRerank: z.boolean(),
    rerankThreshold: z.number().min(0).optional(),
    rerankTopN: z.number().int().min(1).optional(),
  }),
  maxIterations: z.number().int().min(1).optional(),
  toolTimeoutSeconds: z.number().int().min(1).optional(),
  modelTimeoutSeconds: z.number().int().min(1).optional(),
  overallTimeoutSeconds: z.number().int().min(1).optional(),
  enabledToolNames: z.array(z.string()).optional(),
  agent: z
    .object({
      maxIterations: z.number().int().min(1),
      toolTimeoutSeconds: z.number().int().min(1),
      modelTimeoutSeconds: z.number().int().min(1),
      overallTimeoutSeconds: z.number().int().min(1),
      enabledToolNames: z.array(z.string()).optional(),
    })
    .optional(),
  activate: z.boolean(),
}) satisfies z.ZodType<CreateQAConfigVersionRequest>

const llmPayloadSchema = z.object({
  provider: z.literal('ai-gateway'),
  profileId: z.string().min(1, 'profileId 不能为空'),
  modelName: z.string().min(1, 'modelName 不能为空'),
  timeoutSeconds: z.number().int().min(1).optional(),
  temperature: z.number().optional(),
  maxTokens: z.number().int().min(1).optional(),
  activate: z.boolean(),
}) satisfies z.ZodType<CreateQALLMConfigVersionRequest>

function zodErrorMessage(error: z.ZodError): string {
  return error.issues.map((issue) => issue.message).join('；')
}

function optionalNumber(value: string, label: string, min?: number): number | undefined {
  const normalized = value.trim()
  if (normalized === '') return undefined

  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) {
    throw new Error(`${label} 必须是数字`)
  }

  if (min !== undefined && parsed < min) {
    throw new Error(`${label} 不能小于 ${min}`)
  }

  return parsed
}

function optionalInteger(value: string, label: string, min?: number): number | undefined {
  const parsed = optionalNumber(value, label, min)
  if (parsed !== undefined && !Number.isInteger(parsed)) {
    throw new Error(`${label} 必须是整数`)
  }

  return parsed
}

function splitLines(value: string): string[] | undefined {
  const items = value
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean)

  return items.length > 0 ? items : undefined
}

function parseKnowledgeBases(value: string): QAConfigKnowledgeBase[] | undefined {
  const normalized = value.trim()
  if (normalized === '') return undefined

  const parsed = JSON.parse(normalized) as unknown
  if (!Array.isArray(parsed)) {
    throw new Error('知识库配置必须是数组 JSON')
  }

  return parsed.map((item, index) => {
    if (!item || typeof item !== 'object') {
      throw new Error(`知识库配置第 ${index + 1} 项必须是对象`)
    }

    const candidate = item as Record<string, unknown>
    if (typeof candidate.id !== 'string' || candidate.id.trim() === '') {
      throw new Error(`知识库配置第 ${index + 1} 项缺少 id`)
    }

    return {
      id: candidate.id,
      type: typeof candidate.type === 'string' ? candidate.type : undefined,
      displayName: typeof candidate.displayName === 'string' ? candidate.displayName : undefined,
      sortOrder: typeof candidate.sortOrder === 'number' ? candidate.sortOrder : undefined,
    }
  })
}

function formatNumber(value: number | null | undefined): string {
  return value === null || value === undefined ? '' : String(value)
}

function createQAFormState(config?: QAConfigVersion): QAFormState {
  const agent = config?.agent
  const retrieval = config?.retrieval
  const enabledToolNames = config?.enabledToolNames ?? agent?.enabledToolNames ?? []

  return {
    topK: formatNumber(retrieval?.topK),
    scoreThreshold: formatNumber(retrieval?.scoreThreshold ?? retrieval?.similarityThreshold),
    enableRerank: retrieval?.enableRerank ?? retrieval?.useRerank ?? false,
    rerankThreshold: formatNumber(retrieval?.rerankThreshold),
    rerankTopN: formatNumber(retrieval?.rerankTopN),
    maxIterations: formatNumber(config?.maxIterations ?? agent?.maxIterations),
    toolTimeoutSeconds: formatNumber(config?.toolTimeoutSeconds ?? agent?.toolTimeoutSeconds),
    modelTimeoutSeconds: formatNumber(config?.modelTimeoutSeconds ?? agent?.modelTimeoutSeconds),
    overallTimeoutSeconds: formatNumber(
      config?.overallTimeoutSeconds ?? agent?.overallTimeoutSeconds,
    ),
    enabledToolNames: enabledToolNames.join('\n'),
    defaultKnowledgeBaseIds: (config?.defaultKnowledgeBaseIds ?? []).join('\n'),
    knowledgeBases: JSON.stringify(config?.knowledgeBases ?? [], null, 2),
    activate: true,
  }
}

function createLLMFormState(config?: QALLMConfigVersion): LLMFormState {
  return {
    profileId: config?.profileId ?? '',
    modelName: config?.modelName ?? '',
    timeoutSeconds: formatNumber(config?.timeoutSeconds),
    temperature: formatNumber(config?.temperature),
    maxTokens: formatNumber(config?.maxTokens),
    activate: true,
  }
}

function buildQAPayload(form: QAFormState): CreateQAConfigVersionRequest {
  const knowledgeBases = parseKnowledgeBases(form.knowledgeBases)
  const enabledToolNames = splitLines(form.enabledToolNames)
  const defaultKnowledgeBaseIds = splitLines(form.defaultKnowledgeBaseIds)
  const maxIterations = optionalInteger(form.maxIterations, '最大迭代次数', 1)
  const toolTimeoutSeconds = optionalInteger(form.toolTimeoutSeconds, '工具超时', 1)
  const modelTimeoutSeconds = optionalInteger(form.modelTimeoutSeconds, '模型超时', 1)
  const overallTimeoutSeconds = optionalInteger(form.overallTimeoutSeconds, '总超时', 1)

  const agent =
    maxIterations !== undefined &&
    toolTimeoutSeconds !== undefined &&
    modelTimeoutSeconds !== undefined &&
    overallTimeoutSeconds !== undefined
      ? {
          maxIterations,
          toolTimeoutSeconds,
          modelTimeoutSeconds,
          overallTimeoutSeconds,
          enabledToolNames,
        }
      : undefined

  const parsed = qaPayloadSchema.safeParse({
    defaultKnowledgeBaseIds,
    knowledgeBases,
    retrieval: {
      topK: optionalInteger(form.topK, 'Top K', 1),
      scoreThreshold: optionalNumber(form.scoreThreshold, '相似度阈值', 0),
      enableRerank: form.enableRerank,
      rerankThreshold: optionalNumber(form.rerankThreshold, 'Rerank 阈值', 0),
      rerankTopN: optionalInteger(form.rerankTopN, 'Rerank Top N', 1),
    },
    maxIterations,
    toolTimeoutSeconds,
    modelTimeoutSeconds,
    overallTimeoutSeconds,
    enabledToolNames,
    agent,
    activate: form.activate,
  })

  if (!parsed.success) {
    throw new Error(zodErrorMessage(parsed.error))
  }

  return parsed.data
}

function buildLLMPayload(form: LLMFormState): CreateQALLMConfigVersionRequest {
  const profileId = form.profileId.trim()
  const modelName = form.modelName.trim()

  const parsed = llmPayloadSchema.safeParse({
    provider: 'ai-gateway',
    profileId,
    modelName,
    timeoutSeconds: optionalInteger(form.timeoutSeconds, 'LLM 超时', 1),
    temperature: optionalNumber(form.temperature, 'Temperature'),
    maxTokens: optionalInteger(form.maxTokens, 'Max tokens', 1),
    activate: form.activate,
  })

  if (!parsed.success) {
    throw new Error(zodErrorMessage(parsed.error))
  }

  return parsed.data
}

function getErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    return error.requestId ? `${error.message}（requestId: ${error.requestId}）` : error.message
  }

  return error instanceof Error ? error.message : '未知错误'
}

function VersionMeta({
  id,
  versionNo,
  isActive,
  createdAt,
}: {
  id?: string
  versionNo?: number
  isActive?: boolean
  createdAt?: string
}) {
  return (
    <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
      <span className="rounded-md border border-border px-2 py-1">版本 {versionNo ?? '-'}</span>
      <span className="rounded-md border border-border px-2 py-1">ID {id ?? '-'}</span>
      <span className="rounded-md border border-border px-2 py-1">
        {isActive ? '当前生效' : '未生效'}
      </span>
      <span className="rounded-md border border-border px-2 py-1">
        {createdAt ? new Date(createdAt).toLocaleString() : '-'}
      </span>
    </div>
  )
}

function StatusMessage({ type, message }: { type: 'success' | 'error'; message: string }) {
  const Icon = type === 'success' ? CheckCircle2 : AlertCircle
  return (
    <div
      className={
        type === 'success'
          ? 'flex items-start gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-sm text-emerald-700'
          : 'flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-sm text-destructive'
      }
    >
      <Icon aria-hidden="true" className="mt-0.5 size-4 shrink-0" />
      <span>{message}</span>
    </div>
  )
}

export function QASettings() {
  const { qaConfigQuery, llmConfigQuery } = useQASettingsQueries()
  const createQAMutation = useCreateQAConfigVersionMutation()
  const createLLMMutation = useCreateQALLMConfigVersionMutation()
  const testLLMMutation = useCreateQALLMConnectionTestMutation()
  const [qaForm, setQAForm] = useState<QAFormState>(() => createQAFormState())
  const [llmForm, setLLMForm] = useState<LLMFormState>(() => createLLMFormState())
  const [errors, setErrors] = useState<FormErrors>({})
  const [qaResult, setQAResult] = useState<string | null>(null)
  const [llmResult, setLLMResult] = useState<string | null>(null)
  const [testResult, setTestResult] = useState<QALLMConnectionTest | null>(null)

  useEffect(() => {
    if (qaConfigQuery.data) {
      setQAForm(createQAFormState(qaConfigQuery.data))
    }
  }, [qaConfigQuery.data])

  useEffect(() => {
    if (llmConfigQuery.data) {
      setLLMForm(createLLMFormState(llmConfigQuery.data))
    }
  }, [llmConfigQuery.data])

  const isLoading = qaConfigQuery.isLoading || llmConfigQuery.isLoading
  const loadError = useMemo(() => {
    const error = qaConfigQuery.error ?? llmConfigQuery.error
    return error ? getErrorMessage(error) : null
  }, [llmConfigQuery.error, qaConfigQuery.error])

  const publishQAConfig = async () => {
    setErrors((current) => ({ ...current, qa: undefined }))
    setQAResult(null)

    try {
      const payload = buildQAPayload(qaForm)
      const created = await createQAMutation.mutateAsync(payload)
      setQAResult(`QA 配置版本 ${created.versionNo} 已发布`)
    } catch (error) {
      setErrors((current) => ({ ...current, qa: getErrorMessage(error) }))
    }
  }

  const publishLLMConfig = async () => {
    setErrors((current) => ({ ...current, llm: undefined }))
    setLLMResult(null)

    try {
      const payload = buildLLMPayload(llmForm)
      const created = await createLLMMutation.mutateAsync(payload)
      setLLMResult(`LLM 配置版本 ${created.versionNo} 已发布`)
    } catch (error) {
      setErrors((current) => ({ ...current, llm: getErrorMessage(error) }))
    }
  }

  const testLLMConnection = async () => {
    setErrors((current) => ({ ...current, llm: undefined }))
    setTestResult(null)

    try {
      const payload = buildLLMPayload(llmForm)
      const result = await testLLMMutation.mutateAsync({
        provider: payload.provider,
        profileId: payload.profileId,
        modelName: payload.modelName,
        timeoutSeconds: payload.timeoutSeconds,
      })
      setTestResult(result)
    } catch (error) {
      setErrors((current) => ({ ...current, llm: getErrorMessage(error) }))
    }
  }

  return (
    <div className="mx-auto max-w-6xl space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="text-2xl font-semibold text-foreground">QA / LLM 配置</h3>
          <p className="mt-2 text-sm text-muted-foreground">
            查看当前生效配置，调整参数后发布新版本。LLM 仅引用 AI Gateway profile。
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          onClick={() => {
            void qaConfigQuery.refetch()
            void llmConfigQuery.refetch()
          }}
          disabled={qaConfigQuery.isFetching || llmConfigQuery.isFetching}
        >
          <RefreshCw aria-hidden="true" className="size-4" />
          刷新
        </Button>
      </div>

      {loadError && <StatusMessage type="error" message={`加载配置失败：${loadError}`} />}

      {isLoading ? (
        <div className="grid gap-4 lg:grid-cols-2">
          {Array.from({ length: 2 }).map((_, index) => (
            <div
              key={index}
              className="h-96 animate-pulse rounded-lg border border-border bg-card"
            />
          ))}
        </div>
      ) : (
        <div className="grid gap-4 xl:grid-cols-[1.15fr_0.85fr]">
          <section className="space-y-4 rounded-lg border border-border bg-card p-5">
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h4 className="text-lg font-semibold text-foreground">QA 运行配置</h4>
                <VersionMeta
                  id={qaConfigQuery.data?.id}
                  versionNo={qaConfigQuery.data?.versionNo}
                  isActive={qaConfigQuery.data?.isActive}
                  createdAt={qaConfigQuery.data?.createdAt}
                />
              </div>
              <p className="text-sm text-muted-foreground">
                检索、Agent 迭代与工具白名单配置会通过版本化资源保存。
              </p>
            </div>

            {errors.qa && <StatusMessage type="error" message={errors.qa} />}
            {qaResult && <StatusMessage type="success" message={qaResult} />}

            <div className="grid gap-3 md:grid-cols-2">
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">Top K</span>
                <Input
                  value={qaForm.topK}
                  onChange={(event) => setQAForm({ ...qaForm, topK: event.target.value })}
                  inputMode="numeric"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">相似度阈值</span>
                <Input
                  value={qaForm.scoreThreshold}
                  onChange={(event) => setQAForm({ ...qaForm, scoreThreshold: event.target.value })}
                  inputMode="decimal"
                />
              </label>
              <label className="flex items-center gap-2 rounded-lg border border-border p-3 text-sm">
                <input
                  type="checkbox"
                  className="size-4"
                  checked={qaForm.enableRerank}
                  onChange={(event) => setQAForm({ ...qaForm, enableRerank: event.target.checked })}
                />
                <span className="font-medium text-foreground">启用 rerank</span>
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">Rerank 阈值</span>
                <Input
                  value={qaForm.rerankThreshold}
                  onChange={(event) =>
                    setQAForm({ ...qaForm, rerankThreshold: event.target.value })
                  }
                  inputMode="decimal"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">Rerank Top N</span>
                <Input
                  value={qaForm.rerankTopN}
                  onChange={(event) => setQAForm({ ...qaForm, rerankTopN: event.target.value })}
                  inputMode="numeric"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">最大迭代次数</span>
                <Input
                  value={qaForm.maxIterations}
                  onChange={(event) => setQAForm({ ...qaForm, maxIterations: event.target.value })}
                  inputMode="numeric"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">工具超时（秒）</span>
                <Input
                  value={qaForm.toolTimeoutSeconds}
                  onChange={(event) =>
                    setQAForm({ ...qaForm, toolTimeoutSeconds: event.target.value })
                  }
                  inputMode="numeric"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">模型超时（秒）</span>
                <Input
                  value={qaForm.modelTimeoutSeconds}
                  onChange={(event) =>
                    setQAForm({ ...qaForm, modelTimeoutSeconds: event.target.value })
                  }
                  inputMode="numeric"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">总超时（秒）</span>
                <Input
                  value={qaForm.overallTimeoutSeconds}
                  onChange={(event) =>
                    setQAForm({ ...qaForm, overallTimeoutSeconds: event.target.value })
                  }
                  inputMode="numeric"
                />
              </label>
              <label className="flex items-center gap-2 rounded-lg border border-border p-3 text-sm">
                <input
                  type="checkbox"
                  className="size-4"
                  checked={qaForm.activate}
                  onChange={(event) => setQAForm({ ...qaForm, activate: event.target.checked })}
                />
                <span className="font-medium text-foreground">发布后设为当前版本</span>
              </label>
            </div>

            <div className="grid gap-3 lg:grid-cols-2">
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">工具白名单</span>
                <Textarea
                  value={qaForm.enabledToolNames}
                  onChange={(event) =>
                    setQAForm({ ...qaForm, enabledToolNames: event.target.value })
                  }
                  className="min-h-24 font-mono text-xs"
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">默认知识库 ID</span>
                <Textarea
                  value={qaForm.defaultKnowledgeBaseIds}
                  onChange={(event) =>
                    setQAForm({ ...qaForm, defaultKnowledgeBaseIds: event.target.value })
                  }
                  className="min-h-24 font-mono text-xs"
                />
              </label>
            </div>

            <label className="block space-y-1.5 text-sm">
              <span className="font-medium text-foreground">知识库配置 JSON</span>
              <Textarea
                value={qaForm.knowledgeBases}
                onChange={(event) => setQAForm({ ...qaForm, knowledgeBases: event.target.value })}
                className="min-h-36 font-mono text-xs"
              />
            </label>

            <div className="flex justify-end">
              <Button
                type="button"
                onClick={() => void publishQAConfig()}
                disabled={createQAMutation.isPending}
              >
                <Rocket aria-hidden="true" className="size-4" />
                发布 QA 新版本
              </Button>
            </div>
          </section>

          <section className="space-y-4 rounded-lg border border-border bg-card p-5">
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <h4 className="text-lg font-semibold text-foreground">LLM 配置</h4>
                <VersionMeta
                  id={llmConfigQuery.data?.id}
                  versionNo={llmConfigQuery.data?.versionNo}
                  isActive={llmConfigQuery.data?.isActive}
                  createdAt={llmConfigQuery.data?.createdAt}
                />
              </div>
              <p className="text-sm text-muted-foreground">
                页面只展示和提交 profileId、modelName 与生成参数，不包含 provider API key。
              </p>
            </div>

            {errors.llm && <StatusMessage type="error" message={errors.llm} />}
            {llmResult && <StatusMessage type="success" message={llmResult} />}
            {testResult && (
              <StatusMessage
                type={testResult.success ? 'success' : 'error'}
                message={
                  testResult.success
                    ? `连接成功，延迟 ${testResult.latencyMs ?? 0}ms`
                    : `连接失败：${testResult.errorMessage ?? testResult.errorCode ?? '未返回原因'}`
                }
              />
            )}

            <div className="rounded-lg border border-border bg-background p-3 text-sm">
              <div className="mb-2 font-medium text-foreground">当前引用</div>
              <dl className="grid gap-2">
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">provider</dt>
                  <dd className="font-mono text-foreground">ai-gateway</dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">profileId</dt>
                  <dd className="break-all font-mono text-foreground">
                    {llmConfigQuery.data?.profileId ?? '-'}
                  </dd>
                </div>
                <div className="flex justify-between gap-3">
                  <dt className="text-muted-foreground">modelName</dt>
                  <dd className="break-all font-mono text-foreground">
                    {llmConfigQuery.data?.modelName ?? '-'}
                  </dd>
                </div>
              </dl>
            </div>

            <div className="grid gap-3">
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">profileId</span>
                <Input
                  value={llmForm.profileId}
                  onChange={(event) => setLLMForm({ ...llmForm, profileId: event.target.value })}
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">modelName</span>
                <Input
                  value={llmForm.modelName}
                  onChange={(event) => setLLMForm({ ...llmForm, modelName: event.target.value })}
                />
              </label>
              <label className="space-y-1.5 text-sm">
                <span className="font-medium text-foreground">超时（秒）</span>
                <Input
                  value={llmForm.timeoutSeconds}
                  onChange={(event) =>
                    setLLMForm({ ...llmForm, timeoutSeconds: event.target.value })
                  }
                  inputMode="numeric"
                />
              </label>
              <div className="grid gap-3 sm:grid-cols-2">
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium text-foreground">Temperature</span>
                  <Input
                    value={llmForm.temperature}
                    onChange={(event) =>
                      setLLMForm({ ...llmForm, temperature: event.target.value })
                    }
                    inputMode="decimal"
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium text-foreground">Max tokens</span>
                  <Input
                    value={llmForm.maxTokens}
                    onChange={(event) => setLLMForm({ ...llmForm, maxTokens: event.target.value })}
                    inputMode="numeric"
                  />
                </label>
              </div>
              <label className="flex items-center gap-2 rounded-lg border border-border p-3 text-sm">
                <input
                  type="checkbox"
                  className="size-4"
                  checked={llmForm.activate}
                  onChange={(event) => setLLMForm({ ...llmForm, activate: event.target.checked })}
                />
                <span className="font-medium text-foreground">发布后设为当前版本</span>
              </label>
            </div>

            <div className="flex flex-wrap justify-end gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={() => void testLLMConnection()}
                disabled={testLLMMutation.isPending}
              >
                <FlaskConical aria-hidden="true" className="size-4" />
                测试连接
              </Button>
              <Button
                type="button"
                onClick={() => void publishLLMConfig()}
                disabled={createLLMMutation.isPending}
              >
                <Rocket aria-hidden="true" className="size-4" />
                发布 LLM 新版本
              </Button>
            </div>
          </section>
        </div>
      )}
    </div>
  )
}
