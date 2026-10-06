import { useEffect, useState } from 'react'
import { api, type Host, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { ConfirmModal } from '../components/ConfirmModal'
import { HostFormModal, emptyHost } from '../components/HostFormModal'

export function HostsPage() {
  const { tr } = useApp()
  const [rows, setRows] = useState<Host[]>([])
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [modal, setModal] = useState<{ open: boolean; mode: 'add' | 'edit'; host: Host | null }>({
    open: false, mode: 'add', host: null,
  })
  const [confirmId, setConfirmId] = useState<number | null>(null)

  async function load() {
    const [hosts, ib] = await Promise.all([
      api<Host[]>('/hosts'),
      api<Inbound[]>('/inbounds'),
    ])
    setRows(hosts || [])
    setInbounds(ib || [])
  }

  useEffect(() => { load().catch(console.error) }, [])

  async function remove(id: number) {
    await api(`/hosts/${id}`, { method: 'DELETE' })
    await load()
  }

  async function toggleEnable(h: Host) {
    await api(`/hosts/${h.id}/enable`, { method: 'POST', body: JSON.stringify({ enable: !h.enable }) })
    await load()
  }

  function inboundLabel(h: Host) {
    if (h.inboundId > 0) {
      const ib = inbounds.find((i) => i.id === h.inboundId)
      return ib ? `#${ib.id} ${ib.remark || ib.tag}` : `#${h.inboundId}`
    }
    if (h.inboundTag) return `tag:${h.inboundTag}`
    return tr('allInbounds')
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('hosts')}</h1>
          <p className="page-sub">{tr('hostsHint')}</p>
        </div>
        <button className="btn" onClick={() => setModal({ open: true, mode: 'add', host: emptyHost() })}>{tr('create')}</button>
      </div>

      {rows.length === 0 ? (
        <div className="card">
          <div className="empty-state">
            <div>{tr('empty')}</div>
            <button type="button" className="btn" onClick={() => setModal({ open: true, mode: 'add', host: emptyHost() })}>
              {tr('create')}
            </button>
          </div>
        </div>
      ) : (
        <div className="card">
          <table className="table">
            <thead>
              <tr>
                <th>{tr('remark')}</th>
                <th>Address</th>
                <th>{tr('port')}</th>
                <th>Inbound</th>
                <th>SNI</th>
                <th>{tr('status')}</th>
                <th>{tr('actions')}</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((h) => (
                <tr key={h.id}>
                  <td>{h.remark || '—'}</td>
                  <td><code style={{ fontFamily: 'var(--mono)', fontSize: '0.85rem' }}>{h.address}</code></td>
                  <td>{h.port || '↓'}</td>
                  <td>{inboundLabel(h)}</td>
                  <td>{h.sni || '—'}</td>
                  <td>
                    <button className={`badge ${h.enable ? 'on' : 'off'}`} style={{ cursor: 'pointer', border: 'none' }} onClick={() => { void toggleEnable(h) }}>
                      {h.enable ? tr('enable') : tr('disable')}
                    </button>
                  </td>
                  <td className="row-actions">
                    <button className="btn btn-sm secondary" onClick={() => setModal({ open: true, mode: 'edit', host: h })}>{tr('edit')}</button>
                    <button className="btn btn-sm danger" onClick={() => setConfirmId(h.id)}>{tr('delete')}</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmModal
        open={confirmId != null}
        title={tr('confirmDeleteTitle')}
        message={tr('confirmDeleteHost')}
        danger
        onCancel={() => setConfirmId(null)}
        onConfirm={() => {
          const id = confirmId
          setConfirmId(null)
          if (id != null) void remove(id)
        }}
      />

      <HostFormModal
        open={modal.open}
        mode={modal.mode}
        host={modal.host}
        inbounds={inbounds}
        onClose={() => setModal({ open: false, mode: 'add', host: null })}
        onSaved={() => { void load() }}
      />
    </div>
  )
}
