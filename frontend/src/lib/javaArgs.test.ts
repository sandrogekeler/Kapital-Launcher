import { describe, expect, it } from 'vitest'
import { MAX_JAVA_ARG_LEN, javaArgProblem } from './javaArgs'

describe('javaArgProblem', () => {
  it('takes one plain argument that starts with a dash', () => {
    for (const ok of ['-Xss4m', '-XX:+UseStringDeduplication', '-Dfml.readTimeout=180']) {
      expect(javaArgProblem(ok, [], null)).toBeNull()
    }
  })

  it('says why it cannot take the rest, as Go would refuse it', () => {
    expect(javaArgProblem('-Xss4m -Xss8m', [], null)).toMatch(/One argument at a time/)
    expect(javaArgProblem('Xss4m', [], null)).toMatch(/starts with a dash/)
    expect(javaArgProblem('-', [], null)).toMatch(/starts with a dash/)
    expect(javaArgProblem('-Dfoo="a"', [], null)).toMatch(/Quotes and backslashes/)
    expect(javaArgProblem('-Dfoo=C:\\x', [], null)).toMatch(/Quotes and backslashes/)
    expect(javaArgProblem('-Xmx16G', [], null)).toMatch(/slider/)
    expect(javaArgProblem('-Xms1G', [], null)).toMatch(/slider/)
    expect(javaArgProblem('-javaagent:x.jar', [], null)).toMatch(/agent/)
    expect(javaArgProblem('-XX:OnError=calc', [], null)).toMatch(/runs a command/)
    expect(javaArgProblem('-XX:VMOptionsFile=x', [], null)).toMatch(/from a file/)
    expect(javaArgProblem(`-D${'a'.repeat(MAX_JAVA_ARG_LEN)}`, [], null)).toMatch(/At most/)
  })

  it('refuses a repeat, but not the argument being edited itself', () => {
    expect(javaArgProblem('-Xss4m', ['-Xss4m'], null)).toMatch(/already in the list/)
    expect(javaArgProblem('-Xss4m', ['-Xss4m'], 0)).toBeNull()
    expect(javaArgProblem('-Xss4m', ['-Da', '-Xss4m'], 0)).toMatch(/already in the list/)
  })
})
