import {
  HardDrive, Wrench, FileText, Pulse, Shield,
  CurrencyDollar, Lightning, Globe, Package,
  Database, Wallet, ChartLine, ChartBar,
  type Icon as PhosphorIcon,
} from '@phosphor-icons/react'

const ICONS_BY_ID: Record<string, PhosphorIcon> = {
  filebrowser:     HardDrive,
  'stirling-pdf':  ChartLine,
  'uptime-kuma':   ChartBar,
  vaultwarden:     Wallet,
  silverbullet:    FileText,
  memos:           FileText,
  n8n:             Lightning,
  'actual-budget': CurrencyDollar,
  excalidraw:      Wrench,
  couchdb:         Database,
  'it-tools':      Wrench,
  cyberchef:       Wrench,
  adminer:         Database,
  gatus:           Pulse,
  umami:           ChartBar,
  outline:         FileText,
}

const ICONS_BY_CATEGORY: Record<string, PhosphorIcon> = {
  storage:      HardDrive,
  utilities:    Wrench,
  productivity: FileText,
  monitoring:   Pulse,
  security:     Shield,
  finance:      CurrencyDollar,
  automation:   Lightning,
  networking:   Globe,
}

interface AppIconProps {
  appId?: string
  category?: string
  className?: string
}

export default function AppIcon({ appId, category, className = 'w-5 h-5' }: AppIconProps) {
  const Icon: PhosphorIcon =
    (appId && ICONS_BY_ID[appId]) ||
    (category && ICONS_BY_CATEGORY[category]) ||
    Package

  return <Icon className={className} strokeWidth={1.5} />
}
