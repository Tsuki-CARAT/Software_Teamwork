import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from '@tanstack/react-router'

import { AppLayout } from '@/layouts/app-layout'
import type { PermissionRequirement } from '@/lib/permissions'
import { canAccess } from '@/lib/permissions'
import { FileManagement } from '@/pages/admin/file-management'
import { KnowledgeConfig } from '@/pages/admin/knowledge-config'
import { KnowledgeExperience } from '@/pages/admin/knowledge-experience'
import { KnowledgeManagement } from '@/pages/admin/knowledge-management'
import { MaterialManagement } from '@/pages/admin/material-management'
import { AdminPage } from '@/pages/admin/page'
import { PromptManagement } from '@/pages/admin/prompt-management'
import { QARetrievalTestPage } from '@/pages/admin/qa-retrieval-test'
import { QASettings } from '@/pages/admin/qa-settings'
import { ReportCategory } from '@/pages/admin/report-category'
import { RoleManagement } from '@/pages/admin/role-management'
import { StatsOverviewPage } from '@/pages/admin/stats-overview'
import { StyleManagement } from '@/pages/admin/style-management'
import { SystemSettings } from '@/pages/admin/system-settings'
import { TemplateManagement } from '@/pages/admin/template-management'
import { UserManagement } from '@/pages/admin/user-management'
import { ForbiddenPage } from '@/pages/auth/forbidden'
import { LoginPage } from '@/pages/login/page'
import { ChatPage } from '@/pages/qa/chat/page'
import { ReportGeneratePage } from '@/pages/reports/generate/page'
import { ReportRecordsPage } from '@/pages/reports/records/page'
import { ReportTemplatesPage } from '@/pages/reports/templates/page'
import { useAuthStore } from '@/stores/auth-store'

async function restoreAuthForRoute() {
  const store = useAuthStore.getState()

  if (store.status === 'idle' || (store.accessToken && !store.user)) {
    await store.restoreSession()
  }

  return useAuthStore.getState()
}

function requireAuth(requirement?: PermissionRequirement) {
  return async () => {
    const store = await restoreAuthForRoute()

    if (store.status === 'anonymous') {
      throw redirect({ to: '/login' })
    }

    if (store.status === 'error') {
      return
    }

    if (!canAccess(store.user, requirement)) {
      throw redirect({ to: '/forbidden' })
    }
  }
}

async function redirectToReportHome() {
  const store = await restoreAuthForRoute()

  if (canAccess(store.user, reportWriteAccess)) {
    throw redirect({ to: '/reports/generate' })
  }

  if (canAccess(store.user, reportAccess)) {
    throw redirect({ to: '/reports/records' })
  }

  throw redirect({ to: '/forbidden' })
}

async function redirectToAppHome() {
  const store = await restoreAuthForRoute()

  if (canAccess(store.user, qaAccess)) {
    throw redirect({ to: '/chat' })
  }

  if (canAccess(store.user, reportWriteAccess)) {
    throw redirect({ to: '/reports/generate' })
  }

  if (canAccess(store.user, reportAccess)) {
    throw redirect({ to: '/reports/records' })
  }

  if (canAccess(store.user, adminAccess)) {
    throw redirect({ to: '/admin' })
  }

  throw redirect({ to: '/forbidden' })
}

async function redirectToAdminHome() {
  const store = await restoreAuthForRoute()

  if (canAccess(store.user, systemAdminAccess)) {
    throw redirect({ to: '/admin/stats' })
  }

  if (canAccess(store.user, reportAccess)) {
    throw redirect({ to: '/admin/reports/records' })
  }

  if (canAccess(store.user, knowledgeWriteAccess)) {
    throw redirect({ to: '/admin/knowledge' })
  }

  if (canAccess(store.user, knowledgeAccess)) {
    throw redirect({ to: '/admin/knowledge-config' })
  }

  if (canAccess(store.user, qaAdminAccess)) {
    throw redirect({ to: '/admin/prompts' })
  }

  throw redirect({ to: '/forbidden' })
}

const qaAccess: PermissionRequirement = { any: ['qa:use'] }
const qaAdminAccess: PermissionRequirement = {
  any: ['admin:model-profile:write', 'admin:parser-config:write', 'system:admin'],
}
const reportAccess: PermissionRequirement = {
  any: ['report:read', 'report:write', 'reports:write'],
}
const reportWriteAccess: PermissionRequirement = { any: ['report:write', 'reports:write'] }
const knowledgeAccess: PermissionRequirement = {
  any: ['knowledge:read', 'knowledge:write', 'document:upload'],
}
const knowledgeWriteAccess: PermissionRequirement = { any: ['knowledge:write'] }
const systemAdminAccess: PermissionRequirement = { any: ['system:admin'] }
const adminAccess: PermissionRequirement = {
  any: [
    'system:admin',
    'report:read',
    'report:write',
    'reports:write',
    'knowledge:read',
    'knowledge:write',
    'document:upload',
    'admin:model-profile:write',
    'admin:parser-config:write',
  ],
}

const rootRoute = createRootRoute({
  component: Outlet,
})

const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: 'login',
  beforeLoad: async () => {
    const store = await restoreAuthForRoute()
    if (store.status === 'authenticated') {
      await redirectToAppHome()
    }
  },
  component: LoginPage,
})

const authenticatedRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: 'authenticated',
  beforeLoad: requireAuth(),
  component: () => (
    <AppLayout>
      <Outlet />
    </AppLayout>
  ),
})

const indexRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: '/',
  beforeLoad: redirectToAppHome,
})

const forbiddenRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'forbidden',
  component: ForbiddenPage,
})

const chatRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'chat',
  beforeLoad: requireAuth(qaAccess),
  component: ChatPage,
})

const reportsRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'reports',
  beforeLoad: requireAuth(reportAccess),
  component: Outlet,
})

const reportsIndexRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: '/',
  beforeLoad: redirectToReportHome,
})

const reportGenerateRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'generate',
  beforeLoad: requireAuth(reportWriteAccess),
  component: ReportGeneratePage,
})

const reportRecordsRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'records',
  component: ReportRecordsPage,
})

const reportTemplatesRoute = createRoute({
  getParentRoute: () => reportsRoute,
  path: 'templates',
  beforeLoad: requireAuth(reportWriteAccess),
  component: ReportTemplatesPage,
})

const adminRoute = createRoute({
  getParentRoute: () => authenticatedRoute,
  path: 'admin',
  beforeLoad: requireAuth(adminAccess),
  component: AdminPage,
})

const adminIndexRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: '/',
  beforeLoad: redirectToAdminHome,
})

const adminUsersRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'users',
  beforeLoad: requireAuth(systemAdminAccess),
  component: UserManagement,
})

const adminRolesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'roles',
  beforeLoad: requireAuth(systemAdminAccess),
  component: RoleManagement,
})

const adminStylesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'styles',
  beforeLoad: requireAuth(systemAdminAccess),
  component: StyleManagement,
})

const adminReportCategoriesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'report-categories',
  beforeLoad: requireAuth(systemAdminAccess),
  component: ReportCategory,
})

const adminFilesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'files',
  beforeLoad: requireAuth(systemAdminAccess),
  component: FileManagement,
})

const adminTemplatesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'templates',
  beforeLoad: requireAuth(reportWriteAccess),
  component: TemplateManagement,
})

const adminMaterialsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'materials',
  beforeLoad: requireAuth(reportWriteAccess),
  component: MaterialManagement,
})

const adminPromptsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'prompts',
  beforeLoad: requireAuth(qaAdminAccess),
  component: PromptManagement,
})

const adminKnowledgeRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge',
  beforeLoad: requireAuth(knowledgeWriteAccess),
  component: KnowledgeManagement,
})

const adminKnowledgeExperienceRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge-experience',
  beforeLoad: requireAuth(knowledgeAccess),
  component: KnowledgeExperience,
})

const adminKnowledgeConfigRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'knowledge-config',
  beforeLoad: requireAuth(knowledgeAccess),
  component: KnowledgeConfig,
})

const adminQASettingsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'qa-settings',
  beforeLoad: requireAuth(qaAdminAccess),
  component: QASettings,
})

const adminQARetrievalTestRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'qa-retrieval-test',
  beforeLoad: requireAuth(qaAdminAccess),
  component: QARetrievalTestPage,
})

const adminSettingsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'settings',
  beforeLoad: requireAuth(systemAdminAccess),
  component: SystemSettings,
})

const adminStatsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'stats',
  beforeLoad: requireAuth(systemAdminAccess),
  component: StatsOverviewPage,
})

const adminReportRecordsRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'reports/records',
  beforeLoad: requireAuth(reportAccess),
  component: ReportRecordsPage,
})

const adminReportTemplatesRoute = createRoute({
  getParentRoute: () => adminRoute,
  path: 'reports/templates',
  beforeLoad: requireAuth(reportWriteAccess),
  component: ReportTemplatesPage,
})

const routeTree = rootRoute.addChildren([
  loginRoute,
  authenticatedRoute.addChildren([
    indexRoute,
    forbiddenRoute,
    chatRoute,
    reportsRoute.addChildren([
      reportsIndexRoute,
      reportGenerateRoute,
      reportRecordsRoute,
      reportTemplatesRoute,
    ]),
    adminRoute.addChildren([
      adminIndexRoute,
      adminUsersRoute,
      adminRolesRoute,
      adminStylesRoute,
      adminReportCategoriesRoute,
      adminFilesRoute,
      adminTemplatesRoute,
      adminMaterialsRoute,
      adminPromptsRoute,
      adminKnowledgeRoute,
      adminKnowledgeExperienceRoute,
      adminKnowledgeConfigRoute,
      adminQASettingsRoute,
      adminQARetrievalTestRoute,
      adminSettingsRoute,
      adminStatsRoute,
      adminReportRecordsRoute,
      adminReportTemplatesRoute,
    ]),
  ]),
])

export const router = createRouter({ routeTree })

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
