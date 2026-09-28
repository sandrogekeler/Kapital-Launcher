import wordmark from '../../assets/brand/wordmark-inline.png'

/** The Kapitel Kapital wordmark, from the wiki's public/brand, over the app's own eyebrow. */
export function Brand() {
  return (
    <div className="flex flex-col gap-1.5 px-1.5">
      <img src={wordmark} alt="Kapitel Kapital" className="block h-auto w-44" draggable={false} />
      <div className="text-fg-faint text-2xs font-mono tracking-[0.08em] uppercase">Launcher</div>
    </div>
  )
}
