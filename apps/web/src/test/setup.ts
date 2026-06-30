import '@testing-library/jest-dom/vitest'

import { cleanup } from '@testing-library/react'
import { afterEach, beforeEach, vi } from 'vitest'

import { resetApiClientForTests } from '@/api/client'

beforeEach(() => {
  resetApiClientForTests()
})

afterEach(() => {
  cleanup()
  resetApiClientForTests()
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
  localStorage.clear()
  sessionStorage.clear()
})
