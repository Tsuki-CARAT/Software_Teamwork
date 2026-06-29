import { FileText, Upload } from 'lucide-react'

import { Button } from '@/components/ui/button'
import type { ReportMaterial, ReportTemplate } from '@/features/reports'
import { useReportBootstrapQueries, useReportStatisticsQueries } from '@/features/reports'

const fallbackTemplates: ReportTemplate[] = [
  {
    id: 'tpl-local-summer',
    templateName: '迎峰度夏默认模板',
    reportType: 'summer_peak_inspection',
    version: 1,
    enabled: true,
    filename: 'summer-peak-template.docx',
    createdAt: '2026-06-28T10:00:00Z',
  },
  {
    id: 'tpl-local-coal',
    templateName: '煤库存审计模板',
    reportType: 'coal_inventory_audit',
    version: 1,
    enabled: true,
    filename: 'coal-inventory-template.docx',
    createdAt: '2026-06-28T10:00:00Z',
  },
]

const fallbackMaterials: ReportMaterial[] = [
  {
    id: 'mat-equipment-ledger',
    materialName: '设备运行台账与缺陷闭环记录',
    materialType: 'plant_report',
    category: '运行资料',
    enabled: true,
    createdAt: '2026-06-28T10:00:00Z',
  },
  {
    id: 'mat-risk-standard',
    materialName: '迎峰度夏风险检查标准',
    materialType: 'technical_doc',
    category: '技术标准',
    enabled: true,
    createdAt: '2026-06-28T10:00:00Z',
  },
]

export function ReportTemplatesPage() {
  const { templateQuery, materialQuery } = useReportBootstrapQueries()
  const { overviewQuery, dailyQuery } = useReportStatisticsQueries()
  const templates = templateQuery.data?.items.length ? templateQuery.data.items : fallbackTemplates
  const materials = materialQuery.data?.items.length ? materialQuery.data.items : fallbackMaterials
  const overview = overviewQuery.data
  const daily = dailyQuery.data ?? []

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
          <Button variant="outline">
            <Upload className="size-4" />
            上传素材
          </Button>
          <Button>
            <Upload className="size-4" />
            上传模板
          </Button>
        </div>
      </div>

      {(templateQuery.isError || materialQuery.isError || overviewQuery.isError) && (
        <div className="mb-4 rounded-lg border border-border bg-card px-4 py-3 text-sm text-muted-foreground">
          gateway 暂未联通，当前展示本地模板、素材和统计示例。
        </div>
      )}

      <div className="mb-6 grid gap-4 md:grid-cols-3">
        <section className="rounded-lg border border-border bg-card p-4">
          <p className="text-sm text-muted-foreground">模板数量</p>
          <p className="mt-2 text-2xl font-semibold">
            {overview?.templateCount ?? templates.length}
          </p>
        </section>
        <section className="rounded-lg border border-border bg-card p-4">
          <p className="text-sm text-muted-foreground">素材数量</p>
          <p className="mt-2 text-2xl font-semibold">
            {overview?.materialCount ?? materials.length}
          </p>
        </section>
        <section className="rounded-lg border border-border bg-card p-4">
          <p className="text-sm text-muted-foreground">近 30 天报告</p>
          <p className="mt-2 text-2xl font-semibold">
            {overview?.reportCount ?? daily.reduce((total, item) => total + item.createdCount, 0)}
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
            {templates.map((template) => (
              <div key={template.id} className="flex items-center justify-between gap-4 p-4">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{template.templateName}</p>
                  <p className="mt-1 text-xs text-muted-foreground">
                    {template.reportType} · v{template.version} · {template.filename}
                  </p>
                </div>
                <span className="rounded-full bg-muted px-2 py-1 text-xs">
                  {template.enabled ? '启用' : '停用'}
                </span>
              </div>
            ))}
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
            {materials.map((material) => (
              <div key={material.id} className="flex items-center justify-between gap-4 p-4">
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
            ))}
          </div>
        </section>
      </div>
    </div>
  )
}
