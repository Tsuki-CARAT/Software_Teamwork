import { FileText, Trash2, Upload } from 'lucide-react'
import { useState } from 'react'

import { InlineNotice, StateBlock } from '@/components/common'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Textarea } from '@/components/ui/textarea'
import type { ReportTemplate } from '@/features/reports'
import {
  formatReportGatewayError,
  useDeleteTemplate,
  useReportBootstrapQueries,
  useReportStatisticsQueries,
  useTemplateStructure,
  useUpdateTemplateStructure,
} from '@/features/reports'

export function ReportTemplatesPage() {
  const [structureTarget, setStructureTarget] = useState<string | null>(null)
  const [editMode, setEditMode] = useState(false)
  const [editJson, setEditJson] = useState('')
  const [jsonError, setJsonError] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<ReportTemplate | null>(null)

  const { templateQuery, materialQuery } = useReportBootstrapQueries()
  const { overviewQuery, dailyQuery } = useReportStatisticsQueries()
  const structureQuery = useTemplateStructure(structureTarget)
  const updateStructureMutation = useUpdateTemplateStructure(structureTarget ?? '')
  const deleteMutation = useDeleteTemplate()

  const templates = templateQuery.data?.items ?? []
  const materials = materialQuery.data?.items ?? []
  const overview = overviewQuery.data
  const daily = dailyQuery.data ?? []
  const queryErrors = [
    { error: templateQuery.error, label: '模板列表', visible: templateQuery.isError },
    { error: materialQuery.error, label: '素材列表', visible: materialQuery.isError },
    { error: overviewQuery.error, label: '统计概览', visible: overviewQuery.isError },
    { error: dailyQuery.error, label: '统计趋势', visible: dailyQuery.isError },
  ].filter((item) => item.visible)

  const handleOpenStructure = (templateId: string) => {
    setStructureTarget(templateId)
    setEditMode(false)
    setJsonError(null)
  }

  const handleCloseStructure = () => {
    setStructureTarget(null)
    setEditMode(false)
    setJsonError(null)
  }

  const handleEnterEdit = () => {
    const data = structureQuery.data
    if (data) {
      setEditJson(JSON.stringify(data, null, 2))
      setEditMode(true)
      setJsonError(null)
    }
  }

  const handleSaveEdit = () => {
    try {
      const parsed = JSON.parse(editJson) as Record<string, unknown>
      if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
        setJsonError('模板结构必须是一个 JSON 对象')
        return
      }
      setJsonError(null)
      updateStructureMutation.mutate(
        parsed as Parameters<typeof updateStructureMutation.mutate>[0],
        {
          onSuccess: () => setEditMode(false),
          onError: (error) => setJsonError(formatReportGatewayError(error, '保存失败，请重试')),
        },
      )
    } catch {
      setJsonError('JSON 格式无效，请检查语法')
    }
  }

  const handleCancelEdit = () => {
    setEditMode(false)
    setJsonError(null)
  }

  const handleDelete = () => {
    if (!deleteTarget) return
    deleteMutation.mutate(deleteTarget.id)
    setDeleteTarget(null)
  }

  const structureData = structureQuery.data
  const structureJson = structureData ? JSON.stringify(structureData, null, 2) : ''

  return (
    <div className="h-full overflow-auto bg-background p-6">
      <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold">报告模板与素材</h1>
          <p className="mt-1 text-sm text-muted-foreground">
            管理员能力入口：模板、素材、结构配置、统计和任务诊断。
          </p>
        </div>
        <div className="flex gap-2">
          <Button disabled title="上传素材表单尚未接入当前页面" variant="outline">
            <Upload className="size-4" />
            上传素材
          </Button>
          <Button disabled title="上传模板表单尚未接入当前页面">
            <Upload className="size-4" />
            上传模板
          </Button>
        </div>
      </div>

      {queryErrors.map((item) => (
        <InlineNotice
          className="mb-3"
          key={item.label}
          title={`${item.label}加载失败`}
          variant="error"
        >
          {formatReportGatewayError(item.error, `${item.label}加载失败`)}
        </InlineNotice>
      ))}

      {(templateQuery.isError || materialQuery.isError) && (
        <InlineNotice className="mb-4" title="能力边界" variant="warning">
          页面不会使用本地模板或素材示例兜底；请以 Gateway Document API 返回结果为准。
        </InlineNotice>
      )}

      <div className="mb-6 grid gap-4 md:grid-cols-3">
        <section className="rounded-lg border border-border bg-card p-4 hover:shadow-sm transition-shadow duration-200">
          <p className="text-sm text-muted-foreground">模板数量</p>
          <p className="mt-2 text-2xl font-semibold">{overview?.templateCount ?? '-'}</p>
        </section>
        <section className="rounded-lg border border-border bg-card p-4 hover:shadow-sm transition-shadow duration-200">
          <p className="text-sm text-muted-foreground">素材数量</p>
          <p className="mt-2 text-2xl font-semibold">{overview?.materialCount ?? '-'}</p>
        </section>
        <section className="rounded-lg border border-border bg-card p-4 hover:shadow-sm transition-shadow duration-200">
          <p className="text-sm text-muted-foreground">近 30 天报告</p>
          <p className="mt-2 text-2xl font-semibold">
            {overview?.reportCount ??
              (dailyQuery.isSuccess
                ? daily.reduce((total, item) => total + item.createdCount, 0)
                : '-')}
          </p>
        </section>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <section className="rounded-lg border border-border bg-card">
          <div className="border-b border-border px-4 py-3">
            <h2 className="flex items-center gap-2 text-base font-semibold">
              <FileText className="size-4" />
              模板列表
            </h2>
          </div>
          <div className="divide-y divide-border">
            {templateQuery.isLoading ? (
              <StateBlock size="compact" title="模板加载中" variant="loading" />
            ) : templateQuery.isError ? (
              <StateBlock
                description={formatReportGatewayError(templateQuery.error, '模板列表加载失败')}
                size="compact"
                title="模板列表加载失败"
                variant="error"
              />
            ) : templates.length === 0 ? (
              <StateBlock size="compact" title="暂无报告模板" variant="empty" />
            ) : (
              templates.map((template) => (
                <div
                  key={template.id}
                  className="flex items-center justify-between gap-4 p-4 hover:bg-muted/20 transition-colors"
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium">{template.templateName}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {template.reportType} · v{template.version} · {template.filename}
                    </p>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <Button
                      variant="outline"
                      size="xs"
                      onClick={() => handleOpenStructure(template.id)}
                    >
                      查看结构
                    </Button>
                    <span className="rounded-full bg-muted px-2 py-1 text-xs">
                      {template.enabled ? '启用' : '停用'}
                    </span>
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      aria-label="删除模板"
                      onClick={() => setDeleteTarget(template)}
                    >
                      <Trash2 className="size-3 text-destructive" />
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
        </section>

        <section className="rounded-lg border border-border bg-card">
          <div className="border-b border-border px-4 py-3">
            <h2 className="flex items-center gap-2 text-base font-semibold">
              <FileText className="size-4" />
              专业素材
            </h2>
          </div>
          <div className="divide-y divide-border">
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
              <StateBlock size="compact" title="暂无报告素材" variant="empty" />
            ) : (
              materials.map((material) => (
                <div
                  key={material.id}
                  className="flex items-center justify-between gap-4 p-4 hover:bg-muted/20 transition-colors"
                >
                  <div className="min-w-0">
                    <p className="truncate text-sm font-medium">{material.materialName}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {material.category ?? '-'} · {material.materialType ?? 'material'}
                    </p>
                  </div>
                  <span className="rounded-full bg-muted px-2 py-1 text-xs">
                    {material.enabled ? '可引用' : '停用'}
                  </span>
                </div>
              ))
            )}
          </div>
        </section>
      </div>

      {/* Template structure viewer / editor dialog */}
      <Dialog
        open={Boolean(structureTarget)}
        onOpenChange={(open) => !open && handleCloseStructure()}
      >
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {structureTarget
                ? `模板结构 - ${templates.find((t) => t.id === structureTarget)?.templateName ?? structureTarget}`
                : '模板结构'}
            </DialogTitle>
            <DialogDescription>
              {editMode
                ? '编辑模板的 outlineSchema 和 styleConfig 配置。'
                : '模板的 JSON 结构定义。'}
            </DialogDescription>
          </DialogHeader>

          {structureQuery.isLoading && (
            <div className="py-4 text-center text-sm text-muted-foreground">加载中...</div>
          )}

          {structureQuery.isError && (
            <div className="py-4 text-center text-sm text-destructive">
              {formatReportGatewayError(structureQuery.error, '模板结构加载失败')}
            </div>
          )}

          {!structureQuery.isLoading && !structureQuery.isError && (
            <>
              {editMode ? (
                <div className="flex flex-col gap-2">
                  <Textarea
                    className="min-h-80 font-mono text-xs"
                    value={editJson}
                    onChange={(event) => {
                      setEditJson(event.target.value)
                      setJsonError(null)
                    }}
                    placeholder='{"outlineSchema": [...], "styleConfig": {...}}'
                  />
                  {jsonError && <p className="text-xs text-destructive">{jsonError}</p>}
                </div>
              ) : (
                <pre className="max-h-96 overflow-auto rounded-lg bg-muted p-4 font-mono text-xs leading-relaxed">
                  {structureJson || '{}'}
                </pre>
              )}
            </>
          )}

          <DialogFooter>
            {!editMode ? (
              <>
                <Button variant="outline" onClick={handleCloseStructure}>
                  关闭
                </Button>
                {structureTarget && (
                  <Button onClick={handleEnterEdit} disabled={structureQuery.isError}>
                    编辑结构
                  </Button>
                )}
              </>
            ) : (
              <>
                <Button variant="outline" onClick={handleCancelEdit}>
                  取消
                </Button>
                <Button onClick={handleSaveEdit} disabled={updateStructureMutation.isPending}>
                  {updateStructureMutation.isPending ? '保存中...' : '保存'}
                </Button>
              </>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete template confirmation dialog */}
      <Dialog open={Boolean(deleteTarget)} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>确定删除此模板？</DialogTitle>
            <DialogDescription>
              {deleteTarget?.templateName
                ? `即将删除模板"${deleteTarget.templateName}"。此操作不可撤销。`
                : '此操作不可撤销。'}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleteTarget(null)}>
              取消
            </Button>
            <Button
              variant="destructive"
              onClick={handleDelete}
              disabled={deleteMutation.isPending}
            >
              {deleteMutation.isPending ? '删除中...' : '确认删除'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
