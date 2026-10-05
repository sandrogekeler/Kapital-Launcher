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
  Check,
  ChevronRight,
  ChevronDown,
  Code,
  Copy,
  Download,
  Ellipsis,
  ExternalLink,
  Folder,
  FolderOpen,
  Info,
  KeyRound,
  LifeBuoy,
  // lucide's `Map`, renamed: a bare `Map` would shadow the global one.
  Map as MapIcon,
  MemoryStick,
  Minus,
  Package,
  Palette,
  Pencil,
  Play,
  Plus,
  Puzzle,
  RefreshCw,
  Server,
  Settings,
  Square,
  SquareTerminal,
  Trash2,
  Triangle,
  TriangleAlert,
  UserRound,
  X,
} from 'lucide-react'
