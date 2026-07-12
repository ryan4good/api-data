import { describe, expect, it, vi } from 'vitest'
import { ApiError, createApiClient, resolveApiBaseUrl } from './client'

describe('API client', () => {
  it('resolves the API under a configured deployment base path', () => {
    expect(resolveApiBaseUrl('/bizdevops/')).toBe('/bizdevops/api/v1')
    expect(resolveApiBaseUrl('/')).toBe('/api/v1')
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

  it('uses same-origin credentials without exposing a bearer token to browser code', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: [] }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    const client = createApiClient({ transport })

    await client.listSystems()

    expect(transport).toHaveBeenCalledWith('/api/v1/systems', expect.objectContaining({
      credentials: 'same-origin',
      headers: expect.not.objectContaining({ Authorization: expect.anything() }),
    }))
  })

  it('logs in without an existing credential and exposes the current-user endpoint', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {
      accessToken: 'new-token',
      tokenType: 'Bearer',
      expiresIn: 3600,
      user: { id: 'user-1', email: 'admin@example.com', displayName: '管理员', platformRole: 'admin' },
    } }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ transport })

    await client.login({ email: 'admin@example.com', password: 'secret' })

    expect(transport).toHaveBeenCalledWith('/api/v1/auth/login', expect.objectContaining({
      method: 'POST',
      body: JSON.stringify({ email: 'admin@example.com', password: 'secret' }),
      credentials: 'same-origin',
    }))
  })

  it('notifies the session boundary when an authenticated request returns 401', async () => {
    const onUnauthorized = vi.fn()
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: { code: 'UNAUTHORIZED', message: '登录已过期' },
    }), { status: 401, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ onUnauthorized, transport })

    await expect(client.getCurrentUser()).rejects.toMatchObject({ status: 401 })
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('ends the server session through the logout endpoint', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: null }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    const client = createApiClient({ transport })

    await client.logout()

    expect(transport).toHaveBeenCalledWith('/api/v1/auth/logout', expect.objectContaining({
      method: 'POST',
      credentials: 'same-origin',
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
    await client.runScan('system-a', 'scan-1')
    await client.uploadScenarioImport('system-a', { fileName: 'orders.json', document: { schemaVersion: '1.0' } })
    await client.confirmScenarioImportScripts('system-a', 'import-1')
    await client.applyScenarioImport('system-a', 'import-1')

    expect(transport.mock.calls.map(([url, init]) => [url, init.method, init.body])).toEqual([
      ['/api/v1/systems/system-a/scans', 'POST', JSON.stringify({ codeSourceId: 'source-1', sourceRef: 'main', sourceCommit: 'abc', language: 'go', framework: 'gin' })],
      ['/api/v1/systems/system-a/scans/scan-1/run', 'POST', '{}'],
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

  it('saves a scenario revision with PUT on the scoped scenario endpoint', async () => {
    const transport = vi.fn().mockResolvedValue(new Response(JSON.stringify({ data: {} }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })
    const revision = {
      name: '创建订单', description: '主链路', status: 'active' as const,
      steps: [{ key: 'create', name: '创建订单', type: 'http' as const, dependsOn: [], requestConfig: { method: 'POST', path: '/orders' } }],
    }

    await client.updateScenario('system / A', 'scenario / 1', revision)

    expect(transport).toHaveBeenCalledWith('/api/v1/systems/system%20%2F%20A/scenarios/scenario%20%2F%201', expect.objectContaining({
      method: 'PUT', body: JSON.stringify(revision),
    }))
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

  it('uses system-scoped environment and external secret-reference endpoints', async () => {
    const transport = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })
    const environment = { key: 'staging', name: '预发布', status: 'active' as const, variables: { BASE_URL: 'https://staging.example.test' } }
    const reference = { variableKey: 'DB_PASSWORD', secretRef: 'vault://bizdevops/staging/db-password' }

    await client.listEnvironments('system / A')
    await client.upsertEnvironment('system / A', environment)
    await client.upsertEnvironment('system / A', { ...environment, id: 'env / 1', status: 'disabled' })
    await client.listSecretReferences('system / A', 'env / 1')
    await client.createSecretReference('system / A', 'env / 1', reference)

    expect(transport.mock.calls.map(([url, init]) => [url, init.method ?? 'GET', init.body])).toEqual([
      ['/api/v1/systems/system%20%2F%20A/environments', 'GET', undefined],
      ['/api/v1/systems/system%20%2F%20A/environments', 'POST', JSON.stringify(environment)],
      ['/api/v1/systems/system%20%2F%20A/environments', 'POST', JSON.stringify({ ...environment, id: 'env / 1', status: 'disabled' })],
      ['/api/v1/systems/system%20%2F%20A/environments/env%20%2F%201/secret-references', 'GET', undefined],
      ['/api/v1/systems/system%20%2F%20A/environments/env%20%2F%201/secret-references', 'POST', JSON.stringify(reference)],
    ])
  })

  it('uses the owner-only system member endpoints', async () => {
    const transport = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })

    await client.listSystemMembers('system / A')
    await client.upsertSystemMember('system / A', { userId: 'user / 1', role: 'reviewer' })

    expect(transport.mock.calls.map(([url, init]) => [url, init.method ?? 'GET', init.body])).toEqual([
      ['/api/v1/systems/system%20%2F%20A/members', 'GET', undefined],
      ['/api/v1/systems/system%20%2F%20A/members', 'POST', JSON.stringify({ userId: 'user / 1', role: 'reviewer' })],
    ])
  })

  it('uses system-scoped code source endpoints with distinct create and update methods', async () => {
    const transport = vi.fn().mockImplementation(async () => new Response(JSON.stringify({ data: [] }), { status: 200, headers: { 'Content-Type': 'application/json' } }))
    const client = createApiClient({ baseUrl: '/api/v1', transport })
    const source = {
      name: '订单服务', sourceType: 'git' as const, repositoryUrl: 'https://git.example.test/orders.git',
      defaultRef: 'main', includePaths: ['src'], excludePaths: ['vendor'], credentialRef: 'vault://git/orders', status: 'active' as const,
    }

    await client.listCodeSources('system / A')
    await client.createCodeSource('system / A', source)
    await client.updateCodeSource('system / A', 'source / 1', { ...source, status: 'disabled' })

    expect(transport.mock.calls.map(([url, init]) => [url, init.method ?? 'GET', init.body])).toEqual([
      ['/api/v1/systems/system%20%2F%20A/code-sources', 'GET', undefined],
      ['/api/v1/systems/system%20%2F%20A/code-sources', 'POST', JSON.stringify(source)],
      ['/api/v1/systems/system%20%2F%20A/code-sources/source%20%2F%201', 'PUT', JSON.stringify({ ...source, status: 'disabled' })],
    ])
  })
})
