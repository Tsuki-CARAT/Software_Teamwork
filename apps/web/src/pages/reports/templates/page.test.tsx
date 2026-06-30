import { screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { renderWithProviders } from '@/test/render'

import { ReportTemplatesPage } from './page'

function gatewayError(code: string, message: string, requestId: string, status = 503) {
  return new Response(JSON.stringify({ error: { code, message, requestId } }), {
    headers: { 'Content-Type': 'application/json' },
    status,
  })
}

describe('ReportTemplatesPage', () => {
  it('shows gateway errors instead of local fallback templates or materials', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn<typeof fetch>()
        .mockImplementation(async () =>
          gatewayError('dependency_error', 'Document templates unavailable', 'req-templates'),
        ),
    )

    renderWithProviders(<ReportTemplatesPage />)

    expect((await screen.findAllByText(/Document templates unavailable/))[0]).toBeVisible()
    expect(screen.getAllByText(/req-templates/).length).toBeGreaterThan(0)
    expect(screen.queryByText('迎峰度夏默认模板')).not.toBeInTheDocument()
    expect(screen.queryByText('设备运行台账与缺陷闭环记录')).not.toBeInTheDocument()
  })
})
