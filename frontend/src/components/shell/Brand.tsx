import logo from '../../assets/brand/kapital-launcher-logo.png'

/**
 * The Kapital Launcher logo for the header bar. The source artwork lives in
 * ../Art at full size; this copy is scaled to twice its display height. At
 * 16 px in the 40 px bar it has as much air above and below as a title bar's
 * text would (#70).
 */
export function Brand() {
  return <img src={logo} alt="Kapital Launcher" className="block h-4 w-auto" draggable={false} />
}
