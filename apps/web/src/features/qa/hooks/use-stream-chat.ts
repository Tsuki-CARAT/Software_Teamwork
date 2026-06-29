/**
 * Custom hook for SSE streaming chat.
 *
 * Wraps the low-level `streamChat` API with React-friendly state management.
 * Handlers are kept in refs so the SSE callbacks always see the latest props
 * without re-subscribing the stream.
 */

import { useCallback, useEffect, useRef, useState } from 'react'

import type { SSEHandlers } from '@/api/chat'
import { streamChat } from '@/api/chat'
import type { ChatStreamRequest } from '@/lib/types'

export function useStreamChat(handlers: SSEHandlers) {
  const [isStreaming, setIsStreaming] = useState(false)
  const abortRef = useRef<(() => void) | null>(null)
  const handlersRef = useRef(handlers)

  // Keep handlers current without re-triggering effects
  handlersRef.current = handlers

  // Abort on unmount
  useEffect(() => {
    return () => {
      abortRef.current?.()
    }
  }, [])

  const sendMessage = useCallback((params: ChatStreamRequest) => {
    // Cancel any in-flight stream
    abortRef.current?.()

    setIsStreaming(true)

    const { abort } = streamChat(params, {
      onIntentStatus: (data) => {
        handlersRef.current.onIntentStatus?.(data)
      },
      onThinkingStep: (data) => {
        handlersRef.current.onThinkingStep?.(data)
      },
      onToken: (data) => {
        handlersRef.current.onToken?.(data)
      },
      onCitation: (data) => {
        handlersRef.current.onCitation?.(data)
      },
      onDone: (data) => {
        setIsStreaming(false)
        abortRef.current = null
        handlersRef.current.onDone?.(data)
      },
      onError: (data) => {
        if (data.fatal) {
          setIsStreaming(false)
          abortRef.current = null
        }
        handlersRef.current.onError?.(data)
      },
    })

    abortRef.current = abort
  }, [])

  const abort = useCallback(() => {
    abortRef.current?.()
    abortRef.current = null
    setIsStreaming(false)
  }, [])

  return { sendMessage, abort, isStreaming }
}
