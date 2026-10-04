/** Bytes as a person reads them: B under a kilobyte, then KB, MB, GB, one decimal under ten. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${Math.max(0, bytes)} B`
  const units = ['KB', 'MB', 'GB']
  let value = bytes / 1024
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit++
  }
  return `${value < 10 ? value.toFixed(1) : Math.round(value)} ${units[unit]}`
}
