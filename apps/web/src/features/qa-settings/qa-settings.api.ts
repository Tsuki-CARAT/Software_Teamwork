import { gatewayRequest } from '@/api/client'

import type {
  CreateQAConfigVersionRequest,
  CreateQALLMConfigVersionRequest,
  CreateQALLMConnectionTestRequest,
  QAConfigVersion,
  QALLMConfigVersion,
  QALLMConnectionTest,
} from './qa-settings.types'

export function getCurrentQAConfigVersion(): Promise<QAConfigVersion> {
  return gatewayRequest<QAConfigVersion>('/qa-config-versions/current')
}

export function createQAConfigVersion(
  payload: CreateQAConfigVersionRequest,
): Promise<QAConfigVersion> {
  return gatewayRequest<QAConfigVersion>('/qa-config-versions', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function getCurrentQALLMConfigVersion(): Promise<QALLMConfigVersion> {
  return gatewayRequest<QALLMConfigVersion>('/llm-config-versions/current')
}

export function createQALLMConfigVersion(
  payload: CreateQALLMConfigVersionRequest,
): Promise<QALLMConfigVersion> {
  return gatewayRequest<QALLMConfigVersion>('/llm-config-versions', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}

export function createQALLMConnectionTest(
  payload: CreateQALLMConnectionTestRequest,
): Promise<QALLMConnectionTest> {
  return gatewayRequest<QALLMConnectionTest>('/llm-connection-tests', {
    method: 'POST',
    body: JSON.stringify(payload),
  })
}
