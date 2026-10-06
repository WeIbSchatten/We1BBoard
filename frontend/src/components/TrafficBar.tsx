type Props = {
  used: number
  totalGB: number
}

export function TrafficBar({ used, totalGB }: Props) {
  const pct = totalGB > 0 ? Math.min(100, (used / totalGB) * 100) : 0
  const tone = pct >= 95 ? 'danger' : pct >= 80 ? 'warn' : ''
  const label =
    totalGB <= 0
      ? `${used.toFixed(2)} GB / ∞`
      : `${used.toFixed(2)} / ${totalGB} GB`

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 4, minWidth: 96 }}>
      {totalGB > 0 && (
        <div className={`traffic-bar${tone ? ` ${tone}` : ''}`} title={`${pct.toFixed(0)}%`}>
          <i style={{ width: `${pct}%` }} />
        </div>
      )}
      <span
        style={{
          fontFamily: 'var(--mono)',
          fontSize: '0.75rem',
          color: 'var(--text-muted)',
          whiteSpace: 'nowrap',
        }}
      >
        {label}
      </span>
    </div>
  )
}
