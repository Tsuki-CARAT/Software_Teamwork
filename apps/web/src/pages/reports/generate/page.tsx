import { Ban, Download, FileText, Loader2, PencilLine, Play, RefreshCw, Save } from 'lucide-react'
import { type FormEvent, useEffect, useMemo, useState } from 'react'

import { InlineNotice, ProgressSummary, StateBlock } from '@/components/common'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type {
  CreateReportFormValues,
  Report,
  ReportFile,
  ReportJob,
  ReportJobStatus,
  ReportSectionVersion,
} from '@/features/reports'
import {
  createReportSchema,
  defaultCreateReportValues,
  formatReportGatewayError,
  useCancelReportJob,
  useCreateReportFileMutation,
  useCreateReportJobMutation,
  useCreateReportMutation,
  useDownloadReportFileMutation,
  useReportBootstrapQueries,
  useReportDetailQueries,
  useReportEvents,
  useReportJobQuery,
  useRetryReportJobMutation,
  useSectionVersions,
  useUpdateReportOutlineMutation,
  useUpdateReportSectionMutation,
} from '@/features/reports'
import { cn } from '@/lib/utils'

const steps = [
  { key: 'draft', label: '1. 草稿与大纲' },
  { key: 'outline', label: '2. 编辑大纲' },
  { key: 'content', label: '3. 正文生成' },
  { key: 'export', label: '4. DOCX 导出' },
] as const

type StepKey = (typeof steps)[number]['key']

const statusText: Record<ReportJobStatus, string> = {
  pending: '等待中',
  running: '生成中',
  succeeded: '已完成',
  partial_succeeded: '部分成功',
  failed: '失败',
  canceled: '已取消',
}

function getProgressPercent(job?: ReportJob | null): number {
  const value = job?.progress?.percent
  return typeof value === 'number' ? Math.max(0, Math.min(100, value)) : 0
}

function formatDate(value?: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export function ReportGeneratePage() {
  const [step, setStep] = useState<StepKey>('draft')
  const [form, setForm] = useState<CreateReportFormValues>({
    ...defaultCreateReportValues,
    reportType: '',
    templateId: '',
  })
  const [selectedMaterialIds, setSelectedMaterialIds] = useState<string[]>([])
  const [currentReport, setCurrentReport] = useState<Report | null>(null)
  const [activeJobId, setActiveJobId] = useState<string | null>(null)
  const [lastJob, setLastJob] = useState<ReportJob | null>(null)
  const [latestFile, setLatestFile] = useState<ReportFile | null>(null)
  const [activeSectionId, setActiveSectionId] = useState('')
  const [sectionDraft, setSectionDraft] = useState('')
  const [showVersions, setShowVersions] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  const [formError, setFormError] = useState<string | null>(null)

  const { typeQuery, templateQuery, materialQuery } = useReportBootstrapQueries(form.reportType)
  const { outlinesQuery, sectionsQuery } = useReportDetailQueries(currentReport?.id ?? null)
  const jobQuery = useReportJobQuery(activeJobId)
  const createReportMutation = useCreateReportMutation()
  const createJobMutation = useCreateReportJobMutation()
  const saveOutlineMutation = useUpdateReportOutlineMutation(currentReport?.id ?? '')
  const saveSectionMutation = useUpdateReportSectionMutation(currentReport?.id ?? '')
  const createFileMutation = useCreateReportFileMutation()
  const retryJobMutation = useRetryReportJobMutation()
  const downloadMutation = useDownloadReportFileMutation()
  const cancelJobMutation = useCancelReportJob()
  const eventsQuery = useReportEvents(currentReport?.id ?? null)
  const sectionVersionsQuery = useSectionVersions(
    currentReport?.id ?? null,
    showVersions ? activeSectionId : null,
  )

  const reportTypes = useMemo(() => typeQuery.data ?? [], [typeQuery.data])
  const templates = useMemo(() => templateQuery.data?.items ?? [], [templateQuery.data])
  const materials = useMemo(() => materialQuery.data?.items ?? [], [materialQuery.data])
  const outline = outlinesQuery.data?.[0]?.sections ?? []
  const sections = useMemo(() => sectionsQuery.data ?? [], [sectionsQuery.data])
  const activeSection = sections.find((item) => item.id === activeSectionId) ?? sections[0]
  const effectiveJob = jobQuery.data ?? lastJob
  const selectedTemplate = templates.find((template) => template.id === form.templateId)
  const hasDraftPendingOutlineJob = Boolean(currentReport && step === 'draft')

  const bootstrapErrors = useMemo(
    () =>
      [
        { error: typeQuery.error, label: '报告类型', visible: typeQuery.isError },
        { error: templateQuery.error, label: '报告模板', visible: templateQuery.isError },
        { error: materialQuery.error, label: '报告素材', visible: materialQuery.isError },
      ].filter((item) => item.visible),
    [
      materialQuery.error,
      materialQuery.isError,
      templateQuery.error,
      templateQuery.isError,
      typeQuery.error,
      typeQuery.isError,
    ],
  )
  const isBootstrapLoading =
    typeQuery.isLoading || templateQuery.isLoading || materialQuery.isLoading
  const canCreateReport =
    !isBootstrapLoading &&
    !typeQuery.isError &&
    !templateQuery.isError &&
    reportTypes.length > 0 &&
    templates.length > 0

  useEffect(() => {
    if (reportTypes.length === 0) return
    if (reportTypes.some((type) => type.code === form.reportType)) return
    setForm((prev) => ({ ...prev, reportType: reportTypes[0]?.code ?? '' }))
  }, [form.reportType, reportTypes])

  useEffect(() => {
    const firstTemplate = templates[0]
    const hasSelectedTemplate = templates.some((template) => template.id === form.templateId)
    if (firstTemplate && !hasSelectedTemplate) {
      setForm((prev) => ({ ...prev, templateId: firstTemplate.id }))
    } else if (!firstTemplate && form.templateId) {
      setForm((prev) => ({ ...prev, templateId: '' }))
    }
  }, [form.templateId, templates])

  useEffect(() => {
    if (sections.length === 0) {
      if (activeSectionId) setActiveSectionId('')
      return
    }
    if (!sections.some((section) => section.id === activeSectionId)) {
      setActiveSectionId(sections[0]?.id ?? '')
    }
  }, [activeSectionId, sections])

  useEffect(() => {
    if (activeSection) {
      setSectionDraft(activeSection.content ?? '')
    } else {
      setSectionDraft('')
    }
  }, [activeSection])

  useEffect(() => {
    if (jobQuery.data) {
      setLastJob(jobQuery.data)
    }
  }, [jobQuery.data])

  const updateForm = (field: keyof CreateReportFormValues, value: string | number) => {
    setForm((prev) => ({ ...prev, [field]: value }))
  }

  const toggleMaterial = (id: string) => {
    setSelectedMaterialIds((prev) =>
      prev.includes(id) ? prev.filter((item) => item !== id) : [...prev, id],
    )
  }

  const handleCreateReport = async (event: FormEvent) => {
    event.preventDefault()
    setFormError(null)
    setNotice(null)

    const parsed = createReportSchema.safeParse(form)
    if (!parsed.success) {
      setFormError(parsed.error.issues[0]?.message ?? '请检查报告参数')
      return
    }

    const payload = {
      name: parsed.data.name,
      reportType: parsed.data.reportType,
      templateId: parsed.data.templateId,
      topic: parsed.data.topic,
      specialty: parsed.data.specialty,
      businessObject: parsed.data.businessObject,
      year: parsed.data.year,
      extraContext: parsed.data.extraContextText
        ? { note: parsed.data.extraContextText }
        : undefined,
      source: 'frontend' as const,
    }

    let report = currentReport
    if (!report) {
      try {
        report = await createReportMutation.mutateAsync(payload)
        setCurrentReport(report)
      } catch (error) {
        setActiveJobId(null)
        setNotice(formatReportGatewayError(error, '创建报告草稿失败'))
        return
      }
    }

    try {
      const job = await createJobMutation.mutateAsync({
        reportId: report.id,
        payload: {
          jobType: 'outline_generation',
          target: { scope: 'report' },
          materialIds: selectedMaterialIds,
          requirements: parsed.data.extraContextText,
        },
      })
      setLastJob(job)
      setActiveJobId(job.id)
      setStep('outline')
      setNotice(
        '已创建报告草稿，并通过 /api/v1/reports/{reportId}/jobs 创建大纲任务；页面只展示服务端返回的大纲与事件。',
      )
    } catch (error) {
      setActiveJobId(null)
      setNotice(
        `${formatReportGatewayError(
          error,
          '创建大纲任务失败',
        )}；已保留服务端报告草稿"${report.name}"，再次提交将复用该草稿创建大纲任务。`,
      )
    }
  }

  const handleSaveOutline = async () => {
    if (!currentReport || !outlinesQuery.data?.[0]) {
      setNotice('暂无可保存的服务端大纲。请先创建报告并等待大纲接口返回数据。')
      return
    }

    try {
      await saveOutlineMutation.mutateAsync({
        outlineId: outlinesQuery.data[0].id,
        sections: outline,
      })
      setNotice('大纲已保存，后端将负责重新编号和结构合法性校验。')
    } catch (error) {
      setNotice(formatReportGatewayError(error, '大纲保存失败'))
    }
  }

  const handleGenerateContent = async () => {
    if (!currentReport) {
      setNotice('请先创建报告草稿。')
      return
    }
    if (outline.length === 0) {
      setNotice('暂无服务端大纲数据，不能创建正文生成任务。')
      return
    }

    try {
      const job = await createJobMutation.mutateAsync({
        reportId: currentReport.id,
        payload: {
          jobType: 'content_generation',
          target: { scope: 'report' },
          materialIds: selectedMaterialIds,
          options: { preserveManualEdits: true, saveResult: true },
        },
      })
      setLastJob(job)
      setActiveJobId(job.id)
      setStep('content')
      setNotice('已创建正文生成任务；真实 AI 生成能力以后端任务和章节接口返回为准。')
    } catch (error) {
      setNotice(formatReportGatewayError(error, '正文生成任务创建失败'))
    }
  }

  const handleSaveSection = async () => {
    if (!currentReport || !activeSection) {
      setNotice('暂无可保存的服务端章节。')
      return
    }

    try {
      await saveSectionMutation.mutateAsync({
        sectionId: activeSection.id,
        title: activeSection.title,
        content: sectionDraft,
      })
      setNotice('章节正文已保存。')
    } catch (error) {
      setNotice(formatReportGatewayError(error, '章节保存失败'))
    }
  }

  const handleRetry = async () => {
    const retryJob = effectiveJob ?? lastJob
    if (retryJob?.id) {
      try {
        const attempt = await retryJobMutation.mutateAsync({
          jobId: retryJob.id,
          reportId: retryJob.reportId,
        })
        setNotice(`已创建重试尝试：${attempt.id}，当前状态：${attempt.status}`)
      } catch (error) {
        setNotice(formatReportGatewayError(error, '创建重试尝试失败'))
      }
      return
    }
    setNotice('暂无可重试的服务端任务。')
  }

  const handleCancel = async () => {
    if (!effectiveJob?.id) {
      setNotice('暂无可取消的服务端任务。')
      return
    }
    if (effectiveJob.status !== 'pending' && effectiveJob.status !== 'running') {
      setNotice('只有等待中或运行中的任务才能取消。')
      return
    }
    try {
      await cancelJobMutation.mutateAsync(effectiveJob.id)
      setNotice('已请求取消任务。')
    } catch (error) {
      setNotice(formatReportGatewayError(error, '任务取消暂不支持（Gateway 契约待补齐）'))
    }
  }

  const handleExport = async () => {
    if (!currentReport) {
      setNotice('请先创建报告草稿。')
      return
    }

    try {
      const file = await createFileMutation.mutateAsync({
        reportId: currentReport.id,
        format: 'docx',
        templateId: selectedTemplate?.id,
        styleOptions: { numberingMode: 'global' },
      })
      setLatestFile(file)
      setStep('export')
      setNotice('已创建报告文件资源；富 DOCX 工具链是否可用以后端返回状态为准。')
    } catch (error) {
      setNotice(formatReportGatewayError(error, '创建 DOCX 文件资源失败'))
    }
  }

  const handleDownload = async () => {
    if (!latestFile) {
      setNotice('暂无可下载的服务端文件资源。')
      return
    }

    try {
      const blob = await downloadMutation.mutateAsync(latestFile.id)
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = latestFile.filename ?? `${form.name}.docx`
      anchor.click()
      URL.revokeObjectURL(url)
    } catch (error) {
      setNotice(formatReportGatewayError(error, '下载报告文件失败'))
    }
  }

  const progressPercent = getProgressPercent(effectiveJob)

  return (
    <div className="flex h-full flex-col overflow-auto bg-background">
      <div className="border-b border-border bg-muted/30 px-6 py-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-foreground">报告生成</h1>
            <p className="mt-1 text-sm text-muted-foreground">
              按最新 gateway RESTful 契约整合：草稿、大纲、正文任务和 DOCX 文件资源。
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            {steps.map((item) => (
              <Button
                key={item.key}
                type="button"
                variant={step === item.key ? 'default' : 'outline'}
                size="sm"
                onClick={() => setStep(item.key)}
              >
                {item.label}
              </Button>
            ))}
          </div>
        </div>

        <InlineNotice className="mt-4" title="能力边界" variant="warning">
          真实 AI 大纲/正文生成、Document MCP tools 和富 DOCX 工具链尚未就绪；页面只展示 Gateway
          返回的数据和错误，不填充本地示例。
        </InlineNotice>
        {bootstrapErrors.map((item) => (
          <InlineNotice
            className="mt-3"
            key={item.label}
            title={`${item.label}加载失败`}
            variant="error"
          >
            {formatReportGatewayError(item.error, `${item.label}加载失败`)}
          </InlineNotice>
        ))}
        {(notice || formError) && (
          <InlineNotice
            className="mt-4"
            title={formError ? '表单校验失败' : undefined}
            variant={formError ? 'error' : 'info'}
          >
            {formError ?? notice}
          </InlineNotice>
        )}
      </div>

      <div className="grid flex-1 gap-6 p-6 xl:grid-cols-[minmax(0,1.1fr)_360px]">
        <div className="min-w-0 space-y-6">
          {step === 'draft' && (
            <form
              className="rounded-lg border border-border bg-card p-5"
              onSubmit={handleCreateReport}
            >
              <div className="mb-5 flex items-center gap-2">
                <FileText className="size-4 text-muted-foreground" />
                <h2 className="text-base font-semibold">创建草稿并生成大纲</h2>
              </div>

              {hasDraftPendingOutlineJob && (
                <InlineNotice className="mb-4" title="已保留报告草稿" variant="warning">
                  当前服务端报告草稿为"{currentReport?.name}"，再次提交只会复用该草稿创建大纲任务，
                  不会重复创建报告记录。
                </InlineNotice>
              )}

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">报告名称</span>
                  <Input
                    value={form.name}
                    onChange={(event) => updateForm('name', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">报告类型</span>
                  <select
                    className="h-8 w-full rounded-lg border border-input bg-background px-2.5 text-sm"
                    disabled={typeQuery.isLoading || typeQuery.isError || reportTypes.length === 0}
                    value={form.reportType}
                    onChange={(event) => {
                      updateForm('reportType', event.target.value)
                      updateForm('templateId', '')
                    }}
                  >
                    <option value="">请选择报告类型</option>
                    {reportTypes.map((type) => (
                      <option key={type.code} value={type.code}>
                        {type.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">报告模板</span>
                  <select
                    className="h-8 w-full rounded-lg border border-input bg-background px-2.5 text-sm"
                    disabled={
                      templateQuery.isLoading || templateQuery.isError || templates.length === 0
                    }
                    value={form.templateId}
                    onChange={(event) => updateForm('templateId', event.target.value)}
                  >
                    <option value="">请选择报告模板</option>
                    {templates.map((template) => (
                      <option key={template.id} value={template.id}>
                        {template.templateName} v{template.version}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">年份</span>
                  <Input
                    type="number"
                    value={form.year}
                    onChange={(event) => updateForm('year', Number(event.target.value))}
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">专业</span>
                  <Input
                    value={form.specialty ?? ''}
                    onChange={(event) => updateForm('specialty', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm">
                  <span className="font-medium">业务对象</span>
                  <Input
                    value={form.businessObject ?? ''}
                    onChange={(event) => updateForm('businessObject', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm md:col-span-2">
                  <span className="font-medium">报告主题</span>
                  <Input
                    value={form.topic}
                    onChange={(event) => updateForm('topic', event.target.value)}
                  />
                </label>
                <label className="space-y-1.5 text-sm md:col-span-2">
                  <span className="font-medium">补充上下文 / 生成要求</span>
                  <textarea
                    className="min-h-24 w-full rounded-lg border border-input bg-background px-3 py-2 text-sm outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                    value={form.extraContextText ?? ''}
                    onChange={(event) => updateForm('extraContextText', event.target.value)}
                  />
                </label>
              </div>

              <div className="mt-5">
                <p className="mb-2 text-sm font-medium">引用素材</p>
                {materialQuery.isLoading ? (
                  <StateBlock size="compact" title="素材加载中" variant="loading" />
                ) : materialQuery.isError ? (
                  <StateBlock
                    description={formatReportGatewayError(materialQuery.error, '素材列表加载失败')}
                    size="compact"
                    title="素材列表加载失败"
                    variant="error"
                  />
                ) : materials.length === 0 ? (
                  <StateBlock size="compact" title="暂无可引用素材" variant="empty" />
                ) : (
                  <div className="flex flex-wrap gap-2">
                    {materials.map((material) => (
                      <button
                        key={material.id}
                        type="button"
                        className={cn(
                          'rounded-lg border px-3 py-2 text-sm transition-colors',
                          selectedMaterialIds.includes(material.id)
                            ? 'border-primary bg-primary text-primary-foreground'
                            : 'border-border bg-background text-muted-foreground hover:text-foreground',
                        )}
                        onClick={() => toggleMaterial(material.id)}
                      >
                        {material.materialName}
                      </button>
                    ))}
                  </div>
                )}
              </div>

              <div className="mt-5 flex justify-end">
                <Button
                  type="submit"
                  disabled={
                    !canCreateReport ||
                    createReportMutation.isPending ||
                    createJobMutation.isPending
                  }
                >
                  {(createReportMutation.isPending || createJobMutation.isPending) && (
                    <Loader2 className="size-4 animate-spin" />
                  )}
                  {hasDraftPendingOutlineJob ? '复用草稿生成大纲' : '创建草稿并生成大纲'}
                </Button>
              </div>
            </form>
          )}

          {step === 'outline' && (
            <section className="rounded-lg border border-border bg-card p-5">
              <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold">大纲章节</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    保存整棵章节树，后端负责合法性校验和重新编号。
                  </p>
                </div>
                <div className="flex gap-2">
                  <Button
                    disabled={
                      !currentReport || outline.length === 0 || saveOutlineMutation.isPending
                    }
                    variant="outline"
                    onClick={handleSaveOutline}
                  >
                    <Save className="size-4" />
                    保存大纲
                  </Button>
                  <Button
                    disabled={!currentReport || outline.length === 0 || createJobMutation.isPending}
                    onClick={handleGenerateContent}
                  >
                    <Play className="size-4" />
                    生成正文
                  </Button>
                </div>
              </div>
              {outlinesQuery.isLoading ? (
                <StateBlock size="compact" title="大纲加载中" variant="loading" />
              ) : outlinesQuery.isError ? (
                <StateBlock
                  description={formatReportGatewayError(outlinesQuery.error, '大纲加载失败')}
                  size="compact"
                  title="大纲加载失败"
                  variant="error"
                />
              ) : outline.length === 0 ? (
                <StateBlock
                  description="大纲生成能力或数据尚未就绪时，页面不会填充本地示例。"
                  size="compact"
                  title="暂无服务端大纲"
                  variant="empty"
                />
              ) : (
                <div className="space-y-2">
                  {outline.map((node) => (
                    <div
                      key={node.id ?? node.clientSectionId ?? node.title}
                      className={cn(
                        'flex items-center gap-3 rounded-lg border border-border bg-background px-3 py-2',
                        node.level > 1 && 'ml-8',
                      )}
                    >
                      <span className="w-10 text-xs text-muted-foreground">
                        {node.numbering ?? '-'}
                      </span>
                      <span className="min-w-0 flex-1 truncate text-sm font-medium">
                        {node.title}
                      </span>
                      <span className="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground">
                        level {node.level}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </section>
          )}

          {step === 'content' && (
            <section className="grid gap-4 rounded-lg border border-border bg-card p-5 lg:grid-cols-[280px_minmax(0,1fr)]">
              {sectionsQuery.isLoading ? (
                <StateBlock
                  className="lg:col-span-2"
                  size="compact"
                  title="章节加载中"
                  variant="loading"
                />
              ) : sectionsQuery.isError ? (
                <StateBlock
                  className="lg:col-span-2"
                  description={formatReportGatewayError(sectionsQuery.error, '章节加载失败')}
                  size="compact"
                  title="章节加载失败"
                  variant="error"
                />
              ) : sections.length === 0 ? (
                <StateBlock
                  className="lg:col-span-2"
                  description="正文生成能力或章节数据尚未就绪时，页面不会填充本地正文。"
                  size="compact"
                  title="暂无服务端章节"
                  variant="empty"
                />
              ) : (
                <>
                  <div>
                    <h2 className="mb-3 text-base font-semibold">章节列表</h2>
                    <div className="space-y-2">
                      {sections.map((section) => (
                        <button
                          key={section.id}
                          type="button"
                          className={cn(
                            'flex w-full items-center justify-between rounded-lg border px-3 py-2 text-left text-sm',
                            activeSection?.id === section.id
                              ? 'border-primary bg-primary/10 text-primary'
                              : 'border-border bg-background text-muted-foreground hover:text-foreground',
                          )}
                          onClick={() => setActiveSectionId(section.id)}
                        >
                          <span className="min-w-0 truncate">
                            {section.numbering} {section.title}
                          </span>
                          <span>{statusText[section.generationStatus]}</span>
                        </button>
                      ))}
                    </div>
                  </div>

                  <div className="min-w-0">
                    <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                      <div>
                        <h3 className="text-base font-semibold">
                          {activeSection?.title ?? '章节正文'}
                        </h3>
                        <p className="text-sm text-muted-foreground">
                          保存章节只提交结构化正文，不直接生成 DOCX。
                        </p>
                      </div>
                      <div className="flex gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShowVersions((prev) => !prev)}
                        >
                          版本记录{showVersions ? ' ▲' : ' ▼'}
                        </Button>
                        <Button
                          variant="outline"
                          onClick={handleRetry}
                          disabled={
                            effectiveJob?.status !== 'failed' &&
                            effectiveJob?.status !== 'partial_succeeded' &&
                            effectiveJob?.status !== 'canceled'
                          }
                        >
                          <RefreshCw className="size-4" />
                          重试任务
                        </Button>
                        <Button variant="outline" onClick={handleSaveSection}>
                          <PencilLine className="size-4" />
                          保存章节
                        </Button>
                        <Button onClick={handleExport}>
                          <Download className="size-4" />
                          创建 DOCX
                        </Button>
                      </div>
                    </div>

                    {showVersions && (
                      <div className="mb-4 rounded-lg border border-border bg-muted/30 p-3">
                        <h4 className="mb-2 text-sm font-medium">历史版本</h4>
                        {sectionVersionsQuery.isLoading ? (
                          <p className="text-xs text-muted-foreground">加载中...</p>
                        ) : sectionVersionsQuery.isError ? (
                          <p className="text-xs text-muted-foreground">
                            {formatReportGatewayError(
                              sectionVersionsQuery.error,
                              '章节版本加载失败',
                            )}
                          </p>
                        ) : sectionVersionsQuery.data && sectionVersionsQuery.data.length > 0 ? (
                          <div className="max-h-40 space-y-2 overflow-auto">
                            {(sectionVersionsQuery.data as ReportSectionVersion[]).map(
                              (version) => (
                                <div
                                  key={version.id}
                                  className="flex items-center justify-between rounded-lg border border-border bg-background px-3 py-2 text-xs"
                                >
                                  <div className="flex items-center gap-3">
                                    <span className="font-medium">v{version.version}</span>
                                    <span className="rounded-full bg-muted px-2 py-0.5 text-muted-foreground">
                                      {version.source === 'manual' ? '手动' : 'AI'}
                                    </span>
                                    <span className="text-muted-foreground">
                                      {formatDate(version.createdAt)}
                                    </span>
                                  </div>
                                  {version.content && (
                                    <button
                                      type="button"
                                      className="text-primary hover:underline"
                                      onClick={() => {
                                        setSectionDraft(version.content ?? '')
                                        setNotice(`已恢复版本 v${version.version} 的内容到编辑区。`)
                                      }}
                                    >
                                      恢复
                                    </button>
                                  )}
                                </div>
                              ),
                            )}
                          </div>
                        ) : (
                          <p className="text-xs text-muted-foreground">暂无历史版本。</p>
                        )}
                      </div>
                    )}

                    <textarea
                      className="min-h-[360px] w-full rounded-lg border border-input bg-background px-4 py-3 text-sm leading-7 outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
                      value={sectionDraft}
                      onChange={(event) => setSectionDraft(event.target.value)}
                    />
                  </div>
                </>
              )}
            </section>
          )}

          {step === 'export' && (
            <section className="rounded-lg border border-border bg-card p-5">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <h2 className="text-base font-semibold">DOCX 文件资源</h2>
                  <p className="mt-1 text-sm text-muted-foreground">
                    导出通过 POST /api/v1/report-files 创建资源；下载读取文件内容接口。
                  </p>
                </div>
                <Button onClick={handleDownload} disabled={!latestFile}>
                  <Download className="size-4" />
                  下载文件
                </Button>
              </div>

              <div className="mt-4 rounded-lg border border-border bg-background p-4">
                {latestFile ? (
                  <div className="grid gap-2 text-sm md:grid-cols-2">
                    <span className="text-muted-foreground">文件 ID</span>
                    <code>{latestFile.id}</code>
                    <span className="text-muted-foreground">文件名</span>
                    <span>{latestFile.filename ?? `${form.name}.docx`}</span>
                    <span className="text-muted-foreground">状态</span>
                    <span>{statusText[latestFile.status]}</span>
                    <span className="text-muted-foreground">创建时间</span>
                    <span>{formatDate(latestFile.createdAt)}</span>
                  </div>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    尚未创建导出文件。请先生成正文后创建 DOCX 文件资源。
                  </p>
                )}
              </div>
            </section>
          )}
        </div>

        <aside className="flex flex-col space-y-4">
          <section className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-semibold">当前报告</h2>
            <div className="mt-3 space-y-2 text-sm">
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">reportId</span>
                <code className="truncate">{currentReport?.id ?? '-'}</code>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">模板</span>
                <span className="truncate">{selectedTemplate?.templateName ?? '-'}</span>
              </div>
              <div className="flex justify-between gap-4">
                <span className="text-muted-foreground">状态</span>
                <span>{currentReport?.status ?? '未创建'}</span>
              </div>
            </div>
          </section>

          <section className="rounded-lg border border-border bg-card p-4">
            <h2 className="text-sm font-semibold">任务状态</h2>
            <div className="mt-3 space-y-3">
              <div className="flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">jobId</span>
                <code className="truncate">{effectiveJob?.id ?? '-'}</code>
              </div>
              <div className="flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">任务类型</span>
                <span>{effectiveJob?.jobType ?? '-'}</span>
              </div>
              <div className="flex justify-between gap-4 text-sm">
                <span className="text-muted-foreground">状态</span>
                <span
                  className={cn(
                    effectiveJob?.status === 'failed' && 'text-destructive',
                    effectiveJob?.status === 'canceled' && 'text-yellow-600',
                    effectiveJob?.status === 'succeeded' && 'text-green-600',
                    (effectiveJob?.status === 'running' || effectiveJob?.status === 'pending') &&
                      'text-primary',
                  )}
                >
                  {effectiveJob ? statusText[effectiveJob.status] : '-'}
                </span>
              </div>
              <ProgressSummary
                label="任务进度"
                percent={progressPercent}
                status={effectiveJob ? statusText[effectiveJob.status] : '-'}
                tone={
                  effectiveJob?.status === 'failed'
                    ? 'error'
                    : effectiveJob?.status === 'canceled'
                      ? 'warning'
                      : effectiveJob?.status === 'succeeded' ||
                          effectiveJob?.status === 'partial_succeeded'
                        ? 'success'
                        : 'default'
                }
              />
              {(effectiveJob?.status === 'pending' || effectiveJob?.status === 'running') && (
                <Button
                  variant="destructive"
                  size="sm"
                  className="w-full"
                  onClick={handleCancel}
                  title="任务取消暂不支持（Gateway 契约待补齐）"
                  disabled={cancelJobMutation.isPending}
                >
                  {cancelJobMutation.isPending && <Loader2 className="size-3 animate-spin" />}
                  <Ban className="size-3" />
                  取消任务
                </Button>
              )}
              {effectiveJob?.error?.message && (
                <p className="rounded-lg bg-destructive/10 p-3 text-sm text-destructive">
                  {effectiveJob.error.message}
                </p>
              )}
              {effectiveJob?.resultSummary && (
                <p className="rounded-lg bg-muted p-3 text-sm text-muted-foreground">
                  {effectiveJob.resultSummary}
                </p>
              )}
            </div>
          </section>

          {eventsQuery.isError && (
            <InlineNotice title="事件日志加载失败" variant="error">
              {formatReportGatewayError(eventsQuery.error, '事件日志加载失败')}
            </InlineNotice>
          )}

          {eventsQuery.data && eventsQuery.data.length > 0 && (
            <section className="rounded-lg border border-border bg-card p-4">
              <h2 className="text-sm font-semibold">事件日志</h2>
              <div className="mt-3 max-h-96 space-y-2 overflow-auto">
                {eventsQuery.data
                  .slice(-10)
                  .reverse()
                  .map((event) => (
                    <div
                      key={event.id}
                      className="rounded-lg border border-border bg-background px-3 py-2 text-xs"
                    >
                      <div className="flex justify-between text-muted-foreground">
                        <span className="font-medium">{event.eventType}</span>
                        <span>{formatDate(event.createdAt)}</span>
                      </div>
                      {event.message && <p className="mt-1 text-foreground">{event.message}</p>}
                    </div>
                  ))}
              </div>
            </section>
          )}
        </aside>
      </div>
    </div>
  )
}
