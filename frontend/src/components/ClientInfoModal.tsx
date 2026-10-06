import { useCallback, useEffect, useMemo, useState } from 'react'
import { API_BASE, api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'

type Tab = 'info' | 'links' | 'sub' | 'qr'

type Props = {
  open: boolean
  client: Client | null
  inbounds?: Inbound[]
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

export function ClientInfoModal({ open, client, inbounds = [], initialTab = 'info', onClose, onResetTraffic }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('info')
  const [links, setLinks] = useState<string[]>([])
  const [subUrls, setSubUrls] = useState<Record<string, string> | null>(null)
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
      // Fallback to single-link endpoint
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

  useEffect(() => {
    if (!open || !client) return
    setTab(initialTab)
    setMsg('')
    setLinks([])
    setSubUrls(null)
  }, [open, client, initialTab])

  useEffect(() => {
    if (!open || !client) return
    if (tab === 'links') void loadLinks()
    if (tab === 'sub') void loadSub()
  }, [open, client, tab, loadLinks, loadSub])

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

  const tabs: { id: Tab; label: string }[] = [
    { id: 'info', label: tr('tabGeneral') },
    { id: 'links', label: tr('link') },
    { id: 'sub', label: tr('subscription') },
    { id: 'qr', label: 'QR' },
  ]

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal" style={{ width: 'min(640px, 100%)' }} onClick={(e) => e.stopPropagation()}>
        <div className="page-head" style={{ marginBottom: 8 }}>
          <div>
            <h3 style={{ margin: 0 }}>{client.email}</h3>
            <p className="page-sub" style={{ margin: '4px 0 0' }}>
              {client.enable ? tr('enable') : tr('disable')}
              {client.group ? ` · ${client.group}` : ''}
            </p>
          </div>
          <button type="button" className="btn secondary" onClick={onClose}>{tr('cancel')}</button>
        </div>

        <div className="tabs" style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginBottom: 12 }}>
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

        {msg && <p className="page-sub" style={{ marginTop: 10 }}>{msg}</p>}

        <style>{`
          .tab {
            border: 1px solid var(--border);
            background: transparent;
            color: var(--text-muted);
            border-radius: 999px;
            padding: 0.4rem 0.85rem;
            cursor: pointer;
            font-weight: 600;
            font-size: 0.85rem;
          }
          .tab.active {
            background: var(--accent-soft);
            color: var(--accent);
            border-color: transparent;
          }
        `}</style>
      </div>
    </div>
  )
}
