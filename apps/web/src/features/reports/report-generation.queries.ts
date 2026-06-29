import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createReport,
  createReportFile,
  createReportJob,
  createReportJobAttempt,
  downloadReportFile,
  getReportJob,
  getReportStatisticsOverview,
  listDailyReportStatistics,
  listReportMaterials,
  listReportOutlines,
  listReports,
  listReportSections,
  listReportTemplates,
  listReportTypes,
  updateReportOutline,
  updateReportSection,
} from './report-generation.api'
import type {
  CreateReportJobPayload,
  CreateReportPayload,
  ReportOutline,
} from './report-generation.types'

export const reportKeys = {
  all: ['reports'] as const,
  types: () => [...reportKeys.all, 'types'] as const,
  templates: () => [...reportKeys.all, 'templates'] as const,
  materials: () => [...reportKeys.all, 'materials'] as const,
  records: () => [...reportKeys.all, 'records'] as const,
  recordList: (keyword: string) => [...reportKeys.records(), { keyword }] as const,
  outlines: (reportId: string) => [...reportKeys.all, reportId, 'outlines'] as const,
  sections: (reportId: string) => [...reportKeys.all, reportId, 'sections'] as const,
  job: (jobId: string) => [...reportKeys.all, 'jobs', jobId] as const,
  stats: () => [...reportKeys.all, 'statistics'] as const,
}

export function useReportBootstrapQueries(reportType?: string) {
  const typeQuery = useQuery({
    queryKey: reportKeys.types(),
    queryFn: listReportTypes,
  })
  const templateQuery = useQuery({
    queryKey: [...reportKeys.templates(), { reportType }],
    queryFn: () =>
      listReportTemplates({
        reportType,
        enabled: true,
        page: 1,
        pageSize: 20,
      }),
  })
  const materialQuery = useQuery({
    queryKey: reportKeys.materials(),
    queryFn: () => listReportMaterials({ enabled: true, page: 1, pageSize: 20 }),
  })

  return { typeQuery, templateQuery, materialQuery }
}

export function useReportsQuery(keyword = '') {
  return useQuery({
    queryKey: reportKeys.recordList(keyword),
    queryFn: () => listReports({ keyword, page: 1, pageSize: 20 }),
  })
}

export function useReportDetailQueries(reportId: string | null) {
  const enabled = Boolean(reportId)
  const outlinesQuery = useQuery({
    queryKey: reportKeys.outlines(reportId ?? ''),
    queryFn: () => listReportOutlines(reportId ?? ''),
    enabled,
  })
  const sectionsQuery = useQuery({
    queryKey: reportKeys.sections(reportId ?? ''),
    queryFn: () => listReportSections(reportId ?? ''),
    enabled,
  })

  return { outlinesQuery, sectionsQuery }
}

export function useReportJobQuery(jobId: string | null) {
  return useQuery({
    queryKey: reportKeys.job(jobId ?? ''),
    queryFn: () => getReportJob(jobId ?? ''),
    enabled: Boolean(jobId),
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status === 'pending' || status === 'running' ? 3000 : false
    },
  })
}

export function useCreateReportMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateReportPayload) => createReport(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: reportKeys.records() })
    },
  })
}

export function useCreateReportJobMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({ reportId, payload }: { reportId: string; payload: CreateReportJobPayload }) =>
      createReportJob(reportId, payload),
    onSuccess: (job) => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.outlines(job.reportId),
      })
      void queryClient.invalidateQueries({
        queryKey: reportKeys.sections(job.reportId),
      })
      void queryClient.invalidateQueries({ queryKey: reportKeys.records() })
    },
  })
}

export function useUpdateReportOutlineMutation(reportId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      outlineId,
      sections,
    }: {
      outlineId: string
      sections: ReportOutline['sections']
    }) => updateReportOutline(reportId, outlineId, sections),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.outlines(reportId),
      })
    },
  })
}

export function useUpdateReportSectionMutation(reportId: string) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: ({
      sectionId,
      title,
      content,
    }: {
      sectionId: string
      title?: string
      content?: string
    }) => updateReportSection(reportId, sectionId, { title, content }),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: reportKeys.sections(reportId),
      })
    },
  })
}

export function useCreateReportFileMutation() {
  return useMutation({
    mutationFn: createReportFile,
  })
}

export function useRetryReportJobMutation() {
  return useMutation({
    mutationFn: (jobId: string) => createReportJobAttempt(jobId),
  })
}

export function useDownloadReportFileMutation() {
  return useMutation({
    mutationFn: (reportFileId: string) => downloadReportFile(reportFileId),
  })
}

export function useReportStatisticsQueries() {
  const overviewQuery = useQuery({
    queryKey: reportKeys.stats(),
    queryFn: getReportStatisticsOverview,
  })
  const dailyQuery = useQuery({
    queryKey: [...reportKeys.stats(), 'daily'],
    queryFn: () => listDailyReportStatistics(30),
  })

  return { overviewQuery, dailyQuery }
}
