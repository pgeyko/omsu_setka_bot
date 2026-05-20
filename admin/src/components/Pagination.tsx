import { ChevronLeft, ChevronRight } from 'lucide-react'

interface Props {
  page: number
  totalPages: number
  total: number
  onPageChange: (page: number) => void
}

export default function Pagination({ page, totalPages, total, onPageChange }: Props) {
  if (totalPages <= 1) return null

  const pages: (number | string)[] = []
  for (let i = 0; i < totalPages; i++) {
    if (i === 0 || i === totalPages - 1 || Math.abs(i - page) <= 1) {
      pages.push(i)
    } else if (pages[pages.length - 1] !== '...') {
      pages.push('...')
    }
  }

  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '0.75rem 1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
      <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>Total: {total}</span>
      <div style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
        <button className="btn btn-sm" disabled={page === 0} onClick={() => onPageChange(page - 1)}>
          <ChevronLeft size={16} />
        </button>
        {pages.map((p, i) => (
          typeof p === 'number' ? (
            <button key={i} className="btn btn-sm"
              style={p === page ? { background: 'var(--accent)', color: '#fff', borderColor: 'var(--accent)' } : {}}
              onClick={() => onPageChange(p)}>
              {p + 1}
            </button>
          ) : (
            <span key={i} style={{ padding: '0 0.25rem', color: 'var(--text-muted)' }}>...</span>
          )
        ))}
        <button className="btn btn-sm" disabled={page >= totalPages - 1} onClick={() => onPageChange(page + 1)}>
          <ChevronRight size={16} />
        </button>
      </div>
    </div>
  )
}
