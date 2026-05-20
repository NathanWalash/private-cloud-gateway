import {
  HardDrive, Wrench, FileText, Activity, Shield,
  DollarSign, Zap, Globe, Package,
  BookMarked, Database, Wallet, FileChartLine, BarChart3,
  type LucideIcon,
} from 'lucide-react'

const ICONS_BY_ID: Record<string, LucideIcon> = {
  filebrowser:     HardDrive,
  'stirling-pdf':  FileChartLine,
  shiori:          BookMarked,
  'uptime-kuma':   BarChart3,
  vaultwarden:     Wallet,
  silverbullet:    FileText,
  memos:           FileText,
  n8n:             Zap,
  'actual-budget': DollarSign,
  excalidraw:      Wrench,
  couchdb:         Database,
}

const ICONS_BY_CATEGORY: Record<string, LucideIcon> = {
  storage:      HardDrive,
  utilities:    Wrench,
  productivity: FileText,
  monitoring:   Activity,
  security:     Shield,
  finance:      DollarSign,
  automation:   Zap,
  networking:   Globe,
}

interface AppIconProps {
  appId?: string
  category?: string
  className?: string
}

export default function AppIcon({ appId, category, className = 'w-5 h-5' }: AppIconProps) {
  const Icon: LucideIcon =
    (appId && ICONS_BY_ID[appId]) ||
    (category && ICONS_BY_CATEGORY[category]) ||
    Package

  return <Icon className={className} strokeWidth={1.5} />
}
