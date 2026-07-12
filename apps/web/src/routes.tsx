import { Navigate, type RouteObject } from 'react-router-dom'
import { GlobalLayout } from './layouts/GlobalLayout'
import { SystemLayout } from './layouts/SystemLayout'
import { SystemsPage, GlobalSettingsPage } from './pages/global'
import { GlobalRunDetailPage, GlobalRunsPage } from './pages/global-runs'
import { ManagementDashboardPage } from './pages/management'
import { SystemOverviewPage } from './pages/system'
import { ScenarioEditorPage } from './pages/scenario-editor'
import { ScanPage } from './pages/scan'
import { ApiAssetsPage } from './pages/api-assets'
import { CodeSourceSettingsPage } from './pages/code-source-settings'
import { EnvironmentSettingsPage, SystemMembersPage, SystemSettingsPage } from './pages/system-settings'
import { DiscoveryPage, ReviewPage } from './pages/discovery'
import { RunDetailPage, RunsPage } from './pages/runs'
import { NotFoundPage } from './pages/NotFoundPage'
import { AuthenticatedRoute } from './auth/AuthenticatedRoute'
import { LoginPage } from './pages/login'

export const routes: RouteObject[] = [
  {
    id: 'login',
    path: '/login',
    element: <LoginPage />,
  },
  {
    id: 'authenticated',
    element: <AuthenticatedRoute />,
    children: [
      {
        path: '/',
        element: <GlobalLayout />,
        children: [
          { index: true, element: <Navigate to="/dashboard" replace /> },
          { path: 'dashboard', element: <ManagementDashboardPage /> },
          { path: 'systems', element: <SystemsPage /> },
          { path: 'runs', element: <GlobalRunsPage /> },
          { path: 'runs/:runId', element: <GlobalRunDetailPage /> },
          { path: 'settings', element: <GlobalSettingsPage /> },
        ],
      },
      {
        path: '/systems/:systemId',
        element: <SystemLayout />,
        children: [
          { index: true, element: <Navigate to="overview" replace /> },
          { path: 'overview', element: <SystemOverviewPage /> },
          { path: 'scan', element: <ScanPage /> },
          { path: 'apis', element: <ApiAssetsPage /> },
          { path: 'code-sources', element: <CodeSourceSettingsPage /> },
          { path: 'discovery', element: <DiscoveryPage /> },
          { path: 'review', element: <ReviewPage /> },
          { path: 'editor', element: <ScenarioEditorPage /> },
          { path: 'editor/:scenarioId', element: <ScenarioEditorPage /> },
          { path: 'runs', element: <RunsPage /> },
          { path: 'runs/:runId', element: <RunDetailPage /> },
          { path: 'settings', element: <SystemSettingsPage /> },
          { path: 'settings/environments', element: <EnvironmentSettingsPage /> },
          { path: 'settings/members', element: <SystemMembersPage /> },
        ],
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]
