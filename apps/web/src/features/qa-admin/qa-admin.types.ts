import type { components } from '@/api/generated/gateway'

export type CreateQARetrievalTestRunRequest =
  components['schemas']['CreateQARetrievalTestRunRequest']
export type QARetrievalOptions = components['schemas']['QARetrievalOptions']
export type QARetrievalTestResult = components['schemas']['QARetrievalTestResult']
export type QARetrievalTestRun = components['schemas']['QARetrievalTestRun']
export type QAMetricsOverview = components['schemas']['QAMetricsOverview']
export type QAMetricsTrend = components['schemas']['QAMetricsTrend']
export type QAMetricsTrendPoint = components['schemas']['QAMetricsTrendPoint']
export type QATopQuery = components['schemas']['QATopQuery']
export type QAIntentDistributionItem = components['schemas']['QAIntentDistributionItem']

export type QAMetricsFilters = {
  overviewDays: number
  trendDays: number
  rankingDays: number
  rankingLimit: number
}
