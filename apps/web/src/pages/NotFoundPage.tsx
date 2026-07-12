import { Link } from 'react-router-dom'

export function NotFoundPage() {
  return <main className="not-found"><strong>404</strong><h1>页面不存在</h1><Link to="/dashboard">返回全局仪表盘</Link></main>
}
