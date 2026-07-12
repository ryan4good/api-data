import type { ApiEnvelope, ApiErrorEnvelope, ApiOperation, BusinessSystem, CreateDiscoveryInput, CreateDiscoveryResult, CreateScanInput, CreateScenarioRunInput, DiscoveryCandidate, DiscoveryRecord, ManagementOverview, ManagementSystemOverview, PromoteCandidateResult, RetryStepResult, ReviewCandidateInput, RunScanInput, RunScanResult, ScanRun, ScenarioDetail, ScenarioImport, ScenarioRunDetail, ScenarioSummary, UploadScenarioImportInput } from './types'

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
  getAccessToken?: () => string | undefined
  developmentUserId?: string
  transport?: typeof fetch
}

export function createApiClient(options: ApiClientOptions = {}) {
  const baseUrl = (options.baseUrl ?? '/api/v1').replace(/\/$/, '')
  const transport = options.transport ?? fetch

  async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
    const token = options.getAccessToken?.()
    const response = await transport(`${baseUrl}${path}`, {
      ...init,
      headers: {
        Accept: 'application/json',
        ...(init.body ? { 'Content-Type': 'application/json' } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(options.developmentUserId ? { 'X-Dev-User-ID': options.developmentUserId } : {}),
        ...init.headers,
      },
    })
    const payload = await response.json() as ApiEnvelope<T> | ApiErrorEnvelope
    if (!response.ok) {
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
    listSystems: () => request<BusinessSystem[]>('/systems'),
    getSystem: (systemId: string) => request<BusinessSystem>(`/systems/${encodeURIComponent(systemId)}`),
    listScans: (systemId: string) => request<ScanRun[]>(`/systems/${encodeURIComponent(systemId)}/scans`),
    listApiOperations: (systemId: string) => request<ApiOperation[]>(`/systems/${encodeURIComponent(systemId)}/api-operations`),
    listScenarioImports: (systemId: string) => request<ScenarioImport[]>(`/systems/${encodeURIComponent(systemId)}/scenario-imports`),
    createScan: (systemId: string, input: CreateScanInput) => request<ScanRun>(`/systems/${encodeURIComponent(systemId)}/scans`, { method: 'POST', body: JSON.stringify(input) }),
    runScan: (systemId: string, scanId: string, input: RunScanInput) => request<RunScanResult>(`/systems/${encodeURIComponent(systemId)}/scans/${encodeURIComponent(scanId)}/run`, { method: 'POST', body: JSON.stringify(input) }),
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
export const apiClient = createApiClient({
  developmentUserId: import.meta.env.DEV ? import.meta.env.VITE_DEV_USER_ID : undefined,
})
