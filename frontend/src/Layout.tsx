import { NavLink, Outlet } from 'react-router-dom'
import { useApp } from './AppContext'
import { api } from './api'
import type { DictKey } from './i18n'

type NavItem = readonly [to: string, key: DictKey, icon: string]
type NavSection = { label: DictKey; items: readonly NavItem[] }

const sections: readonly NavSection[] = [
  {
    label: 'navOverview',
    items: [['/', 'dashboard', '◎']],
  },
  {
    label: 'navTraffic',
    items: [
      ['/inbounds', 'inbounds', '↓'],
      ['/clients', 'clients', '◉'],
      ['/groups', 'groups', '▦'],
      ['/hosts', 'hosts', '⌂'],
    ],
  },
  {
    label: 'navNetwork',
    items: [
      ['/outbounds', 'outbounds', '↑'],
      ['/bridges', 'bridges', '⇄'],
      ['/nodes', 'nodes', '⬡'],
      ['/tgproxy', 'tgproxy', '✈'],
      ['/routing', 'routing', '⎇'],
      ['/xray', 'xray', '✦'],
    ],
  },
  {
    label: 'navSystem',
    items: [
      ['/logs', 'logs', '☰'],
      ['/settings', 'settings', '⚙'],
    ],
  },
]

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
          {sections.map((section) => (
            <div key={section.label} className="nav-section">
              <div className="section-title nav-section-title">{tr(section.label)}</div>
              {section.items.map(([to, key, icon]) => (
                <NavLink
                  key={to}
                  to={to}
                  end={to === '/'}
                  className={({ isActive }) => (isActive ? 'nav active' : 'nav')}
                >
                  <span className="nav-ico" aria-hidden>{icon}</span>
                  <span>{tr(key)}</span>
                </NavLink>
              ))}
            </div>
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
          overflow: auto;
        }
        .brand-block {
          display: flex;
          gap: 0.75rem;
          align-items: center;
          margin: 0.15rem 0.45rem 1rem;
        }
        .brand-mark {
          width: 36px; height: 36px; border-radius: 11px;
          background:
            linear-gradient(145deg, var(--accent), transparent 62%),
            linear-gradient(225deg, rgba(14,165,233,.45), transparent 55%);
          border: 1px solid var(--border);
          box-shadow: 0 0 20px var(--glow);
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
        .nav-list { display: flex; flex-direction: column; gap: 0.15rem; flex: 1; }
        .nav-section { display: flex; flex-direction: column; gap: 0.15rem; }
        .nav-section-title {
          margin: 0.9rem 0.7rem 0.3rem;
          font-size: 0.68rem;
        }
        .nav-section:first-child .nav-section-title { margin-top: 0.15rem; }
        .nav {
          display: flex;
          align-items: center;
          gap: 0.65rem;
          padding: 0.55rem 0.75rem;
          border-radius: 10px;
          color: var(--text-muted);
          font-weight: 600;
          font-size: 0.92rem;
          transition: background .15s, color .15s;
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
          opacity: 0.8;
          font-size: 0.9rem;
          font-family: var(--mono);
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
            overflow: visible;
          }
          .nav-list { flex-direction: row; flex-wrap: wrap; gap: 0.35rem; }
          .nav-section { flex-direction: row; flex-wrap: wrap; gap: 0.2rem; align-items: center; }
          .nav-section-title {
            width: 100%;
            margin: 0.5rem 0.35rem 0.15rem;
          }
          .nav { padding: 0.45rem 0.65rem; }
          .logout { margin-top: 0.75rem; width: auto; }
          .content { padding: 1.1rem 1rem 2rem; }
        }
      `}</style>
    </div>
  )
}
