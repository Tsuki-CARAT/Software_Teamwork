import { fireEvent, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { renderWithProviders } from '@/test/render'

import { ReportGeneratePage } from './page'

function jsonResponse(body: unknown, init?: ResponseInit) {
  return new Response(JSON.stringify(body), {
    headers: { 'Content-Type': 'application/json', ...init?.headers },
    status: init?.status ?? 200,
    statusText: init?.statusText,
  })
}

function gatewayError(code: string, message: string, requestId: string, status = 503) {
  return jsonResponse({ error: { code, message, requestId } }, { status })
}

function pageResponse(data: unknown[]) {
  return jsonResponse({
    data,
    page: { page: 1, pageSize: 20, total: data.length },
    requestId: 'req-page',
  })
}

const reportType = {
  code: 'summer_peak_inspection',
  defaultTemplateId: 'tpl-real',
  description: '真实服务返回的报告类型',
  enabled: true,
  name: '真实巡检报告',
}

const reportTemplate = {
  createdAt: '2026-06-30T00:00:00Z',
  enabled: true,
  filename: 'real-template.docx',
  id: 'tpl-real',
  reportType: 'summer_peak_inspection',
  templateName: '真实模板',
  version: 1,
}

const reportMaterial = {
  category: '真实素材',
  createdAt: '2026-06-30T00:00:00Z',
  enabled: true,
  id: 'mat-real',
  materialName: '真实素材',
  materialType: 'technical_doc',
}

describe('ReportGeneratePage', () => {
  it('does not render local bootstrap fallback data when gateway bootstrap queries fail', async () => {
    vi.stubGlobal(
      'fetch',
      vi
        .fn<typeof fetch>()
        .mockImplementation(async () =>
          gatewayError('dependency_error', 'Document dependency down', 'req-bootstrap'),
        ),
    )

    renderWithProviders(<ReportGeneratePage />)

    expect((await screen.findAllByText(/Document dependency down/))[0]).toBeVisible()
    expect(screen.getAllByText(/req-bootstrap/).length).toBeGreaterThan(0)
    expect(screen.queryByRole('option', { name: '煤库存审计报告' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /创建草稿/ })).toBeDisabled()
  })

  it('shows gateway request id and does not create a local report when draft creation is not implemented', async () => {
    const fetchMock = vi.fn(async (request: RequestInfo | URL) => {
      const url = new URL(request instanceof Request ? request.url : String(request))

      if (url.pathname.endsWith('/report-types')) {
        return jsonResponse({ data: [reportType], requestId: 'req-types' })
      }
      if (url.pathname.endsWith('/report-templates')) {
        return pageResponse([reportTemplate])
      }
      if (url.pathname.endsWith('/report-materials')) {
        return pageResponse([reportMaterial])
      }
      if (url.pathname.endsWith('/reports')) {
        return gatewayError(
          'not_implemented',
          'Real report creation is not ready',
          'req-create-501',
          501,
        )
      }

      return jsonResponse({ data: [], requestId: 'req-empty' })
    })
    vi.stubGlobal('fetch', fetchMock)

    renderWithProviders(<ReportGeneratePage />)

    await screen.findByRole('option', { name: '真实巡检报告' })
    await waitFor(() => expect(screen.getByRole('button', { name: /创建草稿/ })).toBeEnabled())

    fireEvent.click(screen.getByRole('button', { name: /创建草稿/ }))

    expect(await screen.findByText(/Real report creation is not ready/)).toBeVisible()
    expect(screen.getByText(/req-create-501/)).toBeVisible()
    expect(screen.queryByText(/local-report/)).not.toBeInTheDocument()
    expect(screen.queryByText(/已进入本地原型流程/)).not.toBeInTheDocument()
  })
})
