import { useEffect } from 'react'
import { BrowserRouter, Navigate, Route, Routes } from 'react-router-dom'
import { api } from './api'
import { AppProvider, useApp } from './AppContext'
import { Layout } from './Layout'
import { BridgesPage } from './pages/Bridges'
import { DashboardPage } from './pages/Dashboard'
import { InboundsPage } from './pages/Inbounds'
import { LoginPage } from './pages/Login'
import { NodesPage } from './pages/Nodes'
import { OutboundsPage } from './pages/Outbounds'
import { RoutingPage } from './pages/Routing'
import { SettingsPage } from './pages/Settings'
import { TgProxyPage } from './pages/TgProxy'
import './styles.css'

function Authed() {
  const { authed, setAuthed } = useApp()
  useEffect(() => {
    api('/me').then(() => setAuthed(true)).catch(() => setAuthed(false))
  }, [setAuthed])

  if (!authed) return <LoginPage />
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/inbounds" element={<InboundsPage />} />
        <Route path="/outbounds" element={<OutboundsPage />} />
        <Route path="/bridges" element={<BridgesPage />} />
        <Route path="/nodes" element={<NodesPage />} />
        <Route path="/tgproxy" element={<TgProxyPage />} />
        <Route path="/routing" element={<RoutingPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}

export default function App() {
  const basename = (() => {
    const p = window.location.pathname
    const i = p.indexOf('/we1b')
    if (i >= 0) return p.slice(0, i) + '/we1b'
    return '/we1b'
  })()

  return (
    <AppProvider>
      <BrowserRouter basename={basename}>
        <Authed />
      </BrowserRouter>
    </AppProvider>
  )
}
