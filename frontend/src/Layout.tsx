import { NavLink, Outlet } from 'react-router-dom'
import { useApp } from './AppContext'
import { api } from './api'

const links = [
  ['/', 'dashboard', '◎'],
  ['/inbounds', 'inbounds', '⇢'],
  ['/clients', 'clients', '◉'],
  ['/outbounds', 'outbounds', '⇠'],
  ['/bridges', 'bridges', '⇄'],
  ['/nodes', 'nodes', '⬡'],
  ['/tgproxy', 'tgproxy', '✈'],
  ['/routing', 'routing', '⎇'],
  ['/xray', 'xray', '✦'],
  ['/logs', 'logs', '☰'],
  ['/settings', 'settings', '⚙'],
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
        <div className="brand-block">
          <div className="brand-mark" aria-hidden />
          <div>
            <div className="brand">{tr('brand')}</div>
            <div className="brand-tag">Xray control</div>
          </div>
        </div>
        <nav className="nav-list">
          {links.map(([to, key, icon]) => (
            <NavLink key={to} to={to} end={to === '/'} className={({ isActive }) => (isActive ? 'nav active' : 'nav')}>
              <span className="nav-ico" aria-hidden>{icon}</span>
              <span>{tr(key)}</span>
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
        .shell {
          display: grid;
          grid-template-columns: 260px 1fr;
          min-height: 100vh;
        }
        .sidebar {
          position: sticky;
          top: 0;
          height: 100vh;
          padding: 1.35rem 0.95rem;
          display: flex;
          flex-direction: column;
          gap: 0.35rem;
          background: var(--bg-sidebar);
          border-right: 1px solid var(--border);
          backdrop-filter: blur(18px);
        }
        .brand-block {
          display: flex;
          gap: 0.75rem;
          align-items: center;
          margin: 0.15rem 0.45rem 1.25rem;
        }
        .brand-mark {
          width: 36px; height: 36px; border-radius: 11px;
          background:
            linear-gradient(135deg, var(--accent), transparent 60%),
            linear-gradient(225deg, rgba(14,165,233,.55), transparent 55%);
          border: 1px solid var(--border);
          box-shadow: 0 0 24px var(--glow);
        }
        .brand {
          font-size: 1.2rem;
          font-weight: 780;
          letter-spacing: -0.04em;
          line-height: 1.1;
        }
        .brand-tag {
          color: var(--text-muted);
          font-size: 0.72rem;
          letter-spacing: 0.04em;
          text-transform: uppercase;
          margin-top: 2px;
        }
        .nav-list { display: flex; flex-direction: column; gap: 0.2rem; }
        .nav {
          display: flex;
          align-items: center;
          gap: 0.65rem;
          padding: 0.62rem 0.75rem;
          border-radius: 12px;
          color: var(--text-muted);
          font-weight: 600;
          transition: background .15s, color .15s, transform .15s;
        }
        .nav:hover { background: var(--accent-soft); color: var(--text); }
        .nav.active {
          background: var(--accent-soft);
          color: var(--accent);
          box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 35%, transparent);
        }
        .nav-ico {
          width: 1.25rem;
          text-align: center;
          opacity: 0.85;
          font-size: 0.95rem;
        }
        .logout { margin-top: auto; width: 100%; }
        .content {
          padding: 1.6rem 1.8rem 2.4rem;
          min-width: 0;
        }
        @media (max-width: 900px) {
          .shell { grid-template-columns: 1fr; }
          .sidebar {
            position: relative;
            height: auto;
            border-right: none;
            border-bottom: 1px solid var(--border);
          }
          .nav-list { flex-direction: row; flex-wrap: wrap; }
          .nav { padding: 0.5rem 0.7rem; }
          .logout { margin-top: 0.75rem; width: auto; }
          .content { padding: 1.1rem 1rem 2rem; }
        }
      `}</style>
    </div>
  )
}
