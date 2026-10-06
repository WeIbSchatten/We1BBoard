import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, type Client, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { inboundSupportsClients, parseInboundToForm, suggestedFlow } from '../lib/inboundForm'
import { randomLowerAndNum, randomUUID } from '../lib/random'
import { FormRow } from './FormRow'

type Props = {
  open: boolean
  mode: 'add' | 'edit'
  inbound: Inbound | null
  inbounds?: Inbound[]
  client: Client | null
  groupNames?: string[]
  onClose: () => void
  onSaved: () => void
}

type Tab = 'basic' | 'config' | 'links'

function freshEmail(): string {
  return `${randomLowerAndNum(8)}@we1b`
}

function parseSelectedIds(client: Client | null, fallback: number): number[] {
  if (client?.inboundIds) {
    const ids = client.inboundIds.split(',').map((x) => Number(x.trim())).filter((n) => n > 0)
    if (ids.length) return [...new Set(ids)]
  }
  if (client?.inboundId) return [client.inboundId]
  if (fallback) return [fallback]
  return []
}

export function ClientFormModal({ open, mode, inbound, inbounds, client, groupNames, onClose, onSaved }: Props) {
  const { tr } = useApp()
  const [tab, setTab] = useState<Tab>('basic')
  const [selectedInboundIds, setSelectedInboundIds] = useState<number[]>([])
  const [email, setEmail] = useState('')
  const [uuid, setUuid] = useState('')
  const [password, setPassword] = useState('')
  const [subId, setSubId] = useState('')
  const [group, setGroup] = useState('')
  const [flow, setFlow] = useState('')
  const [enable, setEnable] = useState(true)
  const [totalGB, setTotalGB] = useState(0)
  const [limitIp, setLimitIp] = useState(0)
  const [limitHwid, setLimitHwid] = useState(0)
  const [expiryDays, setExpiryDays] = useState(0)
  const [trafficReset, setTrafficReset] = useState('never')
  const [extraLinks, setExtraLinks] = useState('')
  const [tgId, setTgId] = useState(0)
  const [comment, setComment] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const eligibleInbounds = useMemo(
    () => (inbounds || []).filter((i) => inboundSupportsClients(i.protocol)),
    [inbounds],
  )

  const primaryId = inbound?.id || selectedInboundIds[0] || 0
  const selectedInbound = inbound || eligibleInbounds.find((i) => i.id === primaryId) || null
  const protocol = selectedInbound?.protocol || ''

  useEffect(() => {
    if (!open) return
    setTab('basic')
    setError('')
    if (mode === 'edit' && client) {
      setSelectedInboundIds(parseSelectedIds(client, inbound?.id || 0))
      setEmail(client.email)
      setUuid(client.uuid)
      setPassword(client.password || '')
      setSubId(client.subId || '')
      setGroup(client.group || '')
      setFlow(client.flow || '')
      setEnable(client.enable)
      setTotalGB(client.totalGB || 0)
      setLimitIp(client.limitIp || 0)
      setLimitHwid(client.limitHwid || 0)
      setComment(client.comment || '')
      setTgId(client.tgId || 0)
      setTrafficReset(client.trafficReset || 'never')
      setExtraLinks(client.extraLinks || '')
      if (client.expiryTime > 0) {
        setExpiryDays(Math.max(0, Math.ceil((client.expiryTime - Date.now()) / 86400000)))
      } else setExpiryDays(0)
    } else {
      const ib = inbound || eligibleInbounds[0] || null
      setSelectedInboundIds(ib ? [ib.id] : [])
      setEmail(freshEmail())
      setUuid(randomUUID())
      setPassword(randomLowerAndNum(16))
      setSubId(randomLowerAndNum(16))
      setGroup('')
      setEnable(true)
      setTotalGB(0)
      setLimitIp(0)
      setLimitHwid(0)
      setExpiryDays(0)
      setComment('')
      setTgId(0)
      setTrafficReset('never')
      setExtraLinks('')
      if (ib) {
        const f = parseInboundToForm(ib)
        setFlow(suggestedFlow(f))
      } else {
        setFlow('')
      }
    }
  }, [open, mode, client, inbound, eligibleInbounds])

  function toggleInbound(id: number) {
    setSelectedInboundIds((prev) => {
      if (prev.includes(id)) {
        const next = prev.filter((x) => x !== id)
        return next
      }
      const next = [...prev, id]
      const ib = eligibleInbounds.find((i) => i.id === id)
      if (ib && next.length === 1) setFlow(suggestedFlow(parseInboundToForm(ib)))
      return next
    })
  }

  async function submit(e: FormEvent) {
    e.preventDefault()
    setError('')
    const ids = mode === 'add' && inbound ? [inbound.id] : selectedInboundIds
    if (!ids.length) {
      setError('Select at least one inbound')
      return
    }
    if ((subId || '').length < 16) {
      setError('Sub ID must be at least 16 characters')
      return
    }
    setBusy(true)
    try {
      const body = {
        inboundId: ids[0],
        inboundIds: ids.join(','),
        email,
        uuid: uuid || undefined,
        password: password || undefined,
        subId: subId || undefined,
        group: group.trim(),
        flow,
        enable,
        totalGB,
        limitIp,
        limitHwid,
        trafficReset,
        extraLinks,
        comment,
        tgId,
        expiryTime: expiryDays > 0 ? Date.now() + expiryDays * 86400000 : 0,
      }
      if (mode === 'edit' && client) {
        await api(`/clients/${client.id}`, { method: 'PUT', body: JSON.stringify({ ...body, id: client.id }) })
      } else {
        await api('/clients', { method: 'POST', body: JSON.stringify(body) })
      }
      onSaved()
      onClose()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save client')
    } finally {
      setBusy(false)
    }
  }

  if (!open) return null

  return (
    <div className="modal-backdrop modal-backdrop--top" onClick={onClose}>
      <form className="modal modal--md" onClick={(e) => e.stopPropagation()} onSubmit={submit}>
        <h3>{mode === 'edit' ? tr('edit') : tr('create')} {tr('clients')}</h3>
        <div className="tabs">
          <button type="button" className={`tab ${tab === 'basic' ? 'active' : ''}`} onClick={() => setTab('basic')}>{tr('tabGeneral')}</button>
          <button type="button" className={`tab ${tab === 'config' ? 'active' : ''}`} onClick={() => setTab('config')}>{tr('tabConfig')}</button>
          <button type="button" className={`tab ${tab === 'links' ? 'active' : ''}`} onClick={() => setTab('links')}>Links</button>
        </div>

        <div className="modal-body-scroll">
          {tab === 'basic' && (
            <div>
              {!(mode === 'add' && inbound) && (
                <FormRow stack label="Inbounds (multi)">
                  <div className="check-list">
                    {eligibleInbounds.length === 0 && <span className="page-sub">No eligible inbounds</span>}
                    {eligibleInbounds.map((i) => (
                      <label key={i.id} style={{ display: 'flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
                        <input
                          type="checkbox"
                          checked={selectedInboundIds.includes(i.id)}
                          onChange={() => toggleInbound(i.id)}
                        />
                        <span>#{i.id} {i.remark || i.tag} ({i.protocol}:{i.port})</span>
                      </label>
                    ))}
                  </div>
                </FormRow>
              )}
              {mode === 'add' && inbound && (
                <FormRow stack label="Inbound">
                  <input className="input" readOnly value={`#${inbound.id} ${inbound.remark || inbound.tag} (${inbound.protocol}:${inbound.port})`} />
                </FormRow>
              )}

              <div className="form-grid">
                <FormRow stack label="Email">
                  <div className="input-compact">
                    <input className="input" value={email} onChange={(e) => setEmail(e.target.value)} required />
                    <button type="button" className="btn secondary btn-sm" onClick={() => setEmail(freshEmail())}>↻</button>
                  </div>
                </FormRow>
                <FormRow stack label="Total GB" hint="0 = unlimited">
                  <input className="input" type="number" min={0} value={totalGB} onChange={(e) => setTotalGB(Number(e.target.value))} />
                </FormRow>
                <FormRow stack label="Limit IP" hint="0 = unlimited">
                  <input className="input" type="number" min={0} value={limitIp} onChange={(e) => setLimitIp(Number(e.target.value))} />
                </FormRow>
                <FormRow stack label="Limit HWID" hint="0 = unlimited">
                  <input className="input" type="number" min={0} value={limitHwid} onChange={(e) => setLimitHwid(Number(e.target.value))} />
                </FormRow>
                <FormRow stack label="Expiry days" hint="0 = never">
                  <input className="input" type="number" min={0} value={expiryDays} onChange={(e) => setExpiryDays(Number(e.target.value))} />
                </FormRow>
                <FormRow stack label="Traffic reset">
                  <select className="select" value={trafficReset} onChange={(e) => setTrafficReset(e.target.value)}>
                    <option value="never">never</option>
                    <option value="daily">daily</option>
                    <option value="weekly">weekly</option>
                    <option value="monthly">monthly</option>
                  </select>
                </FormRow>
                <FormRow stack label="Telegram ID">
                  <input className="input" type="number" value={tgId} onChange={(e) => setTgId(Number(e.target.value))} />
                </FormRow>
                <FormRow stack label="Comment">
                  <input className="input" value={comment} onChange={(e) => setComment(e.target.value)} />
                </FormRow>
                <FormRow stack label={tr('group')}>
                  <input
                    className="input"
                    list="client-group-names"
                    value={group}
                    onChange={(e) => setGroup(e.target.value)}
                    placeholder={tr('groupName')}
                  />
                  <datalist id="client-group-names">
                    {(groupNames || []).map((g) => (
                      <option key={g} value={g} />
                    ))}
                  </datalist>
                </FormRow>
                <FormRow stack label={tr('enable')}>
                  <label className="form-switch">
                    <input type="checkbox" checked={enable} onChange={(e) => setEnable(e.target.checked)} />
                    <span>{enable ? 'Enabled' : 'Disabled'}</span>
                  </label>
                </FormRow>
              </div>
            </div>
          )}

          {tab === 'config' && (
            <div>
              <FormRow stack label="UUID">
                <div className="input-compact">
                  <input className="input" value={uuid} onChange={(e) => setUuid(e.target.value)} />
                  <button type="button" className="btn secondary btn-sm" onClick={() => setUuid(randomUUID())}>↻</button>
                </div>
              </FormRow>
              <FormRow stack label="Sub ID" hint="At least 16 characters">
                <div className="input-compact">
                  <input className="input" value={subId} onChange={(e) => setSubId(e.target.value)} />
                  <button type="button" className="btn secondary btn-sm" onClick={() => setSubId(randomLowerAndNum(16))}>↻</button>
                </div>
              </FormRow>
              <FormRow stack label={tr('password')}>
                <div className="input-compact">
                  <input className="input" value={password} onChange={(e) => setPassword(e.target.value)} />
                  <button type="button" className="btn secondary btn-sm" onClick={() => setPassword(randomLowerAndNum(16))}>↻</button>
                </div>
              </FormRow>
              {(protocol === 'vless' || !protocol) && (
                <FormRow stack label="Flow">
                  <select className="select" value={flow} onChange={(e) => setFlow(e.target.value)}>
                    <option value="">(none)</option>
                    <option value="xtls-rprx-vision">xtls-rprx-vision</option>
                    <option value="xtls-rprx-vision-udp443">xtls-rprx-vision-udp443</option>
                  </select>
                </FormRow>
              )}
            </div>
          )}

          {tab === 'links' && (
            <FormRow stack label="Extra share links" hint="One per line — appended to subscription">
              <textarea
                className="textarea"
                rows={8}
                value={extraLinks}
                onChange={(e) => setExtraLinks(e.target.value)}
                placeholder={'vless://...\nss://...'}
              />
            </FormRow>
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
