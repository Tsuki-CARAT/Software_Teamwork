import { useMutation, useQuery } from '@tanstack/react-query'

import {
  createQARetrievalTestRun,
  getQAMetricsOverview,
  getQAMetricsTrend,
  getQARetrievalTestRun,
  listQAIntentDistribution,
  listQATopQueries,
} from './qa-admin.api'
import type { CreateQARetrievalTestRunRequest, QAMetricsFilters } from './qa-admin.types'

export const qaAdminKeys = {
  all: ['qa-admin'] as const,
  retrievalRun: (testRunId: string) =>
    [...qaAdminKeys.all, 'retrieval-test-run', testRunId] as const,
  metricsOverview: (days: number) => [...qaAdminKeys.all, 'metrics-overview', days] as const,
  metricsTrend: (days: number) => [...qaAdminKeys.all, 'metrics-trend', days] as const,
  metricsTopQueries: (days: number, limit: number) =>
    [...qaAdminKeys.all, 'metrics-top-queries', days, limit] as const,
  metricsIntentDistribution: (days: number) =>
    [...qaAdminKeys.all, 'metrics-intent-distribution', days] as const,
}

export function useCreateQARetrievalTestRunMutation() {
  return useMutation({
    mutationFn: (payload: CreateQARetrievalTestRunRequest) => createQARetrievalTestRun(payload),
  })
}

export function useQARetrievalTestRunQuery(testRunId: string | null, enabled: boolean) {
  return useQuery({
    queryKey: qaAdminKeys.retrievalRun(testRunId ?? ''),
    queryFn: () => getQARetrievalTestRun(testRunId ?? ''),
    enabled: enabled && Boolean(testRunId),
    refetchInterval: (query) => (query.state.data?.status === 'running' ? 2000 : false),
  })
}

export function useQAMetricsQueries(filters: QAMetricsFilters) {
  const overviewQuery = useQuery({
    queryKey: qaAdminKeys.metricsOverview(filters.overviewDays),
    queryFn: () => getQAMetricsOverview(filters.overviewDays),
  })
  const trendQuery = useQuery({
    queryKey: qaAdminKeys.metricsTrend(filters.trendDays),
    queryFn: () => getQAMetricsTrend(filters.trendDays),
  })
  const topQueriesQuery = useQuery({
    queryKey: qaAdminKeys.metricsTopQueries(filters.rankingDays, filters.rankingLimit),
    queryFn: () => listQATopQueries(filters.rankingDays, filters.rankingLimit),
  })
  const intentDistributionQuery = useQuery({
    queryKey: qaAdminKeys.metricsIntentDistribution(filters.rankingDays),
    queryFn: () => listQAIntentDistribution(filters.rankingDays),
  })

  return {
    overviewQuery,
    trendQuery,
    topQueriesQuery,
    intentDistributionQuery,
  }
}
