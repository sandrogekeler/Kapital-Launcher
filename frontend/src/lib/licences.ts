/**
 * What the About section credits, and where each licence text lives.
 *
 * A text is a dynamic `?raw` import, so Vite gives each its own chunk and it is
 * fetched only when a player opens it; the entry chunk stays inside
 * `pnpm check-bundle`'s budget. The font texts ship beside the font files
 * (the OFL asks that its licence travel with them); lucide's is a copy of the
 * package's LICENSE, because node_modules is not a stable import path.
 */
export interface Credit {
  id: string
  name: string
  /** What it is used for, for the line under the name. */
  role: string
  /** The licence, by name. */
  licence: string
  loadText: () => Promise<string>
}

export const CREDITS: readonly Credit[] = [
  {
    id: 'fraunces',
    name: 'Fraunces',
    role: 'Headings',
    licence: 'SIL Open Font License 1.1',
    loadText: () => import('../assets/fonts/licences/fraunces-OFL.txt?raw').then((m) => m.default),
  },
  {
    id: 'inter',
    name: 'Inter',
    role: 'Interface text',
    licence: 'SIL Open Font License 1.1',
    loadText: () => import('../assets/fonts/licences/inter-OFL.txt?raw').then((m) => m.default),
  },
  {
    id: 'jetbrains-mono',
    name: 'JetBrains Mono',
    role: 'Paths, versions and addresses',
    licence: 'SIL Open Font License 1.1',
    loadText: () =>
      import('../assets/fonts/licences/jetbrains-mono-OFL.txt?raw').then((m) => m.default),
  },
  {
    id: 'lucide',
    name: 'Lucide',
    role: 'Icons',
    licence: 'ISC License',
    loadText: () => import('../assets/licences/lucide-ISC.txt?raw').then((m) => m.default),
  },
]

/** Prism's repository: where its source is, which the GPL asks us to point to. */
export const PRISM_SOURCE = 'https://github.com/PrismLauncher/PrismLauncher'
