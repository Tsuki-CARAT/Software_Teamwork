/**
 * Base API clients with error handling.
 *
 * `doRequest` is the legacy helper still used by existing QA/admin pages.
 * New gateway integrations should use `gatewayRequest` / `gatewayPageRequest`
 * for the current `{ data, requestId }` and `{ data, page, requestId }`
 * envelope from `docs/services/gateway/api/openapi.yaml`.
 */

export class ApiError extends Error {
  code: number | string
  requestId?: string
  fields?: Record<string, string>

  constructor(
    code: number | string,
    message: string,
    options?: { requestId?: string; fields?: Record<string, string> },
  ) {
    super(message)
    this.name = 'ApiError'
    this.code = code
    this.requestId = options?.requestId
    this.fields = options?.fields
  }
}

/** Resolved at build time by Vite; falls back to same-origin `/api`. */
export const apiClient = {
  baseUrl: import.meta.env?.VITE_API_BASE_URL ?? '/api/v1',
}

interface ApiEnvelope<T> {
  code: number
  message: string
  data: T
}

interface GatewayEnvelope<T> {
  data: T
  requestId: string
}

export interface GatewayPage {
  page: number
  pageSize: number
  total: number
}

interface GatewayPageEnvelope<T> extends GatewayEnvelope<T[]> {
  page: GatewayPage
}

interface GatewayErrorEnvelope {
  error?: {
    code?: string
    message?: string
    requestId?: string
    fields?: Record<string, string>
  }
}

/**
 * Generic request helper that handles the unified { code, message, data }
 * envelope.  Throws `ApiError` for non-0 code or non-OK HTTP status.
 */
export async function doRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${apiClient.baseUrl}${path}`
  const res = await fetch(url, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
  })

  if (!res.ok) {
    throw new ApiError(res.status, `HTTP ${res.status}: ${res.statusText}`)
  }

  const json: ApiEnvelope<T> = await res.json()
  if (json.code !== 0) {
    throw new ApiError(json.code, json.message)
  }

  return json.data
}

function getAccessToken(): string | null {
  return (
    window.localStorage.getItem('accessToken') ??
    window.localStorage.getItem('qa-access-token') ??
    window.localStorage.getItem('auth.accessToken')
  )
}

function createRequestId(): string {
  return `req-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function gatewayHeaders(body?: BodyInit | null): HeadersInit {
  const token = getAccessToken()
  const headers: Record<string, string> = {
    'X-Request-Id': createRequestId(),
  }

  if (!(body instanceof FormData)) {
    headers['Content-Type'] = 'application/json'
  }

  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  return headers
}

export function buildQuery(
  params: Record<string, string | number | boolean | undefined | null>,
): string {
  const search = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      search.set(key, String(value))
    }
  }

  const query = search.toString()
  return query ? `?${query}` : ''
}

async function readGatewayError(res: Response): Promise<ApiError> {
  const fallbackMessage = `HTTP ${res.status}: ${res.statusText}`

  try {
    const json = (await res.json()) as GatewayErrorEnvelope
    const error = json.error
    return new ApiError(error?.code ?? res.status, error?.message ?? fallbackMessage, {
      requestId: error?.requestId,
      fields: error?.fields,
    })
  } catch {
    return new ApiError(res.status, fallbackMessage)
  }
}

export async function gatewayRequest<T>(path: string, options?: RequestInit): Promise<T> {
  const body = options?.body ?? null
  const res = await fetch(`${apiClient.baseUrl}${path}`, {
    ...options,
    headers: {
      ...gatewayHeaders(body),
      ...options?.headers,
    },
  })

  if (!res.ok) {
    throw await readGatewayError(res)
  }

  if (res.status === 204) {
    return undefined as T
  }

  const json = (await res.json()) as GatewayEnvelope<T>
  return json.data
}

export async function gatewayPageRequest<T>(
  path: string,
  options?: RequestInit,
): Promise<{ items: T[]; page: GatewayPage }> {
  const body = options?.body ?? null
  const res = await fetch(`${apiClient.baseUrl}${path}`, {
    ...options,
    headers: {
      ...gatewayHeaders(body),
      ...options?.headers,
    },
  })

  if (!res.ok) {
    throw await readGatewayError(res)
  }

  const json = (await res.json()) as GatewayPageEnvelope<T>
  return { items: json.data, page: json.page }
}

export async function gatewayFileRequest(path: string): Promise<Blob> {
  const res = await fetch(`${apiClient.baseUrl}${path}`, {
    headers: {
      ...gatewayHeaders(),
      Accept:
        'application/vnd.openxmlformats-officedocument.wordprocessingml.document, application/octet-stream',
    },
  })

  if (!res.ok) {
    throw await readGatewayError(res)
  }

  return res.blob()
}
