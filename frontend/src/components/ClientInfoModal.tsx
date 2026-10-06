import { useCallback, useEffect, useMemo, useState } from 'react'
import { API_BASE, api, type Client, type ClientHWIDRow, type ClientIPRow, type Inbound } from '../api'
import { useApp } from '../AppContext'

type Tab = 'info' | 'links' | 'sub' | 'qr' | 'ips' | 'hwids'

type Props = {
  open: boolean
  client: Client | null
  inbounds?: Inbound[]
  online?: boolean
  initialTab?: Tab
  onClose: () => void
  onResetTraffic?: () => void
}

function formatTraffic(c: Client): string {
  const used = ((c.up || 0) + (c.down || 0)) / (1024 * 1024 * 1024)
  const total = c.totalGB || 0
  if (total <= 0) return `${used.toFixed(2)} GB / ∞`
  return `${used.toFixed(2)} / ${total} GB`
}

function expiryLabel(c: Client): string {
  if (!c.expiryTime) return '∞'
  return new Date(c.expiryTime).toLocaleString()
}

function clientInboundList(client: Client, inbounds: Inbound[]): Inbound[] {
  const ids = new Set<number>()
  if (client.inboundIds) {
    for (const part of client.inboundIds.split(',')) {
      const n = Number(part.trim())
      if (n > 0) ids.add(n)
    }
  }
  if (client.inboundId) ids.add(client.inboundId)
  return inbounds.filter((i) => ids.has(i.id))
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    return false
  }
}

function encEmail(email: string) {
  return encodeURIComponent(email)
}

export function ClientInfoModal({ open, client, inbounds = [], online = false, initialTab = 'info', onClose, onResetTraffic }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('info')
  const [links, setLinks] = useState<string[]>([])
  const [subUrls, setSubUrls] = useState<Record<string, string> | null>(null)
  const [ips, setIps] = useState<ClientIPRow[]>([])
  const [hwids, setHwids] = useState<ClientHWIDRow[]>([])
  const [busy, setBusy] = useState(false)
  const [msg, setMsg] = useState('')

  const attached = useMemo(
    () => (client ? clientInboundList(client, inbounds) : []),
    [client, inbounds],
  )

  const qrSrc = client
    ? `${API_BASE}/clients/${client.id}/qr`
    : ''

  const loadLinks = useCallback(async () => {
    if (!client) return
    setBusy(true)
    setMsg('')
    try {
      const data = await api<{ links: string[] }>(`/clients/${client.id}/links`)
      setLinks(data.links || [])
    } catch (e) {
      try {
        const one = await api<{ link: string }>(`/clients/${client.id}/link`)
        setLinks(one.link ? one.link.split('\n').filter(Boolean) : [])
      } catch (err) {
        setMsg(err instanceof Error ? err.message : 'error')
        setLinks([])
      }
    } finally {
      setBusy(false)
    }
  }, [client])

  const loadSub = useCallback(async () => {
    if (!client) return
    setBusy(true)
    setMsg('')
    try {
      const data = await api<{ urls: Record<string, string> }>(`/clients/${client.id}/sub`)
      setSubUrls(data.urls || {})
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
      setSubUrls(null)
    } finally {
      setBusy(false)
    }
  }, [client])

  const loadIps = useCallback(async () => {
    if (!client) return
    setBusy(true)
    setMsg('')
    try {
      setIps(await api<ClientIPRow[]>(`/clients/ips/${encEmail(client.email)}`))
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
      setIps([])
    } finally {
      setBusy(false)
    }
  }, [client])

  const loadHwids = useCallback(async () => {
    if (!client) return
    setBusy(true)
    setMsg('')
    try {
      setHwids(await api<ClientHWIDRow[]>(`/clients/hwids/${encEmail(client.email)}`))
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
      setHwids([])
    } finally {
      setBusy(false)
    }
  }, [client])

  useEffect(() => {
    if (!open || !client) return
    setTab(initialTab)
    setMsg('')
    setLinks([])
    setSubUrls(null)
    setIps([])
    setHwids([])
  }, [open, client, initialTab])

  useEffect(() => {
    if (!open || !client) return
    if (tab === 'links') void loadLinks()
    if (tab === 'sub') void loadSub()
    if (tab === 'ips') void loadIps()
    if (tab === 'hwids') void loadHwids()
  }, [open, client, tab, loadLinks, loadSub, loadIps, loadHwids])

  if (!open || !client) return null

  async function resetTraffic() {
    setBusy(true)
    setMsg('')
    try {
      await api(`/clients/${client!.id}/reset-traffic`, { method: 'POST' })
      setMsg('OK')
      onResetTraffic?.()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function clearIps() {
    if (!confirm(tr('clearIpsWarn') + '?')) return
    setBusy(true)
    try {
      await api(`/clients/${client!.id}/kick`, { method: 'POST' })
      setIps([])
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function clearHwids() {
    if (!confirm(tr('clearHwids') + '?')) return
    setBusy(true)
    try {
      await api(`/clients/hwids/${encEmail(client!.email)}`, { method: 'DELETE' })
      setHwids([])
      setMsg('OK')
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function deleteHwid(id: number) {
    setBusy(true)
    try {
      await api(`/clients/hwids/${encEmail(client!.email)}/${id}`, { method: 'DELETE' })
      await loadHwids()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  const tabs: { id: Tab; label: string }[] = [
    { id: 'info', label: tr('tabGeneral') },
    { id: 'links', label: tr('link') },
    { id: 'sub', label: tr('subscription') },
    { id: 'qr', label: 'QR' },
    { id: 'ips', label: 'IPs' },
    { id: 'hwids', label: 'HWID' },
  ]

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" style={{ width: 'min(640px, 100%)' }} onClick={(e) => e.stopPropagation()}>
        <div className="page-head" style={{ marginBottom: 8 }}>
          <div>
            <h3 style={{ margin: 0 }}>
              {client.email}{' '}
              {online && <span className="badge on">{tr('online')}</span>}
            </h3>
            <p className="page-sub" style={{ margin: '4px 0 0' }}>
              {client.enable ? tr('enable') : tr('disable')}
              {client.group ? ` · ${client.group}` : ''}
              {online ? ` · ${tr('online')}` : ''}
            </p>
          </div>
          <button type="button" className="btn secondary" onClick={onClose}>{tr('cancel')}</button>
        </div>

        <div className="tabs">
          {tabs.map((t) => (
            <button
              key={t.id}
              type="button"
              className={`tab ${tab === t.id ? 'active' : ''}`}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          ))}
        </div>

        {tab === 'info' && (
          <div>
            <div className="grid2">
              <div className="field">
                <label className="label">Email</label>
                <div className="row-actions">
                  <input className="input" readOnly value={client.email} onFocus={(e) => e.target.select()} />
                  <button type="button" className="btn secondary" onClick={() => void copyText(client.email)}>{tr('copy')}</button>
                </div>
              </div>
              <div className="field">
                <label className="label">UUID</label>
                <div className="row-actions">
                  <input className="input" readOnly value={client.uuid || ''} onFocus={(e) => e.target.select()} style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem' }} />
                  <button type="button" className="btn secondary" onClick={() => void copyText(client.uuid || '')}>{tr('copy')}</button>
                </div>
              </div>
              <div className="field">
                <label className="label">subId</label>
                <div className="row-actions">
                  <input className="input" readOnly value={client.subId || ''} onFocus={(e) => e.target.select()} />
                  <button type="button" className="btn secondary" onClick={() => void copyText(client.subId || '')}>{tr('copy')}</button>
                </div>
              </div>
              <div className="field">
                <label className="label">{tr('group')}</label>
                <input className="input" readOnly value={client.group || '—'} />
              </div>
              <div className="field">
                <label className="label">Traffic</label>
                <input className="input" readOnly value={formatTraffic(client)} />
              </div>
              <div className="field">
                <label className="label">Expiry</label>
                <input className="input" readOnly value={expiryLabel(client)} />
              </div>
              <div className="field">
                <label className="label">Status</label>
                <input className="input" readOnly value={online ? tr('online') : 'offline'} />
              </div>
              <div className="field">
                <label className="label">Limit IP / HWID</label>
                <input className="input" readOnly value={`${client.limitIp || 0} / ${client.limitHwid || 0}`} />
              </div>
            </div>
            <div className="field" style={{ marginTop: 8 }}>
              <label className="label">Inbounds</label>
              {attached.length === 0 ? (
                <p className="page-sub">—</p>
              ) : (
                <ul style={{ margin: '4px 0 0', paddingLeft: 18 }}>
                  {attached.map((i) => (
                    <li key={i.id}>
                      <span className="badge">{i.protocol}</span>{' '}
                      #{i.id} {i.remark || i.tag}:{i.port}
                    </li>
                  ))}
                </ul>
              )}
            </div>
            <div className="row-actions" style={{ marginTop: 12 }}>
              <button type="button" className="btn secondary" disabled={busy} onClick={() => void resetTraffic()}>
                {tr('resetTraffic')}
              </button>
            </div>
          </div>
        )}

        {tab === 'links' && (
          <div>
            {busy && <p className="page-sub">…</p>}
            {links.length === 0 && !busy && <p className="page-sub">{tr('empty')}</p>}
            {links.map((link, i) => (
              <div className="field" key={`${i}-${link.slice(0, 24)}`}>
                <label className="label">{tr('link')} {links.length > 1 ? i + 1 : ''}</label>
                <textarea className="textarea" readOnly rows={3} value={link} onFocus={(e) => e.target.select()} />
                <button type="button" className="btn secondary" style={{ marginTop: 6 }} onClick={() => void copyText(link)}>
                  {tr('copy')}
                </button>
              </div>
            ))}
            {links.length > 1 && (
              <button
                type="button"
                className="btn secondary"
                onClick={() => void copyText(links.join('\n'))}
              >
                {tr('copy')} all
              </button>
            )}
          </div>
        )}

        {tab === 'sub' && (
          <div>
            {busy && <p className="page-sub">…</p>}
            {!subUrls && !busy && <p className="page-sub">{tr('empty')}</p>}
            {subUrls && Object.entries(subUrls).map(([k, v]) => (
              <div className="field" key={k}>
                <label className="label">{k}</label>
                <div className="row-actions">
                  <input className="input" readOnly value={v} onFocus={(e) => e.target.select()} />
                  <button type="button" className="btn secondary" onClick={() => void copyText(v)}>{tr('copy')}</button>
                </div>
              </div>
            ))}
          </div>
        )}

        {tab === 'qr' && (
          <div style={{ textAlign: 'center' }}>
            <img
              src={qrSrc}
              alt="qr"
              style={{ width: 220, height: 220, background: '#fff', padding: 10, borderRadius: 10 }}
            />
            <p className="page-sub">{tr('link')} QR</p>
          </div>
        )}

        {tab === 'ips' && (
          <div>
            <div className="row-actions" style={{ marginBottom: 8 }}>
              <button type="button" className="btn secondary" disabled={busy} onClick={() => void loadIps()}>{tr('refresh')}</button>
              <button type="button" className="btn danger" disabled={busy || ips.length === 0} onClick={() => void clearIps()}>{tr('clearIpsWarn')}</button>
            </div>
            {busy && <p className="page-sub">…</p>}
            {ips.length === 0 && !busy && <p className="page-sub">{tr('empty')}</p>}
            {ips.length > 0 && (
              <table className="table">
                <thead>
                  <tr>
                    <th>IP</th>
                    <th>Last seen</th>
                  </tr>
                </thead>
                <tbody>
                  {ips.map((row) => (
                    <tr key={row.ip}>
                      <td><code style={{ fontFamily: 'var(--mono)' }}>{row.ip}</code></td>
                      <td>{row.lastSeen ? new Date(row.lastSeen).toLocaleString() : '—'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {tab === 'hwids' && (
          <div>
            <div className="row-actions" style={{ marginBottom: 8 }}>
              <button type="button" className="btn secondary" disabled={busy} onClick={() => void loadHwids()}>{tr('refresh')}</button>
              <button type="button" className="btn danger" disabled={busy || hwids.length === 0} onClick={() => void clearHwids()}>{tr('clearHwids')}</button>
            </div>
            <p className="page-sub" style={{ marginBottom: 8 }}>
              Limit: {client.limitHwid || 0} (0 = ∞) · registered: {hwids.length}
            </p>
            {busy && <p className="page-sub">…</p>}
            {hwids.length === 0 && !busy && <p className="page-sub">{tr('empty')}</p>}
            {hwids.length > 0 && (
              <table className="table">
                <thead>
                  <tr>
                    <th>HWID</th>
                    <th>Created</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {hwids.map((row) => (
                    <tr key={row.id}>
                      <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.8rem' }}>{row.hwid}</code></td>
                      <td>{row.createdAt ? new Date(row.createdAt).toLocaleString() : '—'}</td>
                      <td>
                        <button type="button" className="btn danger" disabled={busy} onClick={() => void deleteHwid(row.id)}>{tr('delete')}</button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        )}

        {msg && <p className="page-sub" style={{ marginTop: 10 }}>{msg}</p>}
      </div>
    </div>
  )
}
