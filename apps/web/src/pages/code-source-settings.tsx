import { useEffect, useState, type FormEvent } from 'react'
import { useOutletContext } from 'react-router-dom'
import { apiClient } from '../api/client'
import type { CodeSource, CodeSourceType, SystemRole, UpsertCodeSourceInput } from '../api/types'
import type { SystemContextState } from '../layouts/SystemLayout'

export type CodeSourceState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'empty' }
  | { status: 'ready'; items: CodeSource[] }

export type CodeSourceFeedback = { status: 'success' | 'error'; message: string } | undefined

export const canManageCodeSources = (role: SystemRole): boolean => role === 'owner' || role === 'maintainer'

export function parsePathList(source: string): string[] {
  return [...new Set(source.split(/[\n,]/).map((item) => item.trim()).filter(Boolean))]
}

export function validateCodeSourceInput(input: UpsertCodeSourceInput): UpsertCodeSourceInput {
  const normalized: UpsertCodeSourceInput = {
    ...input,
    name: input.name.trim(),
    repositoryUrl: input.repositoryUrl?.trim() || undefined,
    localPath: input.localPath?.trim() || undefined,
    defaultRef: input.defaultRef?.trim() || undefined,
    credentialRef: input.credentialRef?.trim() || undefined,
    includePaths: [...new Set(input.includePaths.map((item) => item.trim()).filter(Boolean))],
    excludePaths: [...new Set(input.excludePaths.map((item) => item.trim()).filter(Boolean))],
  }
  if (!normalized.name) throw new Error('请输入代码源名称')
  if (normalized.name.length > 128) throw new Error('代码源名称不能超过 128 个字符')
  for (const path of [...normalized.includePaths, ...normalized.excludePaths]) {
    const clean = path.replaceAll('\\', '/')
    if (clean.startsWith('/') || clean === '..' || clean.startsWith('../') || clean.includes('/../')) throw new Error('包含和排除项必须是无路径穿越的相对路径')
  }
  if (normalized.sourceType === 'git') {
    if (!normalized.repositoryUrl) throw new Error('Git 代码源必须填写仓库地址')
    try {
      const repository = new URL(normalized.repositoryUrl)
      if (!['https:', 'ssh:'].includes(repository.protocol) || !repository.host) throw new Error()
      if (repository.username || repository.password) throw new Error('仓库地址不能包含内嵌凭据，请使用外部凭据引用')
    } catch (cause) {
      if (cause instanceof Error && cause.message.includes('内嵌凭据')) throw cause
      throw new Error('仓库地址必须是有效的 HTTPS 或 SSH URL')
    }
    if (normalized.credentialRef && !['vault://', 'secret://', 'aws-secrets://', 'gcp-secret://'].some((prefix) => normalized.credentialRef!.startsWith(prefix) && normalized.credentialRef!.length > prefix.length)) {
      throw new Error('凭据必须填写受支持的外部凭据引用')
    }
    delete normalized.localPath
  } else {
    if (!normalized.localPath) throw new Error('本地代码源必须填写本地路径')
    if (!normalized.localPath.startsWith('/') && !/^[A-Za-z]:[\\/]/.test(normalized.localPath)) throw new Error('本地路径必须是绝对路径')
    delete normalized.repositoryUrl
    delete normalized.defaultRef
    delete normalized.credentialRef
  }
  return normalized
}

export function CodeSourceSettingsPage() {
  const context = useOutletContext<SystemContextState>()
  const system = context.status === 'ready' ? context.system : undefined
  const [state, setState] = useState<CodeSourceState>({ status: 'loading' })
  const [feedback, setFeedback] = useState<CodeSourceFeedback>()

  const load = async (systemId: string) => {
    setState({ status: 'loading' })
    try {
      const items = await apiClient.listCodeSources(systemId)
      setState(items.length === 0 ? { status: 'empty' } : { status: 'ready', items })
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : '代码源加载失败' })
    }
  }

  useEffect(() => {
    if (system) void load(system.id)
  }, [system?.id])

  if (!system) {
    return <main className="settings-page"><section className="page-card"><p>{context.status === 'error' ? '工作空间不可用' : '正在加载工作空间…'}</p></section></main>
  }

  const save = async (input: UpsertCodeSourceInput, sourceId?: string) => {
    setFeedback(undefined)
    try {
      if (sourceId) await apiClient.updateCodeSource(system.id, sourceId, input)
      else await apiClient.createCodeSource(system.id, input)
      setFeedback({ status: 'success', message: sourceId ? '代码源已更新' : '代码源已创建' })
      await load(system.id)
    } catch (error) {
      setFeedback({ status: 'error', message: error instanceof Error ? error.message : '代码源保存失败' })
    }
  }

  return <main className="settings-page"><CodeSourceSettingsView
    role={system.myRole}
    state={state}
    feedback={feedback}
    onCreate={(input) => save(input)}
    onUpdate={(sourceId, input) => save(input, sourceId)}
  /></main>
}

export function CodeSourceSettingsView({ role, state, feedback, onCreate, onUpdate }: {
  role: SystemRole
  state: CodeSourceState
  feedback?: CodeSourceFeedback
  onCreate: (input: UpsertCodeSourceInput) => void | Promise<void>
  onUpdate: (sourceId: string, input: UpsertCodeSourceInput) => void | Promise<void>
}) {
  const writable = canManageCodeSources(role)
  return (
    <section className="workspace-page">
      <header className="page-header">
        <div><small>系统设置</small><h1>代码源</h1><p>登记 Git 仓库或扫描代理可访问的本地目录，供扫描任务直接选择。</p></div>
      </header>
      {!writable && <div className="page-card"><p>当前角色为只读，可查看代码源但不能新增或编辑。</p></div>}
      {feedback && <p className={feedback.status === 'error' ? 'error' : 'status'}>{feedback.message}</p>}
      {writable && <CodeSourceForm title="新建代码源" submitLabel="创建代码源" onSubmit={onCreate} />}
      {state.status === 'loading' && <div className="page-card"><p>正在加载代码源…</p></div>}
      {state.status === 'error' && <div className="page-card"><p className="error">{state.message}</p></div>}
      {state.status === 'empty' && <div className="page-card"><p>尚未配置代码源</p></div>}
      {state.status === 'ready' && state.items.map((source) => (
        <article className="page-card" key={source.id}>
          <header><div><small>{source.sourceType === 'git' ? 'Git 仓库' : '本地目录'}</small><h2>{source.name}</h2></div><span className="status">{source.status === 'active' ? '启用' : '停用'}</span></header>
          <dl>
            <div><dt>代码源 ID</dt><dd><code>{source.id}</code></dd></div>
            {source.repositoryUrl && <div><dt>仓库地址</dt><dd><code>{source.repositoryUrl}</code></dd></div>}
            {source.localPath && <div><dt>本地路径</dt><dd><code>{source.localPath}</code></dd></div>}
            {source.defaultRef && <div><dt>默认分支或引用</dt><dd><code>{source.defaultRef}</code></dd></div>}
            <div><dt>包含路径</dt><dd>{source.includePaths.length ? source.includePaths.join('、') : '全部路径'}</dd></div>
            <div><dt>排除路径</dt><dd>{source.excludePaths.length ? source.excludePaths.join('、') : '未配置'}</dd></div>
            {source.credentialRef && <div><dt>外部凭据引用</dt><dd><code>{source.credentialRef}</code></dd></div>}
          </dl>
          {writable && <CodeSourceForm title="编辑代码源" submitLabel="保存更新" source={source} onSubmit={(input) => onUpdate(source.id, input)} />}
        </article>
      ))}
    </section>
  )
}

function CodeSourceForm({ title, submitLabel, source, onSubmit }: {
  title: string
  submitLabel: string
  source?: CodeSource
  onSubmit: (input: UpsertCodeSourceInput) => void | Promise<void>
}) {
  const [sourceType, setSourceType] = useState<CodeSourceType>(source?.sourceType ?? 'git')
  const [error, setError] = useState('')

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const form = new FormData(event.currentTarget)
    try {
      const input = validateCodeSourceInput({
        name: String(form.get('name') ?? ''),
        sourceType,
        repositoryUrl: String(form.get('repositoryUrl') ?? ''),
        localPath: String(form.get('localPath') ?? ''),
        defaultRef: String(form.get('defaultRef') ?? ''),
        includePaths: parsePathList(String(form.get('includePaths') ?? '')),
        excludePaths: parsePathList(String(form.get('excludePaths') ?? '')),
        credentialRef: String(form.get('credentialRef') ?? ''),
        status: form.get('status') === 'disabled' ? 'disabled' : 'active',
      })
      setError('')
      void onSubmit(input)
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : '代码源配置无效')
    }
  }

  return (
    <form className="settings-form" onSubmit={handleSubmit}>
      <h3>{title}</h3>
      <label>名称<input name="name" required defaultValue={source?.name} /></label>
      <label>类型<select name="sourceType" value={sourceType} onChange={(event) => setSourceType(event.target.value as CodeSourceType)}><option value="git">Git 仓库</option><option value="local">本地目录</option></select></label>
      {sourceType === 'git'
        ? <><label>仓库地址<input name="repositoryUrl" required defaultValue={source?.repositoryUrl} placeholder="https://git.example.test/team/service.git" /></label><label>默认分支或引用<input name="defaultRef" defaultValue={source?.defaultRef} placeholder="main" /></label><label>外部凭据引用<input name="credentialRef" defaultValue={source?.credentialRef} placeholder="vault://git/team/service" /></label><small>只填写外部凭据引用，不接收密码、令牌、私钥等秘密值。</small></>
        : <label>本地路径<input name="localPath" required defaultValue={source?.localPath} placeholder="/workspace/service" /></label>}
      <label>包含路径<textarea name="includePaths" rows={3} defaultValue={(source?.includePaths ?? []).join('\n')} placeholder={'src\ncmd'} /></label>
      <label>排除路径<textarea name="excludePaths" rows={3} defaultValue={(source?.excludePaths ?? []).join('\n')} placeholder={'vendor\ndist'} /></label>
      <small>路径可按换行或逗号分隔；留空表示不限制。</small>
      <label>状态<select name="status" defaultValue={source?.status ?? 'active'}><option value="active">启用</option><option value="disabled">停用</option></select></label>
      {error && <p className="error">{error}</p>}
      <button type="submit">{submitLabel}</button>
    </form>
  )
}
