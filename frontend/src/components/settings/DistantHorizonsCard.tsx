import { Download } from '../../lib/icons'
import { Button } from '../ui/Button'
import { Icon } from '../ui/Icon'
import { SubHeading } from '../ui/Section'
import { SettingsCard } from '../ui/SettingsLayout'

/**
 * The place for the server's Distant Horizons world data (issue 136), under the chapter's Server
 * settings, for a chapter whose server sends it: a one-time download of the far terrain that
 * otherwise fills in while the player plays. A placeholder until a snapshot is published: the
 * row says so and Download stays disabled. Nothing here reaches Go yet. When the manifest names
 * a snapshot, this card shows its size and date before anything downloads, then the progress
 * and Cancel, as the issue describes.
 */
export function DistantHorizonsCard() {
  return (
    <div className="flex flex-col gap-3 pt-3">
      <SubHeading>World data</SubHeading>
      <SettingsCard>
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
          <div className="flex min-w-0 flex-col gap-1">
            <span className="text-fg flex items-center gap-2 text-sm font-medium">
              Distant Horizons terrain
              <span className="border-line text-fg-muted rounded-pill text-2xs border px-2 font-normal">
                Not available yet
              </span>
            </span>
          </div>
          <Button onClick={() => undefined} disabled>
            <Icon icon={Download} size="sm" />
            <span>Download</span>
          </Button>
        </div>
      </SettingsCard>
    </div>
  )
}
