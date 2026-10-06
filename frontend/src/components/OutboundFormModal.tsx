import { FormEvent, useEffect, useState } from 'react'
import { api, type Outbound } from '../api'
import { useApp } from '../AppContext'
import {
  OUTBOUND_PROTOCOLS,
  OutboundFormState,
  applyParsedOutboundLink,
  buildOutboundSettings,
  buildOutboundStream,
  emptyOutboundForm,
  extractShareLinks,
  parseOutboundToForm,
} from '../lib/outboundForm'
import { parseOutboundLink } from '../lib/outboundLinkParser'
import { FINGERPRINTS, NETWORKS, SECURITIES } from '../lib/inboundForm'
import { randomUUID } from '../lib/random'
import { FormRow } from './FormRow'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  outbound: Outbound | null
  onClose: () => void
  onSaved: () => void
}

type Tab = 'basic' | 'stream'

export function OutboundFormModal({ open, mode, outbound, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('basic')
  const [form, setForm] = useState<OutboundFormState>(emptyOutboundForm())
  const [paste, setPaste] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setTab('basic')
    setError('')
    setPaste('')
    if (mode === 'edit' && outbound) {
      setForm(parseOutboundToForm(outbound))
    } else {
      const f = emptyOutboundForm()
      f.uuid = randomUUID()
      setForm(f)
    }
  }, [open, mode, outbound])

  function set<K extends keyof OutboundFormState>(key: K, value: OutboundFormState[K]) {
    setForm((prev) => {
      const next = { ...prev, [key]: value }
      if (key === 'protocol') {
        if (value === 'socks') next.port = 1080
        else if (value === 'http') next.port = 8080
        else if (value === 'dns') next.port = 53
        else if (!['freedom', 'blackhole'].includes(String(value))) next.port = 443
        if (['vless', 'vmess'].includes(String(value)) && !next.uuid) next.uuid = randomUUID()
      }
      return next
    })
  }

  const isProxy = !['freedom', 'blackhole', 'dns'].includes(form.protocol)
  const isDns = form.protocol === 'dns'

  async function resolvePasteToLinks(raw: string): Promise<string[]> {
    const trimmed = raw.trim()
    if (!trimmed) return []
    if (/^https?:\/\//i.test(trimmed)) {
      const data = await api<{ body: string }>('/tools/fetch-sub', {
        method: 'POST',
        body: JSON.stringify({ url: trimmed }),
      })
      return extractShareLinks(data.body || '')
    }
    return extractShareLinks(trimmed)
  }

  async function importPaste() {
    setError('')
    setBusy(true)
    try {
      const links = await resolvePasteToLinks(paste)
      if (links.length === 0) {
        throw new Error('No share links found (vmess/vless/trojan/ss/…)')
      }

      if (links.length === 1) {
        const parsed = parseOutboundLink(links[0])
        if (!parsed) throw new Error('Failed to parse share link')
        const next = applyParsedOutboundLink(parsed, form.tag)
        if (!OUTBOUND_PROTOCOLS.includes(next.protocol)) {
          throw new Error(`Unsupported protocol: ${next.protocol}`)
        }
        setForm(next)
        setTab('basic')
        return
      }

      const errors: string[] = []
      let created = 0
      for (let i = 0; i < links.length; i++) {
        const tag = `link-${i + 1}`
        const parsed = parseOutboundLink(links[i])
        if (!parsed) {
          errors.push(`${tag}: parse failed`)
          continue
        }
        const f = applyParsedOutboundLink(parsed, tag)
        if (!f.tag) f.tag = tag
        if (!OUTBOUND_PROTOCOLS.includes(f.protocol)) {
          errors.push(`${f.tag}: unsupported protocol ${f.protocol}`)
          continue
        }
        try {
          await api('/outbounds', {
            method: 'POST',
            body: JSON.stringify({
              tag: f.tag,
              protocol: f.protocol,
              enable: f.enable,
              remark: f.remark || f.tag,
              settings: buildOutboundSettings(f),
              streamSettings: buildOutboundStream(f),
            }),
          })
          created++
        } catch (err) {
          errors.push(`${f.tag}: ${err instanceof Error ? err.message : 'error'}`)
        }
      }
      if (created === 0) {
        throw new Error(errors.join('; ') || 'Import failed')
      }
      if (errors.length) {
        setError(`Imported ${created}/${links.length}. Errors: ${errors.join('; ')}`)
        onSaved()
        return
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'import error')
    } finally {
      setBusy(false)
    }
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const payload = {
        tag: form.tag,
        protocol: form.protocol,
        enable: form.enable,
        remark: form.remark,
        settings: buildOutboundSettings(form),
        streamSettings: buildOutboundStream(form),
      }
      if (mode === 'edit' && form.id) {
        await api(`/outbounds/${form.id}`, { method: 'PUT', body: JSON.stringify({ ...payload, id: form.id }) })
      } else {
        await api('/outbounds', { method: 'POST', body: JSON.stringify(payload) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  if (!open) return null

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <form className="modal modal--lg" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} outbound</h3>

        <div className="modal-body-scroll">
          <div className="form-section">
            <div className="form-section__title">Paste link / subscription</div>
            <div className="form-section__subtitle">vless://… or multi-link / base64 body / https://…sub</div>
            <div className="form-section__body">
              <FormRow stack>
                <textarea
                  className="textarea"
                  rows={3}
                  value={paste}
                  onChange={(e) => setPaste(e.target.value)}
                  placeholder="vless://… or multi-link / base64 body / https://…sub"
                />
              </FormRow>
              <div className="row-actions">
                <button type="button" className="btn secondary btn-sm" disabled={busy || !paste.trim()} onClick={() => { void importPaste() }}>
                  Import
                </button>
              </div>
            </div>
          </div>

          <div className="tabs">
            <button type="button" className={`tab ${tab === 'basic' ? 'active' : ''}`} onClick={() => setTab('basic')}>{tr('tabGeneral')}</button>
            {isProxy && (
              <button type="button" className={`tab ${tab === 'stream' ? 'active' : ''}`} onClick={() => setTab('stream')}>{tr('tabNetwork')} / {tr('tabSecurity')}</button>
            )}
          </div>

          {tab === 'basic' && (
            <div>
              <FormRow label="Tag *">
                <input className="input" value={form.tag} onChange={(e) => set('tag', e.target.value)} required disabled={mode === 'edit' && ['direct', 'blocked'].includes(form.tag)} />
              </FormRow>
              <FormRow label={tr('protocol')}>
                <select className="select" value={form.protocol} onChange={(e) => set('protocol', e.target.value)} disabled={mode === 'edit' && ['direct', 'blocked'].includes(form.tag)}>
                  {OUTBOUND_PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
                </select>
              </FormRow>
              <FormRow label={tr('remark')}>
                <input className="input" value={form.remark} onChange={(e) => set('remark', e.target.value)} />
              </FormRow>
              <FormRow label={tr('enable')}>
                <label className="form-switch">
                  <input type="checkbox" checked={form.enable} onChange={(e) => set('enable', e.target.checked)} />
                  <span>{form.enable ? 'On' : 'Off'}</span>
                </label>
              </FormRow>

              {form.protocol === 'freedom' && (
                <div className="form-section">
                  <div className="form-section__title">Freedom</div>
                  <div className="form-section__body">
                    <FormRow label="domainStrategy">
                      <select className="select" value={form.domainStrategy} onChange={(e) => set('domainStrategy', e.target.value)}>
                        <option value="AsIs">AsIs</option>
                        <option value="UseIP">UseIP</option>
                        <option value="UseIPv4">UseIPv4</option>
                        <option value="UseIPv6">UseIPv6</option>
                      </select>
                    </FormRow>
                  </div>
                </div>
              )}

              {isDns && (
                <div className="form-section">
                  <div className="form-section__title">DNS</div>
                  <div className="form-section__body">
                    <FormRow label="Address">
                      <input className="input" value={form.address} onChange={(e) => set('address', e.target.value)} placeholder="optional (e.g. 8.8.8.8)" />
                    </FormRow>
                    <FormRow label={tr('port')}>
                      <input className="input input-number--compact" type="number" value={form.port} onChange={(e) => set('port', Number(e.target.value))} />
                    </FormRow>
                    <FormRow label={tr('network')}>
                      <select className="select" value={form.dnsNetwork} onChange={(e) => set('dnsNetwork', e.target.value)}>
                        <option value="udp">udp</option>
                        <option value="tcp">tcp</option>
                      </select>
                    </FormRow>
                    <FormRow label="block">
                      <label className="form-switch">
                        <input type="checkbox" checked={form.dnsBlock} onChange={(e) => set('dnsBlock', e.target.checked)} />
                        <span>{form.dnsBlock ? 'On' : 'Off'}</span>
                      </label>
                    </FormRow>
                  </div>
                </div>
              )}

              {isProxy && (
                <div className="form-section">
                  <div className="form-section__title">Server</div>
                  <div className="form-section__body">
                    <FormRow label="Address *">
                      <input className="input" value={form.address} onChange={(e) => set('address', e.target.value)} required />
                    </FormRow>
                    <FormRow label={tr('port')}>
                      <input className="input input-number--compact" type="number" value={form.port} onChange={(e) => set('port', Number(e.target.value))} required />
                    </FormRow>
                    {['vless', 'vmess'].includes(form.protocol) && (
                      <FormRow label="UUID">
                        <div className="input-compact">
                          <input className="input" value={form.uuid} onChange={(e) => set('uuid', e.target.value)} />
                          <button type="button" className="btn secondary btn-sm" onClick={() => set('uuid', randomUUID())}>↻</button>
                        </div>
                      </FormRow>
                    )}
                    {form.protocol === 'vless' && (
                      <FormRow label="Flow">
                        <select className="select" value={form.flow} onChange={(e) => set('flow', e.target.value)}>
                          <option value="">(none)</option>
                          <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                        </select>
                      </FormRow>
                    )}
                    {['trojan', 'shadowsocks', 'socks', 'http'].includes(form.protocol) && (
                      <FormRow label={tr('password')}>
                        <input className="input" value={form.password} onChange={(e) => set('password', e.target.value)} />
                      </FormRow>
                    )}
                    {form.protocol === 'shadowsocks' && (
                      <FormRow label="Method">
                        <input className="input" value={form.method} onChange={(e) => set('method', e.target.value)} />
                      </FormRow>
                    )}
                    {['socks', 'http'].includes(form.protocol) && (
                      <FormRow label="User">
                        <input className="input" value={form.email} onChange={(e) => set('email', e.target.value)} />
                      </FormRow>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}

          {tab === 'stream' && isProxy && (
            <div>
              <FormRow label={tr('network')}>
                <select className="select" value={form.network} onChange={(e) => set('network', e.target.value)}>
                  {NETWORKS.map((n) => <option key={n} value={n}>{n}</option>)}
                </select>
              </FormRow>
              <FormRow label={tr('security')}>
                <div className="seg">
                  {SECURITIES.map((s) => (
                    <button
                      key={s}
                      type="button"
                      className={form.security === s ? 'active' : ''}
                      onClick={() => set('security', s)}
                    >
                      {s}
                    </button>
                  ))}
                </div>
              </FormRow>

              {form.network === 'ws' && (
                <div className="form-section">
                  <div className="form-section__title">WebSocket</div>
                  <div className="form-section__body">
                    <FormRow label="Path">
                      <input className="input" value={form.path} onChange={(e) => set('path', e.target.value)} />
                    </FormRow>
                    <FormRow label="Host">
                      <input className="input" value={form.host} onChange={(e) => set('host', e.target.value)} />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.network === 'grpc' && (
                <div className="form-section">
                  <div className="form-section__title">gRPC</div>
                  <div className="form-section__body">
                    <FormRow label="Service name">
                      <input className="input" value={form.serviceName} onChange={(e) => set('serviceName', e.target.value)} />
                    </FormRow>
                  </div>
                </div>
              )}

              {form.security !== 'none' && (
                <div className="form-section">
                  <div className="form-section__title">{form.security === 'reality' ? 'REALITY' : 'TLS'}</div>
                  <div className="form-section__body">
                    <FormRow label="SNI">
                      <input className="input" value={form.sni} onChange={(e) => set('sni', e.target.value)} />
                    </FormRow>
                    {form.security === 'reality' && (
                      <>
                        <FormRow label="Public key">
                          <input className="input" value={form.publicKey} onChange={(e) => set('publicKey', e.target.value)} />
                        </FormRow>
                        <FormRow label="Short ID">
                          <input className="input" value={form.shortId} onChange={(e) => set('shortId', e.target.value)} />
                        </FormRow>
                      </>
                    )}
                    <FormRow label="Fingerprint">
                      <select className="select" value={form.fingerprint} onChange={(e) => set('fingerprint', e.target.value)}>
                        {FINGERPRINTS.map((fp) => <option key={fp} value={fp}>{fp}</option>)}
                      </select>
                    </FormRow>
                  </div>
                </div>
              )}
            </div>
          )}
        </div>

        {error && <p className="error">{error}</p>}
        <div className="form-actions">
          <div className="form-actions__end">
            <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
            <button className="btn" type="submit" disabled={busy}>{busy ? '…' : tr('save')}</button>
          </div>
        </div>
      </form>
    </div>
  )
}
