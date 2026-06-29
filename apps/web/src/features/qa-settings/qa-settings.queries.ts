import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  createQAConfigVersion,
  createQALLMConfigVersion,
  createQALLMConnectionTest,
  getCurrentQAConfigVersion,
  getCurrentQALLMConfigVersion,
} from './qa-settings.api'
import type {
  CreateQAConfigVersionRequest,
  CreateQALLMConfigVersionRequest,
  CreateQALLMConnectionTestRequest,
} from './qa-settings.types'

export const qaSettingsKeys = {
  all: ['qa-settings'] as const,
  qaCurrent: () => [...qaSettingsKeys.all, 'qa-config-current'] as const,
  llmCurrent: () => [...qaSettingsKeys.all, 'llm-config-current'] as const,
}

export function useQASettingsQueries() {
  const qaConfigQuery = useQuery({
    queryKey: qaSettingsKeys.qaCurrent(),
    queryFn: getCurrentQAConfigVersion,
  })
  const llmConfigQuery = useQuery({
    queryKey: qaSettingsKeys.llmCurrent(),
    queryFn: getCurrentQALLMConfigVersion,
  })

  return { qaConfigQuery, llmConfigQuery }
}

export function useCreateQAConfigVersionMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateQAConfigVersionRequest) => createQAConfigVersion(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: qaSettingsKeys.qaCurrent() })
    },
  })
}

export function useCreateQALLMConfigVersionMutation() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (payload: CreateQALLMConfigVersionRequest) => createQALLMConfigVersion(payload),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: qaSettingsKeys.llmCurrent() })
    },
  })
}

export function useCreateQALLMConnectionTestMutation() {
  return useMutation({
    mutationFn: (payload: CreateQALLMConnectionTestRequest) => createQALLMConnectionTest(payload),
  })
}
