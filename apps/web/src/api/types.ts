export type Identifier = string
export type IsoDateTime = string
export type SystemRole = 'owner' | 'maintainer' | 'reviewer' | 'runner' | 'viewer'

export interface BusinessSystem {
  id: Identifier
  code: string
  name: string
  description?: string
  status: 'active' | 'archived'
  myRole: SystemRole
  createdAt: IsoDateTime
  updatedAt: IsoDateTime
}

export interface ApiOperation {
  id: Identifier
  systemId: Identifier
  scanRunId?: Identifier
  operationKey?: string
  method: string
  path: string
  summary?: string
  source?: { repository: string; file: string; line?: number }
  verificationStatus?: string
  lifecycleStatus?: string
  contentHash?: string
  createdAt?: IsoDateTime
  updatedAt?: IsoDateTime
}

export interface ScanRun {
  id: Identifier
  systemId: Identifier
  codeSourceId: Identifier
  sourceRef?: string
  sourceCommit?: string
  language?: string
  framework?: string
  status: 'queued' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  errorMessage?: string
  startedAt?: IsoDateTime
  finishedAt?: IsoDateTime
  createdAt: IsoDateTime
}

export interface CreateScanInput {
  codeSourceId: Identifier
  sourceRef?: string
  sourceCommit?: string
  language?: string
  framework?: string
}

export interface RunScanInput {
  repositoryRoot: string
}

export interface RunScanResult {
  scanId: Identifier
  status: string
}

export interface ScenarioImport {
  id: Identifier
  systemId: Identifier
  format: string
  fileName: string
  status: 'uploaded' | 'validating' | 'ready' | 'failed' | 'applied'
  errorMessage?: string
  importedBy?: Identifier
  createdAt: IsoDateTime
  updatedAt?: IsoDateTime
}

export interface UploadScenarioImportInput {
  fileName: string
  document: unknown
}

export interface ScenarioSummary {
  id: Identifier
  systemId: Identifier
  key?: string
  name: string
  status: string
  version?: number
  currentVersionId?: Identifier
  description?: string
  createdBy?: Identifier
  createdAt?: IsoDateTime
  updatedAt?: IsoDateTime
}

export interface ScenarioVersion {
  id: Identifier
  systemId: Identifier
  scenarioId: Identifier
  versionNo: number
  sourceType: string
  createdBy: Identifier
  createdAt: IsoDateTime
}

export interface ScenarioStep {
  id: Identifier
  systemId: Identifier
  versionId: Identifier
  key: string
  name: string
  position: number
  type: string
  operationId?: Identifier
  dependsOn: string[]
  createdAt: IsoDateTime
}

export interface ScenarioDetail {
  scenario: ScenarioSummary
  version: ScenarioVersion
  steps: ScenarioStep[]
}

export interface PromoteCandidateResult extends ScenarioDetail { created: boolean }

export type DiscoveryType = 'code' | 'prd' | 'prompt' | 'mixed'

export interface DiscoveryRecord {
  id: Identifier
  systemId: Identifier
  codeSourceId?: Identifier
  type: DiscoveryType
  name: string
  status: 'queued' | 'running' | 'ready' | 'failed'
  errorMessage?: string
  requestedBy: Identifier
  startedAt?: IsoDateTime
  finishedAt?: IsoDateTime
  createdAt: IsoDateTime
  updatedAt: IsoDateTime
}

export interface DiscoveryCandidateStep {
  key: string
  name: string
  operationId?: Identifier
  method?: string
  path?: string
}

export interface DiscoveryCandidate {
  id: Identifier
  systemId: Identifier
  discoveryId: Identifier
  key: string
  name: string
  description?: string
  priority: string
  confidence: number
  sourceRefs: string[]
  requiresReview: boolean
  steps: DiscoveryCandidateStep[]
  reviewStatus: 'pending' | 'accepted' | 'rejected'
  reviewNote?: string
  reviewedBy?: Identifier
  reviewedAt?: IsoDateTime
  createdAt: IsoDateTime
  updatedAt: IsoDateTime
}

export interface CreateDiscoveryInput {
  codeSourceId?: Identifier
  type: DiscoveryType
  name: string
  prd?: string
  prompt?: string
  operations?: ApiOperation[]
}

export interface CreateDiscoveryResult {
  discovery: DiscoveryRecord
  candidates: DiscoveryCandidate[]
}

export interface ReviewCandidateInput { note: string }

export interface AssertionResult {
  id?: Identifier
  key: string
  type: string
  status: 'passed' | 'failed' | 'skipped' | 'error' | string
  expected?: unknown
  actual?: unknown
  message?: string
  durationMs?: number
}

export interface StepAttempt {
  id: Identifier
  systemId: Identifier
  runId: Identifier
  stepId: Identifier
  attemptNo: number
  position: number
  status: string
  requestSnapshot?: Record<string, unknown>
  responseSnapshot?: Record<string, unknown>
  extractedVariables?: Record<string, unknown>
  errorMessage?: string
  durationMs: number
  assertions: AssertionResult[]
  startedAt: IsoDateTime
  finishedAt: IsoDateTime
  createdAt: IsoDateTime
}

export interface ScenarioRunSummary {
  totalSteps: number
  executedSteps: number
  failedStepId?: Identifier
  stopAfterStepId?: Identifier
}

export interface ScenarioRunDetail {
  id: Identifier
  systemId: Identifier
  scenarioId: Identifier
  scenarioVersionId: Identifier
  environmentId: Identifier
  status: string
  outcome: string
  triggerType: string
  inputVariables?: Record<string, unknown>
  outputVariables?: Record<string, unknown>
  summary: ScenarioRunSummary
  requestedBy: Identifier
  startedAt: IsoDateTime
  finishedAt: IsoDateTime
  createdAt: IsoDateTime
  attempts: StepAttempt[]
}

export type ScenarioRun = ScenarioRunDetail

export interface CreateScenarioRunInput {
  scenarioId: Identifier
  scenarioVersionId: Identifier
  environmentId: Identifier
  stopAfterStepId?: Identifier
  inputVariables?: Record<string, unknown>
}

export interface RetryStepResult { run: ScenarioRunDetail; attempt: StepAttempt }

export interface ApiEnvelope<T> {
  data: T
  meta?: Record<string, unknown>
}

export interface ApiErrorEnvelope {
  error: { code: string; message: string; requestId?: string; details?: unknown }
}
