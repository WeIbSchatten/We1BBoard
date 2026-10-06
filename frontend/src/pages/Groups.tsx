import { FormEvent, useEffect, useMemo, useState } from 'react'
import { api, type Client, type GroupSummary, type Inbound } from '../api'
import { useApp } from '../AppContext'
import { ConfirmModal } from '../components/ConfirmModal'

type ConfirmState =
  | { kind: 'delete'; name: string }
  | { kind: 'reset'; name: string }

export function GroupsPage() {
  const { tr } = useApp()
  const [groups, setGroups] = useState<GroupSummary[]>([])
  const [inbounds, setInbounds] = useState<Inbound[]>([])
  const [assignOpen, setAssignOpen] = useState(false)
  const [assignName, setAssignName] = useState('')
  const [selected, setSelected] = useState<number[]>([])
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)

  async function load() {
    const [g, ib] = await Promise.all([
      api<GroupSummary[]>('/clients/groups'),
      api<Inbound[]>('/inbounds'),
    ])
    setGroups(g || [])
    setInbounds(ib || [])
  }

  useEffect(() => { load().catch(console.error) }, [])

  const clients: Client[] = useMemo(() => {
    const byId = new Map<number, Client>()
    for (const ib of inbounds) {
      for (const c of ib.clients || []) {
        if (!byId.has(c.id)) byId.set(c.id, c)
      }
    }
    return [...byId.values()].sort((a, b) => a.email.localeCompare(b.email))
  }, [inbounds])

  async function createGroup() {
    const name = prompt(tr('groupName'))
    if (!name?.trim()) return
    await api('/clients/groups', { method: 'POST', body: JSON.stringify({ name: name.trim() }) })
    await load()
  }

  async function renameGroup(oldName: string) {
    const newName = prompt(tr('renameGroup'), oldName)
    if (!newName?.trim() || newName.trim() === oldName) return
    await api('/clients/groups/rename', { method: 'POST', body: JSON.stringify({ oldName, newName: newName.trim() }) })
    await load()
  }

  async function deleteGroup(name: string) {
    await api(`/clients/groups/${encodeURIComponent(name)}`, { method: 'DELETE' })
    await load()
  }

  async function resetTraffic(name: string) {
    await api('/clients/groups/reset-traffic', { method: 'POST', body: JSON.stringify({ name }) })
    await load()
  }

  function openAssign(name: string) {
    setAssignName(name)
    const already = clients.filter((c) => (c.group || '') === name).map((c) => c.id)
    setSelected(already)
    setAssignOpen(true)
  }

  async function submitAssign(e: FormEvent) {
    e.preventDefault()
    const inGroup = clients.filter((c) => (c.group || '') === assignName).map((c) => c.id)
    const toAdd = selected.filter((id) => !inGroup.includes(id))
    const toRemove = inGroup.filter((id) => !selected.includes(id))
    if (toAdd.length) {
      await api('/clients/groups/assign', { method: 'POST', body: JSON.stringify({ name: assignName, ids: toAdd }) })
    }
    if (toRemove.length) {
      await api('/clients/groups/unassign', { method: 'POST', body: JSON.stringify({ ids: toRemove }) })
    }
    setAssignOpen(false)
    await load()
  }

  return (
    <div>
      <div className="page-head">
        <div>
          <h1 className="page-title">{tr('groups')}</h1>
          <p className="page-sub">{tr('groupsHint')}</p>
        </div>
        <button className="btn" onClick={() => { void createGroup() }}>{tr('create')}</button>
      </div>

      <div className="card">
        <table className="table">
          <thead>
            <tr>
              <th>{tr('group')}</th>
              <th>{tr('clients')}</th>
              <th>Up</th>
              <th>Down</th>
              <th>{tr('actions')}</th>
            </tr>
          </thead>
          <tbody>
            {groups.length === 0 && <tr><td colSpan={5}>{tr('empty')}</td></tr>}
            {groups.map((g) => (
              <tr key={g.name}>
                <td>{g.name}</td>
                <td>{g.count}</td>
                <td>{(g.up / (1024 * 1024 * 1024)).toFixed(2)} GB</td>
                <td>{(g.down / (1024 * 1024 * 1024)).toFixed(2)} GB</td>
                <td className="row-actions">
                  <button className="btn btn-sm secondary" onClick={() => openAssign(g.name)}>{tr('assignClients')}</button>
                  <button className="btn btn-sm secondary" onClick={() => { void renameGroup(g.name) }}>{tr('rename')}</button>
                  <button className="btn btn-sm secondary" onClick={() => setConfirm({ kind: 'reset', name: g.name })}>{tr('resetTraffic')}</button>
                  <button className="btn btn-sm danger" onClick={() => setConfirm({ kind: 'delete', name: g.name })}>{tr('delete')}</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {assignOpen && (
        <div className="modal-backdrop" onClick={() => setAssignOpen(false)}>
          <form className="modal" style={{ width: 'min(560px, 100%)' }} onClick={(e) => e.stopPropagation()} onSubmit={submitAssign}>
            <h3>{tr('assignClients')} — {assignName}</h3>
            <div className="field">
              <label className="label">
                <input
                  type="checkbox"
                  checked={selected.length === clients.length && clients.length > 0}
                  onChange={(e) => setSelected(e.target.checked ? clients.map((c) => c.id) : [])}
                />{' '}
                {tr('clients')} ({selected.length}/{clients.length})
              </label>
              <div className="check-list" style={{ maxHeight: 280 }}>
                {clients.map((c) => (
                  <label key={c.id} style={{ display: 'flex', gap: 8, alignItems: 'center', cursor: 'pointer' }}>
                    <input
                      type="checkbox"
                      checked={selected.includes(c.id)}
                      onChange={(e) => {
                        setSelected((prev) => e.target.checked ? [...prev, c.id] : prev.filter((x) => x !== c.id))
                      }}
                    />
                    <span>{c.email}</span>
                    {c.group && c.group !== assignName && <span className="badge">{c.group}</span>}
                  </label>
                ))}
              </div>
            </div>
            <div className="modal-footer">
              <button className="btn secondary btn-sm" type="button" onClick={() => setAssignOpen(false)}>{tr('cancel')}</button>
              <button className="btn btn-sm" type="submit">{tr('save')}</button>
            </div>
          </form>
        </div>
      )}

      <ConfirmModal
        open={!!confirm}
        title={confirm?.kind === 'reset' ? tr('confirmResetTitle') : tr('confirmDeleteTitle')}
        message={
          confirm?.kind === 'reset'
            ? `${tr('confirmResetGroup')} «${confirm.name}»`
            : `${tr('confirmDeleteGroup')} «${confirm?.name || ''}»`
        }
        confirmLabel={confirm?.kind === 'reset' ? tr('resetTraffic') : tr('delete')}
        danger={confirm?.kind === 'delete'}
        onCancel={() => setConfirm(null)}
        onConfirm={() => {
          const c = confirm
          setConfirm(null)
          if (!c) return
          if (c.kind === 'delete') void deleteGroup(c.name)
          else void resetTraffic(c.name)
        }}
      />
    </div>
  )
}
