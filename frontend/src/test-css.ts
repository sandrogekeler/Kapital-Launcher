/// <reference types="node" />
import { readFileSync } from 'node:fs'

/**
 * The app's own stylesheet as written, for a test of what it says: motion
 * lives in CSS, which jsdom neither runs nor, under vitest, even loads (a CSS
 * import is served empty). Read from the frontend folder, where the tests run.
 */
export const STYLE_CSS = readFileSync('src/style.css', 'utf8')
