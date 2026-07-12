import { Navigate, type RouteObject } from 'react-router-dom'
import { GlobalLayout } from './layouts/GlobalLayout'
import { SystemLayout } from './layouts/SystemLayout'
import { DashboardPage, SystemsPage, GlobalRunDetailPage, GlobalRunsPage, GlobalSettingsPage } from './pages/global'
import {
  ApiAssetsPage,
  EditorPage,
  ScanPage,
  SystemOverviewPage,
  SystemSettingsPage,
} from './pages/system'
import { DiscoveryPage, ReviewPage } from './pages/discovery'
import { RunDetailPage, RunsPage } from './pages/runs'
import { NotFoundPage } from './pages/NotFoundPage'

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <GlobalLayout />,
    children: [
      { index: true, element: <Navigate to="/dashboard" replace /> },
      { path: 'dashboard', element: <DashboardPage /> },
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
]
