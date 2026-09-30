/**
 * Wording and bounds for a chapter's settings panel (#36), kept pure so the
 * panel only renders them.
 */

/** The launcher's fixed heap minimum, Prism's own default (services.minMemMiB). */
export const MIN_MEMORY_MB = 512

/** The slider's step: a quarter gigabyte reads as a round number at every stop. */
export const MEMORY_STEP_MB = 256

/** The slider's end when the machine's memory is unknown. */
const FALLBACK_MAX_MB = 32768

/** A memory amount for a readout: gigabytes with one decimal from a gigabyte up. */
export function memoryLabel(mb: number): string {
  if (mb >= 1024) return `${(mb / 1024).toFixed(1)} GB`
  return `${mb} MB`
}

/** The slider's end: the machine's memory, rounded down to a step. */
export function sliderMaxMb(machineMemoryMb: number): number {
  const ceiling = machineMemoryMb > 0 ? machineMemoryMb : FALLBACK_MAX_MB
  return Math.max(MIN_MEMORY_MB, Math.floor(ceiling / MEMORY_STEP_MB) * MEMORY_STEP_MB)
}

/** What a preset does, for its radio label; an unknown name is shown as is. */
export function presetLabel(name: string): string {
  switch (name) {
    case '':
      return "Prism's own arguments"
    case 'zgc':
      return 'ZGC garbage collector (Java 21 and later)'
    default:
      return name
  }
}
