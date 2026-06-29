import type { components } from '@/api/generated/gateway'

export type ReportTypeCode = string

export type ReportStatus = components['schemas']['ReportStatus']
export type ReportJobStatus = components['schemas']['ReportJobStatus']
export type ReportJobType = components['schemas']['ReportJobType']
export type ReportType = components['schemas']['ReportType']
export type ReportTemplate = components['schemas']['ReportTemplate']
export type ReportMaterial = components['schemas']['ReportMaterial']
export type Report = components['schemas']['Report']
export type ReportOutlineNode = components['schemas']['ReportOutlineNode']
export type ReportOutline = components['schemas']['ReportOutline']
export type ReportSection = components['schemas']['ReportSection']
export type ReportJob = components['schemas']['ReportJob']
export type ReportFile = components['schemas']['ReportFile']
export type ReportStatisticsOverview = components['schemas']['ReportStatisticsOverview']
export type ReportDailyStatistic = components['schemas']['ReportDailyStatistic']
export type CreateReportPayload = components['schemas']['CreateReportRequest']
export type CreateReportJobPayload = components['schemas']['CreateReportJobRequest']
