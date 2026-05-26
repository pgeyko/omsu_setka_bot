import { BarChart3, Users, MessageSquare, Shield, FileText, Settings, Send, Calendar } from 'lucide-react'

export interface NavItem {
  to: string
  icon: typeof BarChart3
  label: string
}

export const navItems: NavItem[] = [
  { to: '/stats', icon: BarChart3, label: 'Статистика' },
  { to: '/groups', icon: Users, label: 'Группы' },
  { to: '/topics', icon: MessageSquare, label: 'Топики' },
  { to: '/permissions', icon: Shield, label: 'Действия' },
  { to: '/prompts', icon: FileText, label: 'Промпты' },
  { to: '/settings', icon: Settings, label: 'Настройки' },
  { to: '/send-message', icon: Send, label: 'Отправить' },
  { to: '/schedule', icon: Calendar, label: 'Расписание' },
]

export const titleMap: Record<string, string> = {
  '/stats': 'Статистика',
  '/groups': 'Группы',
  '/topics': 'Топики',
  '/permissions': 'Действия',
  '/prompts': 'Промпты',
  '/settings': 'Настройки',
  '/send-message': 'Отправить сообщение',
  '/schedule': 'Расписание',
}
