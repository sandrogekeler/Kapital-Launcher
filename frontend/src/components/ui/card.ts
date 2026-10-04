/**
 * The raised card every screen sits in; a column, so a page's body can take
 * the height left. Its own module so the chapter stage and the page layer,
 * which load with the launcher, do not pull the page component with them.
 */
export const CARD =
  'bg-raised border-line flex min-h-0 grow flex-col overflow-hidden rounded-lg border'
