import type { ApiEnvelope, ApiErrorEnvelope, ApiOperation, AuthUser, BusinessSystem, CodeSource, CreateDiscoveryInput, CreateDiscoveryResult, CreateScanInput, CreateScenarioRunInput, CreateSecretReferenceInput, DiscoveryCandidate, DiscoveryRecord, Environment, LoginInput, LoginResult, ManagementOverview, ManagementSystemOverview, PromoteCandidateResult, RetryStepResult, ReviewCandidateInput, RunScanResult, ScanRun, ScenarioDetail, ScenarioImport, ScenarioRunDetail, ScenarioSummary, SecretReference, SystemMember, UpdateScenarioInput, UploadScenarioImportInput, UpsertCodeSourceInput, UpsertEnvironmentInput, UpsertSystemMemberInput } from './types'
import { clearAuthSession } from '../auth/session'

export class ApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly code: string,
    public readonly requestId?: string,
    public readonly details?: unknown,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

export interface ApiClientOptions {
  baseUrl?: string
  onUnauthorized?: () => void
  transport?: typeof fetch
}

export function resolveApiBaseUrl(basePath: string): string {
  const normalized = `/${basePath}`.replace(/\/+/g, '/').replace(/\/+$/, '')
  return `${normalized === '' ? '' : normalized}/api/v1`
}

export function createApiClient(options: ApiClientOptions = {}) {
  const baseUrl = (options.baseUrl ?? resolveApiBaseUrl(import.meta.env.BASE_URL)).replace(/\/$/, '')
  const transport = options.transport ?? fetch

  async function request<T>(path: string, init: RequestInit = {}, authenticate = true): Promise<T> {
    const response = await transport(`${baseUrl}${path}`, {
      ...init,
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...init.headers,
      },
    })
    const payload = await response.json() as ApiEnvelope<T> | ApiErrorEnvelope
    if (!response.ok) {
      if (response.status === 401 && authenticate) options.onUnauthorized?.()
      const failure = (payload as ApiErrorEnvelope).error
      throw new ApiError(
        failure?.message ?? `Request failed with status ${response.status}`,
        response.status,
        failure?.code ?? 'UNKNOWN_ERROR',
        failure?.requestId,
        failure?.details,
      )
    }
    return (payload as ApiEnvelope<T>).data
  }

  return {
    login: (input: LoginInput) => request<LoginResult>('/auth/login', { method: 'POST', body: JSON.stringify(input) }, false),
    getCurrentUser: () => request<AuthUser>('/auth/me'),
    logout: () => request<null>('/auth/logout', { method: 'POST' }, false),
    listSystems: () => request<BusinessSystem[]>('/systems'),
    getSystem: (systemId: string) => request<BusinessSystem>(`/systems/${encodeURIComponent(systemId)}`),
    listSystemMembers: (systemId: string) => request<SystemMember[]>(`/systems/${encodeURIComponent(systemId)}/members`),
    upsertSystemMember: (systemId: string, input: UpsertSystemMemberInput) => request<SystemMember>(`/systems/${encodeURIComponent(systemId)}/members`, { method: 'POST', body: JSON.stringify(input) }),
    listCodeSources: (systemId: string) => request<CodeSource[]>(`/systems/${encodeURIComponent(systemId)}/code-sources`),
    createCodeSource: (systemId: string, input: UpsertCodeSourceInput) => request<CodeSource>(`/systems/${encodeURIComponent(systemId)}/code-sources`, { method: 'POST', body: JSON.stringify(input) }),
    updateCodeSource: (systemId: string, sourceId: string, input: UpsertCodeSourceInput) => request<CodeSource>(`/systems/${encodeURIComponent(systemId)}/code-sources/${encodeURIComponent(sourceId)}`, { method: 'PUT', body: JSON.stringify(input) }),
    listEnvironments: (systemId: string) => request<Environment[]>(`/systems/${encodeURIComponent(systemId)}/environments`),
    upsertEnvironment: (systemId: string, input: UpsertEnvironmentInput) => request<Environment>(`/systems/${encodeURIComponent(systemId)}/environments`, { method: 'POST', body: JSON.stringify(input) }),
    listSecretReferences: (systemId: string, environmentId: string) => request<SecretReference[]>(`/systems/${encodeURIComponent(systemId)}/environments/${encodeURIComponent(environmentId)}/secret-references`),
    createSecretReference: (systemId: string, environmentId: string, input: CreateSecretReferenceInput) => request<SecretReference>(`/systems/${encodeURIComponent(systemId)}/environments/${encodeURIComponent(environmentId)}/secret-references`, { method: 'POST', body: JSON.stringify(input) }),
    listScans: (systemId: string) => request<ScanRun[]>(`/systems/${encodeURIComponent(systemId)}/scans`),
    listApiOperations: (systemId: string) => request<ApiOperation[]>(`/systems/${encodeURIComponent(systemId)}/api-operations`),
    listScenarioImports: (systemId: string) => request<ScenarioImport[]>(`/systems/${encodeURIComponent(systemId)}/scenario-imports`),
    createScan: (systemId: string, input: CreateScanInput) => request<ScanRun>(`/systems/${encodeURIComponent(systemId)}/scans`, { method: 'POST', body: JSON.stringify(input) }),
    runScan: (systemId: string, scanId: string) => request<RunScanResult>(`/systems/${encodeURIComponent(systemId)}/scans/${encodeURIComponent(scanId)}/run`, { method: 'POST', body: '{}' }),
    uploadScenarioImport: (systemId: string, input: UploadScenarioImportInput) => request<ScenarioImport>(`/systems/${encodeURIComponent(systemId)}/scenario-imports`, { method: 'POST', body: JSON.stringify(input) }),
    confirmScenarioImportScripts: (systemId: string, importId: string) => request<ScenarioImport>(`/systems/${encodeURIComponent(systemId)}/scenario-imports/${encodeURIComponent(importId)}/confirm-scripts`, { method: 'POST' }),
    applyScenarioImport: (systemId: string, importId: string) => request<ScenarioImport>(`/systems/${encodeURIComponent(systemId)}/scenario-imports/${encodeURIComponent(importId)}/apply`, { method: 'POST' }),
    listDiscoveries: (systemId: string) => request<DiscoveryRecord[]>(`/systems/${encodeURIComponent(systemId)}/discoveries`),
    createDiscovery: (systemId: string, input: CreateDiscoveryInput) => request<CreateDiscoveryResult>(`/systems/${encodeURIComponent(systemId)}/discoveries`, { method: 'POST', body: JSON.stringify(input) }),
    listDiscoveryCandidates: (systemId: string, discoveryId: string) => request<DiscoveryCandidate[]>(`/systems/${encodeURIComponent(systemId)}/discoveries/${encodeURIComponent(discoveryId)}/candidates`),
    reviewCandidate: (systemId: string, discoveryId: string, candidateId: string, decision: 'accept' | 'reject', input: ReviewCandidateInput) => request<DiscoveryCandidate>(`/systems/${encodeURIComponent(systemId)}/discoveries/${encodeURIComponent(discoveryId)}/candidates/${encodeURIComponent(candidateId)}/${decision}`, { method: 'POST', body: JSON.stringify(input) }),
    promoteCandidate: (systemId: string, discoveryId: string, candidateId: string) => request<PromoteCandidateResult>(`/systems/${encodeURIComponent(systemId)}/discoveries/${encodeURIComponent(discoveryId)}/candidates/${encodeURIComponent(candidateId)}/promote`, { method: 'POST', body: '{}' }),
    listScenarios: (systemId: string) => request<ScenarioSummary[]>(`/systems/${encodeURIComponent(systemId)}/scenarios`),
    getScenario: (systemId: string, scenarioId: string) => request<ScenarioDetail>(`/systems/${encodeURIComponent(systemId)}/scenarios/${encodeURIComponent(scenarioId)}`),
    updateScenario: (systemId: string, scenarioId: string, input: UpdateScenarioInput) => request<ScenarioDetail>(`/systems/${encodeURIComponent(systemId)}/scenarios/${encodeURIComponent(scenarioId)}`, { method: 'PUT', body: JSON.stringify(input) }),
    createScenarioRun: (systemId: string, input: CreateScenarioRunInput) => request<ScenarioRunDetail>(`/systems/${encodeURIComponent(systemId)}/scenario-runs`, { method: 'POST', body: JSON.stringify(input) }),
    listScenarioRuns: (systemId: string) => request<ScenarioRunDetail[]>(`/systems/${encodeURIComponent(systemId)}/scenario-runs`),
    getScenarioRun: (systemId: string, runId: string) => request<ScenarioRunDetail>(`/systems/${encodeURIComponent(systemId)}/scenario-runs/${encodeURIComponent(runId)}`),
    retryScenarioStep: (systemId: string, runId: string, stepId: string) => request<RetryStepResult>(`/systems/${encodeURIComponent(systemId)}/scenario-runs/${encodeURIComponent(runId)}/steps/${encodeURIComponent(stepId)}/retry`, { method: 'POST' }),
    getManagementOverview: () => request<ManagementOverview>('/management/overview'),
    getManagementSystemOverview: (systemId: string) => request<ManagementSystemOverview>(`/management/systems/${encodeURIComponent(systemId)}/overview`),
    request,
  }
}

export type ApiClient = ReturnType<typeof createApiClient>

function redirectToLogin(): void {
  if (typeof window === 'undefined') return
  const basePath = import.meta.env.BASE_URL.endsWith('/') ? import.meta.env.BASE_URL : `${import.meta.env.BASE_URL}/`
  const loginPath = `${basePath}login`.replace(/\/+/g, '/')
  if (window.location.pathname !== loginPath) window.location.replace(loginPath)
}

export const apiClient = createApiClient({
  onUnauthorized: () => {
    clearAuthSession()
    redirectToLogin()
  },
})
