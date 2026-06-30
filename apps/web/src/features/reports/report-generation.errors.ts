import { ApiError } from '@/api/client'

export type ReportGatewayErrorDetails = {
  code?: string
  isCapabilityUnavailable: boolean
  message: string
  requestId?: string
  status?: number
}

const unavailableCodes = new Set(['dependency_error', 'not_implemented'])

export function getReportGatewayErrorDetails(
  error: unknown,
  defaultMessage = '请求失败，请稍后重试',
): ReportGatewayErrorDetails {
  if (error instanceof ApiError) {
    return {
      code: error.code,
      isCapabilityUnavailable: unavailableCodes.has(error.code) || error.status === 501,
      message: error.message || defaultMessage,
      requestId: error.requestId,
      status: error.status,
    }
  }

  if (error instanceof Error) {
    return {
      isCapabilityUnavailable: false,
      message: error.message || defaultMessage,
    }
  }

  return {
    isCapabilityUnavailable: false,
    message: defaultMessage,
  }
}

export function formatReportGatewayError(
  error: unknown,
  defaultMessage = '请求失败，请稍后重试',
): string {
  const details = getReportGatewayErrorDetails(error, defaultMessage)
  return details.requestId
    ? `${details.message}（requestId: ${details.requestId}）`
    : details.message
}
