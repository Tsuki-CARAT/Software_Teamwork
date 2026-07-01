import { describe, expect, it, vi } from 'vitest'

import { streamChat } from './chat'

function streamResponse(body: string) {
  return new Response(new TextEncoder().encode(body), {
    headers: { 'Content-Type': 'text/event-stream' },
    status: 200,
  })
}

describe('chat stream API', () => {
  it('normalizes answer.delta text payloads to content', async () => {
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.delta',
            'data: {"text":"root","index":0}',
            '',
            'event: answer.delta',
            'data: {"content":" cause","index":1}',
            '',
            'event: answer.completed',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(onError).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ content: 'root', index: 0, seq: 1, text: 'root' }),
    )
    expect(onAnswerDelta).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ content: ' cause', index: 1, seq: 2 }),
    )
  })

  it('does not treat payload sequenceNo as the cross-event stream seq', async () => {
    const onMessageCreated = vi.fn()
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: message.created',
            'data: {"messageId":"assistant-1","sequenceNo":99}',
            '',
            'event: answer.delta',
            'data: {"content":"root","sequenceNo":1}',
            '',
            'event: answer.completed',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
      onMessageCreated,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(onError).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onMessageCreated).toHaveBeenCalledWith(
      expect.objectContaining({ messageId: 'assistant-1', seq: 1, sequenceNo: 99 }),
    )
    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'root', seq: 2, sequenceNo: 1 }),
    )
  })

  it('uses SSE id as the cross-event stream seq before payload seq fields', async () => {
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.delta',
            'id: 5',
            'data: {"content":"root","seq":99,"eventSeq":88}',
            '',
            'event: answer.completed',
            'id: 6',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    expect(onError).not.toHaveBeenCalled()
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'root', eventSeq: 88, seq: 5 }),
    )
    expect(onAnswerCompleted).toHaveBeenCalledWith(
      expect.objectContaining({ responseRunId: 'run-1', seq: 6 }),
    )
  })

  it('does not dispatch stale stream events to handlers', async () => {
    const onAnswerDelta = vi.fn()
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.delta',
            'id: 2',
            'data: {"content":"first"}',
            '',
            'event: answer.completed',
            'id: 1',
            'data: {"responseRunId":"old-run"}',
            '',
            'event: answer.delta',
            'id: 3',
            'data: {"content":" second"}',
            '',
            'event: answer.completed',
            'id: 4',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onError).not.toHaveBeenCalled()
    expect(onAnswerDelta).toHaveBeenCalledTimes(2)
    expect(onAnswerDelta).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ content: 'first', seq: 2 }),
    )
    expect(onAnswerDelta).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ content: ' second', seq: 3 }),
    )
    expect(onAnswerCompleted).toHaveBeenCalledWith(
      expect.objectContaining({ responseRunId: 'run-1', seq: 4 }),
    )
  })

  it('uses the dispatched max stream seq for fatal stream errors', async () => {
    const onAnswerDelta = vi.fn()
    const onError = vi.fn()
    let pullCount = 0
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        new ReadableStream<Uint8Array>({
          pull(controller) {
            pullCount += 1
            if (pullCount === 1) {
              controller.enqueue(
                new TextEncoder().encode(
                  'event: answer.delta\ndata: {"content":"root","seq":50}\n\n',
                ),
              )
              return
            }
            controller.error(new Error('connection lost'))
          },
        }),
        {
          headers: { 'Content-Type': 'text/event-stream' },
          status: 200,
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(onAnswerDelta).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'root', seq: 50 }),
    )
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'network_error',
        fatal: true,
        message: 'connection lost',
        seq: 51,
      }),
    )
  })

  it('reports fatal error when the stream ends before answer.completed', async () => {
    const onAnswerDelta = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          ['event: answer.delta', 'id: 5', 'data: {"content":"partial"}', '', ''].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))

    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'partial', seq: 5 }),
    )
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'stream_ended_without_completion',
        fatal: true,
        seq: 6,
      }),
    )
  })

  it('accepts zero as the first dispatched terminal stream seq', async () => {
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          ['event: answer.completed', 'id: 0', 'data: {"responseRunId":"run-1"}', '', ''].join(
            '\n',
          ),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onAnswerCompleted).toHaveBeenCalledWith(
      expect.objectContaining({ responseRunId: 'run-1', seq: 0 }),
    )
    expect(onError).not.toHaveBeenCalled()
  })

  it('dispatches fatal QA error events that arrive after answer.completed', async () => {
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.completed',
            'id: 3',
            'data: {"responseRunId":"run-1"}',
            '',
            'event: error',
            'id: 4',
            'data: {"code":"finalize_failed","message":"finalize failed","fatal":true}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))

    expect(onAnswerCompleted).toHaveBeenCalledWith(
      expect.objectContaining({ responseRunId: 'run-1', seq: 3 }),
    )
    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'finalize_failed',
        fatal: true,
        message: 'finalize failed',
        seq: 4,
      }),
    )
  })

  it('dispatches transport errors that arrive after answer.completed before EOF', async () => {
    const onAnswerCompleted = vi.fn()
    const onError = vi.fn()
    let pullCount = 0
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(
        new ReadableStream<Uint8Array>({
          pull(controller) {
            pullCount += 1
            if (pullCount === 1) {
              controller.enqueue(
                new TextEncoder().encode(
                  'event: answer.completed\nid: 3\ndata: {"responseRunId":"run-1"}\n\n',
                ),
              )
              return
            }
            controller.error(new Error('connection lost after completion'))
          },
        }),
        {
          headers: { 'Content-Type': 'text/event-stream' },
          status: 200,
        },
      ),
    )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onError,
    })

    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))

    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'network_error',
        fatal: true,
        message: 'connection lost after completion',
        seq: 4,
      }),
    )
  })

  it('continues dispatching after non-fatal QA error events', async () => {
    const onAnswerCompleted = vi.fn()
    const onAnswerDelta = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: error',
            'id: 1',
            'data: {"code":"dependency_error","message":"retrieval degraded","fatal":false}',
            '',
            'event: answer.delta',
            'id: 2',
            'data: {"content":"answer"}',
            '',
            'event: answer.completed',
            'id: 3',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onAnswerCompleted).toHaveBeenCalledTimes(1))

    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'dependency_error',
        fatal: false,
        message: 'retrieval degraded',
        seq: 1,
      }),
    )
    expect(onAnswerDelta).toHaveBeenCalledWith(
      expect.objectContaining({ content: 'answer', seq: 2 }),
    )
    expect(onAnswerCompleted).toHaveBeenCalledWith(
      expect.objectContaining({ responseRunId: 'run-1', seq: 3 }),
    )
  })

  it('stops dispatching events after a malformed SSE payload', async () => {
    const onAnswerCompleted = vi.fn()
    const onAnswerDelta = vi.fn()
    const onError = vi.fn()
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        streamResponse(
          [
            'event: answer.delta',
            'id: 1',
            'data: {',
            '',
            'event: answer.delta',
            'id: 2',
            'data: {"content":"late"}',
            '',
            'event: answer.completed',
            'id: 3',
            'data: {"responseRunId":"run-1"}',
            '',
            '',
          ].join('\n'),
        ),
      )
    vi.stubGlobal('fetch', fetchMock)

    streamChat('session-1', 'question', {
      onAnswerCompleted,
      onAnswerDelta,
      onError,
    })

    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1))
    await vi.waitFor(() => expect(onError).toHaveBeenCalledTimes(1))

    expect(onError).toHaveBeenCalledWith(
      expect.objectContaining({
        code: 'invalid_sse_event',
        fatal: true,
        seq: 1,
      }),
    )
    expect(onAnswerDelta).not.toHaveBeenCalled()
    expect(onAnswerCompleted).not.toHaveBeenCalled()
  })
})
