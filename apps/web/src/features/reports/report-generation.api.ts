import { buildQuery, gatewayFileRequest, gatewayPageRequest, gatewayRequest } from '@/api/client'

import type {
  CreateReportJobPayload,
  CreateReportPayload,
  Report,
  ReportDailyStatistic,
  ReportFile,
  ReportJob,
  ReportMaterial,
  ReportOutline,
  ReportSection,
  ReportStatisticsOverview,
  ReportStatus,
  ReportTemplate,
  ReportType,
  ReportTypeCode,
} from './report-generation.types'

export type ReportListParams = {
  page?: number
  pageSize?: number
  reportType?: ReportTypeCode
  status?: ReportStatus | string
  keyword?: string
}

export type ReportTemplateListParams = {
  page?: number
  pageSize?: number
  reportType?: ReportTypeCode
  enabled?: boolean
}

export type ReportMaterialListParams = {
  page?: number
  pageSize?: number
  category?: string
  enabled?: boolean
}

export function listReportTypes(): Promise<ReportType[]> {
  return gatewayRequest<ReportType[]>('/report-types')
}

export function listReportTemplates(params: ReportTemplateListParams = {}) {
  return gatewayPageRequest<ReportTemplate>(`/report-templates${buildQuery(params)}`)
}

export function listReportMaterials(params: ReportMaterialListParams = {}) {
  return gatewayPageRequest<ReportMaterial>(`/report-materials${buildQuery(params)}`)
}

export function createReport(payload: CreateReportPayload): Promise<Report> {
  return gatewayRequest<Report>('/reports', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function listReports(params: ReportListParams = {}) {
  return gatewayPageRequest<Report>(`/reports${buildQuery(params)}`)
}

export function listReportOutlines(reportId: string): Promise<ReportOutline[]> {
  return gatewayRequest<ReportOutline[]>(`/reports/${encodeURIComponent(reportId)}/outlines`)
}

export function updateReportOutline(
  reportId: string,
  outlineId: string,
  sections: ReportOutline['sections'],
): Promise<ReportOutline> {
  return gatewayRequest<ReportOutline>(
    `/reports/${encodeURIComponent(reportId)}/outlines/${encodeURIComponent(outlineId)}`,
    {
      method: 'PATCH',
      body: JSON.stringify({ sections, manualEdited: true }),
    },
  )
}

export function listReportSections(reportId: string): Promise<ReportSection[]> {
  return gatewayRequest<ReportSection[]>(`/reports/${encodeURIComponent(reportId)}/sections`)
}

export function updateReportSection(
  reportId: string,
  sectionId: string,
  payload: { title?: string; content?: string; tables?: Record<string, unknown>[] },
): Promise<ReportSection> {
  return gatewayRequest<ReportSection>(
    `/reports/${encodeURIComponent(reportId)}/sections/${encodeURIComponent(sectionId)}`,
    {
      method: 'PATCH',
      body: JSON.stringify({ ...payload, manualEdited: true }),
    },
  )
}

export function createReportJob(
  reportId: string,
  payload: CreateReportJobPayload,
): Promise<ReportJob> {
  return gatewayRequest<ReportJob>(`/reports/${encodeURIComponent(reportId)}/jobs`, {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function getReportJob(jobId: string): Promise<ReportJob> {
  return gatewayRequest<ReportJob>(`/report-jobs/${encodeURIComponent(jobId)}`)
}

export function createReportJobAttempt(jobId: string): Promise<ReportJob> {
  return gatewayRequest<ReportJob>(`/report-jobs/${encodeURIComponent(jobId)}/attempts`, {
    method: 'POST',
    body: JSON.stringify({ reason: 'frontend_retry' }),
  })
}

export function createReportFile(payload: {
  reportId: string
  format: 'docx'
  templateId?: string
  styleOptions?: Record<string, unknown>
}): Promise<ReportFile> {
  return gatewayRequest<ReportFile>('/report-files', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function downloadReportFile(reportFileId: string): Promise<Blob> {
  return gatewayFileRequest(`/report-files/${encodeURIComponent(reportFileId)}/content`)
}

export function getReportStatisticsOverview(): Promise<ReportStatisticsOverview> {
  return gatewayRequest<ReportStatisticsOverview>('/report-statistics/overview')
}

export function listDailyReportStatistics(days = 30): Promise<ReportDailyStatistic[]> {
  return gatewayRequest<ReportDailyStatistic[]>(`/report-statistics/daily${buildQuery({ days })}`)
}
