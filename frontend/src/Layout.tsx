import { NavLink, Outlet } from 'react-router-dom'
import { useApp } from './AppContext'
import { api } from './api'

const links = [
  ['/', 'dashboard'],
  ['/inbounds', 'inbounds'],
  ['/outbounds', 'outbounds'],
  ['/bridges', 'bridges'],
  ['/nodes', 'nodes'],
  ['/tgproxy', 'tgproxy'],
  ['/routing', 'routing'],
  ['/settings', 'settings'],
] as const

export function Layout() {
  const { tr, setAuthed } = useApp()
  async function logout() {
    await api('/logout', { method: 'POST' })
    setAuthed(false)
  }
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="brand">{tr('brand')}</div>
        <nav>
          {links.map(([to, key]) => (
            <NavLink key={to} to={to} end={to === '/'} className={({ isActive }) => (isActive ? 'nav active' : 'nav')}>
              {tr(key)}
            </NavLink>
          ))}
        </nav>
        <button className="btn ghost logout" onClick={logout}>
          {tr('logout')}
        </button>
      </aside>
      <main className="content">
        <Outlet />
      </main>
      <style>{`
        .shell { display: grid; grid-template-columns: 240px 1fr; min-height: 100vh; }
        .sidebar { background: var(--bg-sidebar); border-right: 1px solid var(--border); padding: 1.25rem 0.9rem; display: flex; flex-direction: column; gap: 0.35rem; }
        .brand { font-size: 1.35rem; font-weight: 800; letter-spacing: -0.03em; margin: 0.2rem 0.55rem 1.1rem; color: var(--accent); }
        .nav { display: block; padding: 0.55rem 0.7rem; border-radius: 10px; color: var(--text-muted); font-weight: 600; }
        .nav:hover { background: var(--bg-elevated); color: var(--text); }
        .nav.active { background: var(--accent); color: #fff; }
        .logout { margin-top: auto; }
        .content { padding: 1.4rem 1.6rem; }
        @media (max-width: 860px) {
          .shell { grid-template-columns: 1fr; }
          .sidebar { border-right: none; border-bottom: 1px solid var(--border); }
          nav { display: flex; flex-wrap: wrap; gap: 0.25rem; }
        }
      `}</style>
    </div>
  )
}
