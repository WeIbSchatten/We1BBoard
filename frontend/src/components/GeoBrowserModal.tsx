import { useEffect, useMemo, useState } from 'react'
import { api } from '../api'
import { useApp } from '../AppContext'

type Props = {
  open: boolean
  kind: 'geosite' | 'geoip'
  onClose: () => void
  onPick: (value: string) => void
}

type ListResp = {
  type: string
  tags: string[]
  source: string
  path?: string
  note?: string
}

export function GeoBrowserModal({ open, kind, onClose, onPick }: Props) {
  const { tr } = useApp()
  const [tags, setTags] = useState<string[]>([])
  const [source, setSource] = useState('')
  const [q, setQ] = useState('')
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (!open) return
    setQ('')
    setBusy(true)
    api<ListResp>(`/geodata/list?type=${kind}`)
      .then((d) => {
        setTags(d.tags || [])
        setSource(d.source || 'static')
      })
      .catch(() => {
        setTags([])
        setSource('')
      })
      .finally(() => setBusy(false))
  }, [open, kind])

  const filtered = useMemo(() => {
    const needle = q.trim().toLowerCase()
    if (!needle) return tags
    return tags.filter((t) => t.toLowerCase().includes(needle))
  }, [tags, q])

  if (!open) return null

  const prefix = kind === 'geosite' ? 'geosite:' : 'geoip:'

  return (
    <div className="modal-backdrop" onClick={onClose} style={{ zIndex: 60 }}>
      <div className="modal" style={{ width: 'min(480px, 100%)' }} onClick={(e) => e.stopPropagation()}>
        <h3>{tr('geoBrowser')} — {kind}</h3>
        <p className="page-sub" style={{ marginTop: 0 }}>
          {source ? `source: ${source}` : ''} · click to insert chip
        </p>
        <input
          className="input"
          autoFocus
          placeholder={tr('search')}
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
        <div className="geo-list">
          {busy && <p className="page-sub">…</p>}
          {!busy && filtered.length === 0 && <p className="page-sub">{tr('empty')}</p>}
          {filtered.map((tag) => (
            <button
              key={tag}
              type="button"
              className="geo-item"
              onClick={() => {
                onPick(`${prefix}${tag}`)
                onClose()
              }}
            >
              <code>{prefix}{tag}</code>
            </button>
          ))}
        </div>
        <div className="row-actions" style={{ marginTop: 12 }}>
          <button className="btn secondary" type="button" onClick={onClose}>{tr('cancel')}</button>
        </div>
        <style>{`
          .geo-list {
            margin-top: 10px;
            max-height: min(50vh, 360px);
            overflow: auto;
            display: flex;
            flex-direction: column;
            gap: 4px;
            border: 1px solid var(--border);
            border-radius: 12px;
            padding: 6px;
          }
          .geo-item {
            text-align: left;
            border: none;
            background: transparent;
            color: var(--text);
            padding: 0.45rem 0.65rem;
            border-radius: 8px;
            cursor: pointer;
            font-size: 0.85rem;
          }
          .geo-item:hover {
            background: color-mix(in srgb, var(--accent) 12%, transparent);
          }
          .geo-item code { font-family: var(--mono); }
        `}</style>
      </div>
    </div>
  )
}
