import logo from '../../assets/brand/kapital-launcher-logo.png'

/**
 * The Kapital Launcher logo for the header bar. The source artwork lives in
 * ../Art at full size; this copy is scaled to twice its display height.
 */
export function Brand() {
  return <img src={logo} alt="Kapital Launcher" className="block h-5 w-auto" draggable={false} />
}
