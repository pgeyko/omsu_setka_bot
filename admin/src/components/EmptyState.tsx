import { ReactNode } from 'react'

interface Props {
  icon?: ReactNode
  text: string
}

export default function EmptyState({ icon, text }: Props) {
  return (
    <div className="empty-state">
      {icon}
      <p>{text}</p>
    </div>
  )
}
