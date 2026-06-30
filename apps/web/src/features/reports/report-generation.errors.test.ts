import { describe, expect, it } from 'vitest'

import { ApiError } from '@/api/client'

import { formatReportGatewayError, getReportGatewayErrorDetails } from './report-generation.errors'

describe('report generation gateway error helpers', () => {
  it('preserves gateway message and request id for user-visible diagnostics', () => {
    const error = new ApiError({
      code: 'dependency_error',
      message: 'Document service unavailable',
      requestId: 'req-report-1',
      status: 503,
    })

    expect(getReportGatewayErrorDetails(error)).toEqual({
      code: 'dependency_error',
      isCapabilityUnavailable: true,
      message: 'Document service unavailable',
      requestId: 'req-report-1',
      status: 503,
    })
    expect(formatReportGatewayError(error)).toBe(
      'Document service unavailable（requestId: req-report-1）',
    )
  })
})
