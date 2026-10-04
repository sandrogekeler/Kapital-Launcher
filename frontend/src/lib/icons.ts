/**
 * The only module in the app that imports `lucide-react`; eslint.config.js
 * refuses the import anywhere else.
 *
 * Every render site takes the icon as a component through components/ui/Icon,
 * so swapping the icon set is an edit here, not a sweep across the tree, and
 * the entry chunk pays only for what is re-exported (`sideEffects: false`).
 * Adding an icon: find it on https://lucide.dev/icons, add its PascalCase name
 * to the export below. Never hand-copy an SVG into a component.
 *
 * lucide-react is ISC licensed (c) Lucide Icons and Contributors.
 */
export type { LucideIcon } from 'lucide-react'

export {
  ArrowDown,
  ArrowLeft,
  BookOpen,
  Check,
  ChevronRight,
  ChevronDown,
  Copy,
  Download,
  ExternalLink,
  Folder,
  FolderOpen,
  Info,
  Minus,
  Pencil,
  Play,
  RefreshCw,
  Settings,
  Square,
  SquareTerminal,
  TriangleAlert,
  X,
} from 'lucide-react'
