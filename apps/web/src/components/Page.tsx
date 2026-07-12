import type { ReactNode } from 'react'

interface PageProps {
  eyebrow?: string
  title: string
  description: string
  action?: ReactNode
  children?: ReactNode
}

export function Page({ eyebrow, title, description, action, children }: PageProps) {
  return (
    <div className="page">
      <header className="page-header">
        <div>
          {eyebrow && <div className="eyebrow">{eyebrow}</div>}
          <h1>{title}</h1>
          <p>{description}</p>
        </div>
        {action && <div className="page-actions">{action}</div>}
      </header>
      {children}
    </div>
  )
}

export function MetricCard({ label, value, hint }: { label: string; value: string; hint: string }) {
  return <article className="metric-card"><span>{label}</span><strong>{value}</strong><small>{hint}</small></article>
}

export function PlaceholderPanel({ title, children }: { title: string; children: ReactNode }) {
  return <section className="panel"><h2>{title}</h2>{children}</section>
}
