import { ReactNode } from 'react'
import type { PaginationMeta } from '../api/client'
import Pagination from './Pagination'

interface Column {
  key: string
  header: string
  render?: (item: any) => ReactNode
  width?: string
  align?: 'left' | 'center' | 'right'
  hideMobile?: boolean
}

interface DataTableProps {
  columns: Column[]
  data: any[]
  meta: PaginationMeta
  loading: boolean
  onPageChange: (page: number) => void
  onRowClick?: (item: any) => void
  emptyMessage?: string
}

export default function DataTable({
  columns, data, meta, loading, onPageChange, onRowClick, emptyMessage,
}: DataTableProps) {
  const page = Math.floor(meta.offset / meta.limit)
  const totalPages = Math.ceil(meta.total / meta.limit)

  if (loading) {
    return (
      <div className="card" style={{ overflow: 'hidden' }}>
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} style={{ display: 'flex', gap: '1rem', padding: '0.75rem', borderBottom: i < 4 ? '1px solid var(--glass-border)' : 'none' }}>
            {columns.filter((c) => !c.hideMobile).map((col) => (
              <div key={col.key} className="skeleton" style={{ flex: 1, height: 16, width: col.width }} />
            ))}
          </div>
        ))}
      </div>
    )
  }

  if (data.length === 0) {
    return <div className="empty-state">{emptyMessage || 'Нет данных'}</div>
  }

  const visibleColumns = columns.filter((c) => !c.hideMobile)

  return (
    <div className="card" style={{ overflow: 'hidden', padding: 0 }}>
      <div style={{ overflowX: 'auto' }}>
        <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.875rem' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid var(--glass-border)' }}>
              {columns.map((col) => (
                !col.hideMobile && (
                  <th key={col.key} style={{
                    textAlign: col.align || 'left', padding: '0.75rem 1rem',
                    fontWeight: 600, color: 'var(--text-muted)', fontSize: '0.8rem',
                    textTransform: 'uppercase', letterSpacing: '0.05em', whiteSpace: 'nowrap',
                    width: col.width,
                  }}>{col.header}</th>
                )
              ))}
            </tr>
          </thead>
          <tbody>
            {data.map((item: any, idx: number) => {
              const rowId = item.id !== undefined ? String(item.id) : String(idx)
              return (
                <tr key={rowId} onClick={() => onRowClick?.(item)}
                  style={{
                    borderBottom: '1px solid var(--glass-border)',
                    cursor: onRowClick ? 'pointer' : undefined,
                    transition: 'var(--transition)',
                  }}
                  onMouseEnter={(e) => { if (onRowClick) (e.currentTarget as HTMLElement).style.background = 'var(--glass-bg)' }}
                  onMouseLeave={(e) => { if (onRowClick) (e.currentTarget as HTMLElement).style.background = 'transparent' }}
                >
                  {columns.map((col) => (
                    !col.hideMobile && (
                      <td key={col.key} data-label={col.header} style={{
                        textAlign: col.align || 'left', padding: '0.65rem 1rem', whiteSpace: 'nowrap',
                      }}>
                        {col.render ? col.render(item) : String(item[col.key] ?? '')}
                      </td>
                    )
                  ))}
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
      <Pagination page={page} totalPages={totalPages} total={meta.total} onPageChange={onPageChange} />
    </div>
  )
}
