import { useEffect, useState, type FormEvent } from 'react'
import { useOutletContext } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { CreateSecretReferenceInput, Environment, SecretReference, SystemMember, SystemRole, UpsertEnvironmentInput, UpsertSystemMemberInput } from '../api/types'
import type { SystemContextState } from '../layouts/SystemLayout'
import { CodeSourceSettingsPage } from './code-source-settings'

export type EnvironmentState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: Environment[] }

export type SecretReferenceState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: SecretReference[] }

export type MemberState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: SystemMember[] }

type ReferenceStates = Record<string, SecretReferenceState>
type Feedback = { status: 'success' | 'error'; message: string } | undefined

const sensitiveVariableWords = ['password', 'token', 'secret', 'credential', 'api_key', 'apikey']
const externalSecretSchemes = ['vault://', 'secret://', 'aws-secrets://', 'gcp-secret://'] as const
const canManage = (role: SystemRole) => role === 'owner' || role === 'maintainer'
export const canManageMembers = (role: SystemRole) => role === 'owner'

export function parseEnvironmentVariables(source: string): Record<string, string> {
  if (source.trim() === '') return {}
  const value: unknown = JSON.parse(source)
  if (value === null || Array.isArray(value) || typeof value !== 'object') throw new Error('环境变量必须是 JSON 对象')
  const entries = Object.entries(value)
  if (entries.some(([, item]) => typeof item !== 'string')) throw new Error('环境变量值必须都是字符串')
  if (entries.some(([key]) => sensitiveVariableWords.some((word) => key.toLowerCase().includes(word)))) {
    throw new Error('敏感变量必须通过外部密钥引用配置')
  }
  return Object.fromEntries(entries) as Record<string, string>
}

export function validateExternalSecretReference(value: string): boolean {
  return externalSecretSchemes.some((scheme) => value.startsWith(scheme) && value.length > scheme.length)
}

export function SystemSettingsPage() {
  const context = useOutletContext<SystemContextState>()
  const system = context.status === 'ready' ? context.system : undefined
  const [state, setState] = useState<EnvironmentState>({ status: 'loading' })
  const [references, setReferences] = useState<ReferenceStates>({})
  const [feedback, setFeedback] = useState<Feedback>()
  const [memberState, setMemberState] = useState<MemberState>({ status: 'loading' })
  const [memberFeedback, setMemberFeedback] = useState<Feedback>()

  const loadReference = async (systemId: string, environmentId: string) => {
    setReferences((current) => ({ ...current, [environmentId]: { status: 'loading' } }))
    try {
      const items = await apiClient.listSecretReferences(systemId, environmentId)
      setReferences((current) => ({ ...current, [environmentId]: items.length === 0 ? { status: 'empty' } : { status: 'ready', items } }))
    } catch (error) {
      const message = error instanceof Error ? error.message : '密钥引用加载失败'
      setReferences((current) => ({ ...current, [environmentId]: { status: 'error', message } }))
    }
  }

  const loadEnvironments = async (systemId: string) => {
    setState({ status: 'loading' })
    try {
      const items = await apiClient.listEnvironments(systemId)
      setState(items.length === 0 ? { status: 'empty' } : { status: 'ready', items })
      setReferences({})
      for (const environment of items) void loadReference(systemId, environment.id)
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : '环境加载失败' })
    }
  }

  const loadMembers = async (systemId: string) => {
    setMemberState({ status: 'loading' })
    try {
      const items = await apiClient.listSystemMembers(systemId)
      setMemberState(items.length === 0 ? { status: 'empty' } : { status: 'ready', items })
    } catch (error) {
      setMemberState({ status: 'error', message: error instanceof Error ? error.message : '成员加载失败' })
    }
  }

  useEffect(() => {
    if (!system) return
    void loadEnvironments(system.id)
    if (canManageMembers(system.myRole)) void loadMembers(system.id)
    else setMemberState({ status: 'empty' })
  }, [system?.id, system?.myRole])

  if (!system) {
    return <main className="settings-page"><section className="page-card"><p>{context.status === 'error' ? '工作空间不可用' : '正在加载工作空间…'}</p></section></main>
  }

  const upsertEnvironment = async (input: UpsertEnvironmentInput) => {
    setFeedback(undefined)
    try {
      await apiClient.upsertEnvironment(system.id, input)
      setFeedback({ status: 'success', message: input.id ? '环境已更新' : '环境已创建' })
      await loadEnvironments(system.id)
    } catch (error) {
      setFeedback({ status: 'error', message: error instanceof Error ? error.message : '环境保存失败' })
    }
  }

  const createSecretReference = async (environmentId: string, input: CreateSecretReferenceInput) => {
    setFeedback(undefined)
    try {
      await apiClient.createSecretReference(system.id, environmentId, input)
      setFeedback({ status: 'success', message: '外部密钥引用已保存' })
      await loadReference(system.id, environmentId)
    } catch (error) {
      setFeedback({ status: 'error', message: error instanceof Error ? error.message : '外部密钥引用保存失败' })
    }
  }

  const upsertMember = async (input: UpsertSystemMemberInput) => {
    setMemberFeedback(undefined)
    try {
      await apiClient.upsertSystemMember(system.id, input)
      setMemberFeedback({ status: 'success', message: '成员角色已保存' })
      await loadMembers(system.id)
    } catch (error) {
      setMemberFeedback({ status: 'error', message: error instanceof Error ? error.message : '成员角色保存失败' })
    }
  }

  return <main className="settings-page">
    <CodeSourceSettingsPage />
    <EnvironmentSettingsView role={system.myRole} state={state} references={references} feedback={feedback} onUpsertEnvironment={upsertEnvironment} onCreateSecretReference={createSecretReference} />
    <MemberSettingsView role={system.myRole} state={memberState} feedback={memberFeedback} onUpsertMember={upsertMember} />
  </main>
}

export function EnvironmentSettingsView({ role, state, references, feedback, onUpsertEnvironment, onCreateSecretReference }: {
  role: SystemRole
  state: EnvironmentState
  references: ReferenceStates
  feedback?: Feedback
  onUpsertEnvironment: (input: UpsertEnvironmentInput) => void | Promise<void>
  onCreateSecretReference: (environmentId: string, input: CreateSecretReferenceInput) => void | Promise<void>
}) {
  const writable = canManage(role)
  return (
    <section className="workspace-page">
      <header className="page-header">
        <div><small>系统设置</small><h1>环境与密钥引用</h1><p>管理非敏感环境变量，并通过外部引用连接密钥系统。</p></div>
      </header>
      {feedback && <p className={feedback.status === 'error' ? 'error' : 'status'}>{feedback.message}</p>}
      {writable && <EnvironmentForm title="新建环境" submitLabel="创建环境" onSubmit={onUpsertEnvironment} />}
      {state.status === 'loading' && <div className="page-card"><p>正在加载环境…</p></div>}
      {state.status === 'error' && <div className="page-card"><p className="error">{state.message}</p></div>}
      {state.status === 'empty' && <div className="page-card"><p>尚未配置环境</p></div>}
      {state.status === 'ready' && state.items.map((environment) => (
        <article className="page-card" key={environment.id}>
          <header><div><small>{environment.key}</small><h2>{environment.name}</h2></div><span className="status">{environment.status === 'active' ? '启用' : '停用'}</span></header>
          <h3>非敏感环境变量</h3>
          {Object.keys(environment.variables ?? {}).length === 0
            ? <p>未配置非敏感变量</p>
            : <dl>{Object.entries(environment.variables).map(([key, value]) => <div key={key}><dt>{key}</dt><dd><code>{value}</code></dd></div>)}</dl>}
          {writable && <EnvironmentForm title="更新环境" submitLabel="保存更新" environment={environment} onSubmit={onUpsertEnvironment} />}
          <h3>外部密钥引用</h3>
          <SecretReferencesView state={references[environment.id] ?? { status: 'loading' }} />
          {writable && <SecretReferenceForm environmentId={environment.id} onSubmit={onCreateSecretReference} />}
        </article>
      ))}
    </section>
  )
}

export function MemberSettingsView({ role, state, feedback, onUpsertMember }: {
  role: SystemRole
  state: MemberState
  feedback?: Feedback
  onUpsertMember: (input: UpsertSystemMemberInput) => void | Promise<void>
}) {
  if (!canManageMembers(role)) {
    return <section className="workspace-page"><div className="page-card"><h2>系统成员</h2><p>仅系统所有者可管理成员；当前角色无权读取成员目录。</p></div></section>
  }
  return (
    <section className="workspace-page">
      <header className="page-header"><div><small>访问控制</small><h2>系统成员</h2><p>添加已有平台用户，或更新其系统级角色。</p></div></header>
      {feedback && <p className={feedback.status === 'error' ? 'error' : 'status'}>{feedback.message}</p>}
      <MemberRoleForm title="添加或更新成员" submitLabel="保存成员" onSubmit={onUpsertMember} />
      {state.status === 'loading' && <div className="page-card"><p>正在加载成员…</p></div>}
      {state.status === 'error' && <div className="page-card"><p className="error">{state.message}</p></div>}
      {state.status === 'empty' && <div className="page-card"><p>尚未配置成员</p></div>}
      {state.status === 'ready' && <div className="page-card"><ul>{state.items.map((member) => (
        <li key={member.userId}>
          <strong>{member.displayName || member.userId}</strong>
          {member.email && <> · {member.email}</>}
          <> · {member.status === 'active' ? '已启用' : '已停用'}</>
          <MemberRoleForm title="更新角色" submitLabel="更新角色" member={member} onSubmit={onUpsertMember} />
        </li>
      ))}</ul></div>}
    </section>
  )
}

const systemRoles: { value: SystemRole; label: string }[] = [
  { value: 'owner', label: '所有者' },
  { value: 'maintainer', label: '维护者' },
  { value: 'reviewer', label: '审核者' },
  { value: 'runner', label: '执行者' },
  { value: 'viewer', label: '只读成员' },
]

function MemberRoleForm({ title, submitLabel, member, onSubmit }: {
  title: string
  submitLabel: string
  member?: SystemMember
  onSubmit: (input: UpsertSystemMemberInput) => void | Promise<void>
}) {
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    void onSubmit({ userId: String(form.get('userId') ?? '').trim(), role: String(form.get('role') ?? 'viewer') as SystemRole })
  }
  return (
    <form className="settings-form" onSubmit={handleSubmit}>
      <h3>{title}</h3>
      <label>用户 ID<input name="userId" required readOnly={Boolean(member)} defaultValue={member?.userId} placeholder="用户 UUID" /></label>
      <label>系统角色<select name="role" defaultValue={member?.role ?? 'viewer'}>{systemRoles.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label>
      <button type="submit">{submitLabel}</button>
    </form>
  )
}

function EnvironmentForm({ title, submitLabel, environment, onSubmit }: { title: string; submitLabel: string; environment?: Environment; onSubmit: (input: UpsertEnvironmentInput) => void | Promise<void> }) {
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    try {
      void onSubmit({
        ...(environment ? { id: environment.id } : {}),
        key: String(form.get('key') ?? '').trim(),
        name: String(form.get('name') ?? '').trim(),
        status: form.get('status') === 'disabled' ? 'disabled' : 'active',
        variables: parseEnvironmentVariables(String(form.get('variables') ?? '')),
      })
    } catch (error) {
      event.currentTarget.querySelector<HTMLElement>('[data-form-error]')!.textContent = error instanceof Error ? error.message : '环境变量格式错误'
    }
  }
  return (
    <form className="settings-form" onSubmit={handleSubmit}>
      <h3>{title}</h3>
      <label>环境 key<input name="key" required defaultValue={environment?.key} /></label>
      <label>环境名称<input name="name" required defaultValue={environment?.name} /></label>
      <label>状态<select name="status" defaultValue={environment?.status ?? 'active'}><option value="active">启用</option><option value="disabled">停用</option></select></label>
      <label>非敏感变量（JSON 对象）<textarea name="variables" rows={4} defaultValue={JSON.stringify(environment?.variables ?? {}, null, 2)} /></label>
      <small>password、token、secret、credential、api_key 等敏感键只能使用外部密钥引用。</small>
      <p className="error" data-form-error />
      <button type="submit">{submitLabel}</button>
    </form>
  )
}

function SecretReferencesView({ state }: { state: SecretReferenceState }) {
  if (state.status === 'loading') return <p>正在加载密钥引用…</p>
  if (state.status === 'error') return <p className="error">{state.message}</p>
  if (state.status === 'empty') return <p>尚未配置外部密钥引用</p>
  return <ul>{state.items.map((reference) => <li key={reference.id}><strong>{reference.variableKey}</strong>：{reference.secretRef ? <code>{reference.secretRef}</code> : <span>引用位置已隐藏</span>}</li>)}</ul>
}

function SecretReferenceForm({ environmentId, onSubmit }: { environmentId: string; onSubmit: (environmentId: string, input: CreateSecretReferenceInput) => void | Promise<void> }) {
  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    const secretRef = String(form.get('secretRef') ?? '').trim()
    const error = event.currentTarget.querySelector<HTMLElement>('[data-secret-error]')!
    if (!validateExternalSecretReference(secretRef)) {
      error.textContent = '请输入受支持且非空的外部密钥引用'
      return
    }
    error.textContent = ''
    void onSubmit(environmentId, { variableKey: String(form.get('variableKey') ?? '').trim(), secretRef })
  }
  return (
    <form className="settings-form" onSubmit={handleSubmit}>
      <h3>新增外部密钥引用</h3>
      <label>变量键<input name="variableKey" required placeholder="DB_PASSWORD" /></label>
      <label>引用地址<input name="secretRef" required placeholder="vault://path/to/secret" /></label>
      <small>支持 {externalSecretSchemes.join('、')}；这里只保存引用地址。</small>
      <p className="error" data-secret-error />
      <button type="submit">保存引用</button>
    </form>
  )
}
