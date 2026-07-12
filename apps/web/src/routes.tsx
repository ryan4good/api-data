import { Navigate, type RouteObject } from 'react-router-dom'
import { GlobalLayout } from './layouts/GlobalLayout'
import { SystemLayout } from './layouts/SystemLayout'
import { SystemsPage, GlobalRunDetailPage, GlobalRunsPage, GlobalSettingsPage } from './pages/global'
import { ManagementDashboardPage } from './pages/management'
import { EditorPage, SystemOverviewPage } from './pages/system'
import { ScanPage } from './pages/scan'
import { ApiAssetsPage } from './pages/api-assets'
import { SystemSettingsPage } from './pages/system-settings'
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
          { path: 'discovery', element: <DiscoveryPage /> },
          { path: 'review', element: <ReviewPage /> },
          { path: 'editor', element: <EditorPage /> },
          { path: 'editor/:scenarioId', element: <EditorPage /> },
          { path: 'runs', element: <RunsPage /> },
          { path: 'runs/:runId', element: <RunDetailPage /> },
          { path: 'settings', element: <SystemSettingsPage /> },
        ],
      },
      { path: '*', element: <NotFoundPage /> },
    ],
  },
]
