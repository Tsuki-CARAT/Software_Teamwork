import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { renderWithProviders } from '@/test/render'

import { ReportRecordsPage } from './page'

function gatewayError(code: string, message: string, requestId: string, status = 503) {
  return new Response(JSON.stringify({ error: { code, message, requestId } }), {
    headers: { 'Content-Type': 'application/json' },
    status,
  })
}

describe('ReportRecordsPage', () => {
  it('shows gateway errors instead of local fallback report records', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          gatewayError('dependency_error', 'Document reports unavailable', 'req-records'),
        ),
    )

    renderWithProviders(<ReportRecordsPage />)

    expect((await screen.findAllByText(/Document reports unavailable/))[0]).toBeVisible()
    expect(screen.getAllByText(/req-records/).length).toBeGreaterThan(0)
    expect(screen.queryByText('2026年迎峰度夏检查报告')).not.toBeInTheDocument()
  })
})
