import { describe, expect, it, vi } from 'vitest'
import { ApiError, createApiClient, resolveApiBaseUrl, resolveDevelopmentUserId } from './client'

describe('API client', () => {
  it('resolves the API under a configured deployment base path', () => {
    expect(resolveApiBaseUrl('/bizdevops/')).toBe('/bizdevops/api/v1')
    expect(resolveApiBaseUrl('/')).toBe('/api/v1')
  })

  it('keeps an explicitly configured trial identity in production builds', () => {
    expect(resolveDevelopmentUserId(' 10000000-0000-4000-8000-000000000001 '))
      .toBe('10000000-0000-4000-8000-000000000001')
    expect(resolveDevelopmentUserId('')).toBeUndefined()
    expect(resolveDevelopmentUserId(undefined)).toBeUndefined()
  })
  it('sends JSON and unwraps the API envelope', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ data: { id: 'sys-1', name: '订单中心' } }),
      { status: 200, headers: { 'Content-Type': 'application/json' } },
    ))
    const client = createApiClient({ baseUrl: '/api/v1', transport: fetchMock })

    await expect(client.getSystem('sys-1')).resolves.toMatchObject({ id: 'sys-1' })
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/systems/sys-1', expect.objectContaining({
      headers: expect.objectContaining({ Accept: 'application/json' }),
    }))
  })

  it('sends an explicit development identity only when configured', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    const client = createApiClient({ developmentUserId: 'dev-user', transport })

    await client.listSystems()

    expect(transport).toHaveBeenCalledWith('/api/v1/systems', expect.objectContaining({
      headers: expect.objectContaining({ 'X-Dev-User-ID': 'dev-user' }),
    }))
  })

  it('raises a typed error with server request id', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(
      JSON.stringify({ error: { code: 'SYSTEM_NOT_FOUND', message: '不存在', requestId: 'req-7' } }),
      { status: 404, headers: { 'Content-Type': 'application/json' } },
    ))
    const client = createApiClient({ baseUrl: '/api/v1', transport })

    const error = await client.getSystem('missing').catch((reason: unknown) => reason)
    expect(error).toBeInstanceOf(ApiError)
    expect(error).toMatchObject({ status: 404, code: 'SYSTEM_NOT_FOUND', requestId: 'req-7' })
  })

  it('lists only the authorized systems returned by the API', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      data: [
        {
          id: '11111111-1111-1111-1111-111111111111',
          code: 'order-center',
          name: '订单中心',
          status: 'active',
          myRole: 'maintainer',
          createdAt: '2026-07-11T10:00:00Z',
          updatedAt: '2026-07-11T10:00:00Z',
        },
      ],
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })

    await expect(client.listSystems()).resolves.toEqual([
      expect.objectContaining({ code: 'order-center', myRole: 'maintainer' }),
    ])
    expect(transport).toHaveBeenCalledWith('/api/v1/systems', expect.objectContaining({
      headers: expect.objectContaining({ Accept: 'application/json' }),
    }))
  })

  it('loads all workspace resources from system-scoped endpoints', async () => {
    const transport = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ data: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })

    await Promise.all([
      client.listScans('system / A'),
      client.listApiOperations('system / A'),
      client.listScenarioImports('system / A'),
    ])

    expect(transport.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/systems/system%20%2F%20A/scans',
      '/api/v1/systems/system%20%2F%20A/api-operations',
      '/api/v1/systems/system%20%2F%20A/scenario-imports',
    ])
  })

  it('sends workflow mutations to their scoped endpoints', async () => {
    const transport = vi.fn().mockImplementation(async (url: string) => new Response(JSON.stringify({
      data: url.endsWith('/run') ? { scanId: 'scan-1', status: 'succeeded' } : { id: 'result-1' },
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })

    await client.createScan('system-a', { codeSourceId: 'source-1', sourceRef: 'main', sourceCommit: 'abc', language: 'go', framework: 'gin' })
    await client.runScan('system-a', 'scan-1', { repositoryRoot: 'D:/repo' })
    await client.uploadScenarioImport('system-a', { fileName: 'orders.json', document: { schemaVersion: '1.0' } })
    await client.confirmScenarioImportScripts('system-a', 'import-1')
    await client.applyScenarioImport('system-a', 'import-1')

    expect(transport.mock.calls.map(([url, init]) => [url, init.method, init.body])).toEqual([
      ['/api/v1/systems/system-a/scans', 'POST', JSON.stringify({ codeSourceId: 'source-1', sourceRef: 'main', sourceCommit: 'abc', language: 'go', framework: 'gin' })],
      ['/api/v1/systems/system-a/scans/scan-1/run', 'POST', JSON.stringify({ repositoryRoot: 'D:/repo' })],
      ['/api/v1/systems/system-a/scenario-imports', 'POST', JSON.stringify({ fileName: 'orders.json', document: { schemaVersion: '1.0' } })],
      ['/api/v1/systems/system-a/scenario-imports/import-1/confirm-scripts', 'POST', undefined],
      ['/api/v1/systems/system-a/scenario-imports/import-1/apply', 'POST', undefined],
    ])
  })

  it('uses system-scoped discovery and run endpoints', async () => {
    const transport = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })
    const discovery = { type: 'prompt' as const, name: '下单', prompt: '用户提交订单' }
    const run = { scenarioId: 'scenario-1', scenarioVersionId: 'version-1', environmentId: 'env-1', stopAfterStepId: 'step-2', inputVariables: { orderId: 'A1' } }

    await client.listDiscoveries('system-a')
    await client.createDiscovery('system-a', discovery)
    await client.listDiscoveryCandidates('system-a', 'discovery-1')
    await client.reviewCandidate('system-a', 'discovery-1', 'candidate-1', 'accept', { note: 'P0 主链路' })
    await client.promoteCandidate('system-a', 'discovery-1', 'candidate-1')
    await client.listScenarios('system-a')
    await client.getScenario('system-a', 'scenario-1')
    await client.createScenarioRun('system-a', run)
    await client.listScenarioRuns('system-a')
    await client.getScenarioRun('system-a', 'run-1')
    await client.retryScenarioStep('system-a', 'run-1', 'step-2')

    expect(transport.mock.calls.map(([url, init]) => [url, init.method ?? 'GET', init.body])).toEqual([
      ['/api/v1/systems/system-a/discoveries', 'GET', undefined],
      ['/api/v1/systems/system-a/discoveries', 'POST', JSON.stringify(discovery)],
      ['/api/v1/systems/system-a/discoveries/discovery-1/candidates', 'GET', undefined],
      ['/api/v1/systems/system-a/discoveries/discovery-1/candidates/candidate-1/accept', 'POST', JSON.stringify({ note: 'P0 主链路' })],
      ['/api/v1/systems/system-a/discoveries/discovery-1/candidates/candidate-1/promote', 'POST', '{}'],
      ['/api/v1/systems/system-a/scenarios', 'GET', undefined],
      ['/api/v1/systems/system-a/scenarios/scenario-1', 'GET', undefined],
      ['/api/v1/systems/system-a/scenario-runs', 'POST', JSON.stringify(run)],
      ['/api/v1/systems/system-a/scenario-runs', 'GET', undefined],
      ['/api/v1/systems/system-a/scenario-runs/run-1', 'GET', undefined],
      ['/api/v1/systems/system-a/scenario-runs/run-1/steps/step-2/retry', 'POST', undefined],
    ])
  })

  it('reads backend-authorized management overview projections', async () => {
    const transport = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ data: {} }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })
    await client.getManagementOverview()
    await client.getManagementSystemOverview('system / A')
    expect(transport.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/management/overview',
      '/api/v1/management/systems/system%20%2F%20A/overview',
    ])
  })
})
