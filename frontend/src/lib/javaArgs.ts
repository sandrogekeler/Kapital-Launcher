/**
 * The player's own Java arguments (issue 191): what the list says about one as
 * it is typed. Go's services.ValidateJVMArgs is the rule that counts and refuses
 * the save; this mirrors it so the reason shows before Save, in words.
 */

/** As Go's maxJVMArgs and maxJVMArgLen. */
export const MAX_JAVA_ARGS = 32
export const MAX_JAVA_ARG_LEN = 200

/** As Go's refusedJVMArgs, with the reason in the panel's words. */
const REFUSED: readonly { prefix: string; why: string }[] = [
  { prefix: '-Xmx', why: 'Memory is set with the slider above.' },
  { prefix: '-Xms', why: 'Memory is set with the slider above.' },
  { prefix: '-javaagent', why: 'An agent loads code into the game, so it is not allowed.' },
  { prefix: '-agentpath', why: 'An agent loads code into the game, so it is not allowed.' },
  { prefix: '-agentlib', why: 'An agent loads code into the game, so it is not allowed.' },
  { prefix: '-XX:OnError', why: 'This runs a command, so it is not allowed.' },
  { prefix: '-XX:OnOutOfMemoryError', why: 'This runs a command, so it is not allowed.' },
  { prefix: '-XX:VMOptionsFile', why: 'This reads options from a file, so it is not allowed.' },
  { prefix: '-XX:Flags', why: 'This reads options from a file, so it is not allowed.' },
]

/**
 * Why `value` cannot be added to `args` (or replace `args[index]` when editing),
 * or null when it can. `value` is the trimmed text of the field.
 */
export function javaArgProblem(
  value: string,
  args: readonly string[],
  index: number | null,
): string | null {
  if (/\s/.test(value)) return 'One argument at a time. Add the next one with +.'
  if (!value.startsWith('-') || value.length < 2) {
    return 'An argument starts with a dash, like -Xss4m.'
  }
  if (/["'\\]/.test(value) || /\p{Cc}/u.test(value)) {
    return 'Quotes and backslashes are not supported.'
  }
  if (value.length > MAX_JAVA_ARG_LEN) return `At most ${MAX_JAVA_ARG_LEN} characters.`
  const refused = REFUSED.find((r) => value.startsWith(r.prefix))
  if (refused) return refused.why
  if (args.some((a, i) => a === value && i !== index)) {
    return 'This argument is already in the list.'
  }
  return null
}
