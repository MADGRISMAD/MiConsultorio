/** The public address of the app: the links people share (reservar, portal, ARCO) always use it, wherever the page is open. */
export const PUBLIC_ORIGIN = 'https://caresia.mx';

/** Only a developer's own machine keeps its own address, so the links can be tried there. */
export function publicOrigin(): string {
  if (typeof location === 'undefined') return PUBLIC_ORIGIN;
  return /^(localhost|127\.0\.0\.1|\[::1\])$/.test(location.hostname) ? location.origin : PUBLIC_ORIGIN;
}
