/**
 * The profile name on the account page (issue 192). Prism picks the account by
 * its profile name, which is the player's Minecraft name, so a name that cannot
 * be one is worth a word before Play finds out. Go's own check (LaunchArgs) is
 * narrower and is the one that refuses: this only warns.
 */

/** Minecraft's own rule for a Java Edition name. */
const MINECRAFT_NAME = /^[A-Za-z0-9_]{3,16}$/

/** Why `name` cannot be a Minecraft name, or null when it can or is empty (Prism's default). */
export function profileNameProblem(name: string): string | null {
  const n = name.trim()
  if (n === '' || MINECRAFT_NAME.test(n)) return null
  return 'A Minecraft name is 3 to 16 letters, digits or _.'
}

/**
 * The name to show for the account: the profile name, or with none the name the game last
 * reported for Prism's default account (`playerName`), or empty when neither is known.
 */
export function accountName(profileName: string, playerName = ''): string {
  return profileName.trim() || playerName.trim()
}

/** Up to two letters for the account's tile, from the name's words. */
export function profileInitials(name: string): string {
  return name
    .split(/[\s_-]+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
    .toUpperCase()
}
