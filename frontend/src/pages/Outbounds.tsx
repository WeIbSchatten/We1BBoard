import { FormEvent, useEffect, useState } from 'react'
import { api, type Outbound, type OutboundSubscription } from '../api'
import { useApp } from '../AppContext'
import { OutboundFormModal } from '../components/OutboundFormModal'

type SubForm = {
  remark: string
  url: string
  enable: boolean
  prefix: string
  intervalMin: number
}

const emptySub = (): SubForm => ({
  remark: '',
  url: '',
  enable: true,
  prefix: 'sub-',
  intervalMin: 0,
})

type LatencyResult = { id: number; tag: string; ok: boolean; latencyMs: number; error?: string }

export function OutboundsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Outbound[]>([])
  const [subs, setSubs] = useState<OutboundSubscription[]>([])
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; outbound: Outbound | null }>({
    open: false, mode: 'add', outbound: null,
  })
  const [subForm, setSubForm] = useState<SubForm>(emptySub())
  const [editSub, setEditSub] = useState<OutboundSubscription | null>(null)
  const [latency, setLatency] = useState<Record<number, LatencyResult>>({})
  const [msg, setMsg] = useState('')
  const [busy, setBusy] = useState(false)

  function isProxy(o: Outbound) {
    if (['direct', 'blocked', 'block', 'blackhole'].includes(o.tag)) return false
    return !['freedom', 'blackhole', 'dns', 'block'].includes(o.protocol)
  }

  async function load() {
    const [o, s] = await Promise.all([
      api<Outbound[]>('/outbounds'),
      api<OutboundSubscription[]>('/outbound-subs'),
    ])
    setRows(o)
    setSubs(s || [])
  }
  useEffect(() => { load().catch(console.error) }, [])

  async function remove(id: number) {
    if (!confirm('Delete outbound?')) return
    await api(`/outbounds/${id}`, { method: 'DELETE' })
    await load()
  }

  async function addWarp() {
    setBusy(true)
    setMsg('')
    try {
      const data = await api<{
        outbound: Outbound
        note: string
      }>('/xray/warp/generate', { method: 'POST' })
      const ob = data.outbound
      let tag = ob.tag || 'warp'
      const existing = new Set(rows.map((r) => r.tag))
      if (existing.has(tag)) {
        let i = 2
        while (existing.has(`${tag}-${i}`)) i++
        tag = `${tag}-${i}`
      }
      await api('/outbounds', {
        method: 'POST',
        body: JSON.stringify({
          tag,
          protocol: 'wireguard',
          enable: true,
          remark: ob.remark || 'Cloudflare WARP (placeholder)',
          settings: ob.settings,
          streamSettings: '',
        }),
      })
      setMsg(data.note || 'WARP placeholder added — fill address / peer key / reserved from warp-cli')
      await load()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function saveSub(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setMsg('')
    try {
      if (editSub) {
        await api(`/outbound-subs/${editSub.id}`, {
          method: 'PUT',
          body: JSON.stringify({ ...editSub, ...subForm }),
        })
      } else {
        await api('/outbound-subs', {
          method: 'POST',
          body: JSON.stringify(subForm),
        })
      }
      setEditSub(null)
      setSubForm(emptySub())
      await load()
    } catch (err) {
      setMsg(err instanceof Error ? err.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function refreshSub(id: number) {
    setBusy(true)
    setMsg('')
    try {
      const r = await api<{ count: number }>(`/outbound-subs/${id}/refresh`, { method: 'POST' })
      setMsg(`Refreshed ${r.count} outbounds`)
      await load()
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
      await load()
    } finally {
      setBusy(false)
    }
  }

  async function deleteSub(id: number) {
    if (!confirm('Delete subscription?')) return
    await api(`/outbound-subs/${id}`, { method: 'DELETE' })
    await load()
  }

  function startEdit(s: OutboundSubscription) {
    setEditSub(s)
    setSubForm({
      remark: s.remark || '',
      url: s.url,
      enable: s.enable,
      prefix: s.prefix || 'sub-',
      intervalMin: s.intervalMin || 0,
    })
  }

  async function testOne(id: number) {
    setBusy(true)
    setMsg('')
    try {
      const r = await api<LatencyResult>(`/outbounds/${id}/test`, { method: 'POST' })
      setLatency((prev) => ({ ...prev, [id]: r }))
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  async function testAll() {
    setBusy(true)
    setMsg('')
    try {
      const data = await api<{ results: LatencyResult[] }>('/outbounds/test-all', { method: 'POST' })
      const map: Record<number, LatencyResult> = {}
      for (const r of data.results || []) map[r.id] = r
      setLatency(map)
    } catch (e) {
      setMsg(e instanceof Error ? e.message : 'error')
    } finally {
      setBusy(false)
    }
  }

  function latencyBadge(id: number) {
    const r = latency[id]
    if (!r) return null
    if (r.ok) {
      return <span className="badge on">{Math.round(r.latencyMs)} ms</span>
    }
    return <span className="badge off" title={r.error || ''}>{tr('testFail')}</span>
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('outbounds')}</h1>
          <p className="page-sub">{tr('outboundsHint')}</p>
        </div>
        <div className="row-actions">
          <button className="btn secondary" type="button" disabled={busy} onClick={() => void testAll()}>
            {tr('testAll')}
          </button>
          <button className="btn secondary" type="button" disabled={busy} onClick={() => void addWarp()}>
            {tr('addWarp')}
          </button>
          <button className="btn" type="button" onClick={() => setModal({ open: true, mode: 'add', outbound: null })}>
            {tr('create')}
          </button>
        </div>
      </div>
      {msg && <p className="page-sub" style={{ marginBottom: 10 }}>{msg}</p>}

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>Tag</th>
              <th>{tr('protocol')}</th>
              <th>{tr('remark')}</th>
              <th>{tr('status')}</th>
              <th>{tr('latency')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 && <tr><td colSpan={6}>{tr('empty')}</td></tr>}
            {rows.map((o) => (
              <tr key={o.id}>
                <td><code style={{ fontFamily: 'var(--mono)' }}>{o.tag}</code></td>
                <td><span className="badge">{o.protocol}</span></td>
                <td>{o.remark || '—'}</td>
                <td><span className={`badge ${o.enable ? 'on' : 'off'}`}>{o.enable ? tr('enable') : tr('disable')}</span></td>
                <td>{latencyBadge(o.id) || '—'}</td>
                <td className="row-actions">
                  {isProxy(o) && (
                    <button className="btn secondary" type="button" disabled={busy} onClick={() => void testOne(o.id)}>{tr('test')}</button>
                  )}
                  <button className="btn secondary" type="button" onClick={() => setModal({ open: true, mode: 'edit', outbound: o })}>{tr('edit')}</button>
                  {!['direct', 'blocked'].includes(o.tag) && (
                    <button className="btn danger" type="button" onClick={() => remove(o.id)}>{tr('delete')}</button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="card" style={{ marginTop: 16 }}>
        <h3 style={{ marginTop: 0 }}>{tr('outboundSubs')}</h3>
        <p className="page-sub" style={{ marginTop: 0 }}>{tr('outboundSubsHint')}</p>

        <form onSubmit={saveSub} style={{ marginBottom: 16 }}>
          <div className="grid2">
            <div className="field">
              <label className="label">{tr('remark')}</label>
              <input className="input" value={subForm.remark} onChange={(e) => setSubForm({ ...subForm, remark: e.target.value })} />
            </div>
            <div className="field">
              <label className="label">Prefix</label>
              <input className="input" value={subForm.prefix} onChange={(e) => setSubForm({ ...subForm, prefix: e.target.value })} placeholder="sub-" />
            </div>
            <div className="field" style={{ gridColumn: '1 / -1' }}>
              <label className="label">URL</label>
              <input className="input" value={subForm.url} required onChange={(e) => setSubForm({ ...subForm, url: e.target.value })} placeholder="https://…" />
            </div>
            <div className="field">
              <label className="label">Interval (min, 0=manual)</label>
              <input
                className="input"
                type="number"
                min={0}
                value={subForm.intervalMin}
                onChange={(e) => setSubForm({ ...subForm, intervalMin: Number(e.target.value) || 0 })}
              />
            </div>
            <div className="field">
              <label className="label" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <input type="checkbox" checked={subForm.enable} onChange={(e) => setSubForm({ ...subForm, enable: e.target.checked })} />
                {tr('enable')}
              </label>
            </div>
          </div>
          <div className="row-actions">
            <button className="btn" type="submit" disabled={busy}>
              {editSub ? tr('save') : tr('create')}
            </button>
            {editSub && (
              <button className="btn secondary" type="button" onClick={() => { setEditSub(null); setSubForm(emptySub()) }}>
                {tr('cancel')}
              </button>
            )}
          </div>
        </form>

        <table className="table">
          <thead>
            <tr>
              <th>{tr('remark')}</th>
              <th>URL</th>
              <th>Prefix</th>
              <th>Interval</th>
              <th>{tr('status')}</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {subs.length === 0 && <tr><td colSpan={6}>{tr('empty')}</td></tr>}
            {subs.map((s) => (
              <tr key={s.id}>
                <td>{s.remark || '—'}</td>
                <td style={{ maxWidth: 220, overflow: 'hidden', textOverflow: 'ellipsis' }}>
                  <code style={{ fontFamily: 'var(--mono)', fontSize: 12 }}>{s.url}</code>
                </td>
                <td><code style={{ fontFamily: 'var(--mono)' }}>{s.prefix}</code></td>
                <td>{s.intervalMin || 'manual'}</td>
                <td>
                  <span className={`badge ${s.enable ? 'on' : 'off'}`}>{s.enable ? tr('enable') : tr('disable')}</span>
                  {s.lastError && <div className="page-sub" style={{ color: 'var(--danger)' }}>{s.lastError}</div>}
                </td>
                <td className="row-actions">
                  <button className="btn secondary" type="button" disabled={busy} onClick={() => void refreshSub(s.id)}>{tr('refresh')}</button>
                  <button className="btn secondary" type="button" onClick={() => startEdit(s)}>{tr('edit')}</button>
                  <button className="btn danger" type="button" onClick={() => void deleteSub(s.id)}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <OutboundFormModal
        open={modal.open}
        mode={modal.mode}
        outbound={modal.outbound}
        onClose={() => setModal({ open: false, mode: 'add', outbound: null })}
        onSaved={() => { void load() }}
      />
    </div>
  )
}
