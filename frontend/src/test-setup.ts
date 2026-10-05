import '@testing-library/jest-dom/vitest'

// jsdom has no HTMLImageElement.decode; the app decodes the panorama's faces off the main
// thread before drawing them (lib/panorama). Here a face is decoded at once.
if (
  !('decode' in HTMLImageElement.prototype) ||
  typeof HTMLImageElement.prototype.decode !== 'function'
) {
  HTMLImageElement.prototype.decode = () => Promise.resolve()
}
