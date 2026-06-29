import { buildQuery, gatewayRequest } from '@/api/client'

import type {
  CreateQARetrievalTestRunRequest,
  QAIntentDistributionItem,
  QAMetricsOverview,
  QAMetricsTrend,
  QARetrievalTestRun,
  QATopQuery,
} from './qa-admin.types'

export function createQARetrievalTestRun(
  payload: CreateQARetrievalTestRunRequest,
): Promise<QARetrievalTestRun> {
  return gatewayRequest<QARetrievalTestRun>('/retrieval-test-runs', {
    method: 'POST',
    body: payload,
  })
}

export function getQARetrievalTestRun(testRunId: string): Promise<QARetrievalTestRun> {
  return gatewayRequest<QARetrievalTestRun>(`/retrieval-test-runs/${encodeURIComponent(testRunId)}`)
}

export function getQAMetricsOverview(days: number): Promise<QAMetricsOverview> {
  return gatewayRequest<QAMetricsOverview>(`/qa-metrics/overview${buildQuery({ days })}`)
}

export function getQAMetricsTrend(days: number): Promise<QAMetricsTrend> {
  return gatewayRequest<QAMetricsTrend>(`/qa-metrics/trend${buildQuery({ days })}`)
}

export function listQATopQueries(days: number, limit: number): Promise<QATopQuery[]> {
  return gatewayRequest<QATopQuery[]>(`/qa-metrics/top-queries${buildQuery({ days, limit })}`)
}

export function listQAIntentDistribution(days: number): Promise<QAIntentDistributionItem[]> {
  return gatewayRequest<QAIntentDistributionItem[]>(
    `/qa-metrics/intent-distribution${buildQuery({ days })}`,
  )
}
