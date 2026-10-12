/* TwinTube - server clock estimate.
 * Playback position is computed from server timestamps. Using this computer's clock
 * instead would seek a viewer whose clock is minutes off to the wrong moment. */

let offsetMs = 0;

/** Record the server's timestamp from an incoming message. */
export function observeServerTime(serverMs) {
  if (typeof serverMs !== 'number' || !Number.isFinite(serverMs)) return;
  // The newest message wins. Network latency is small next to the drift that matters
  // here, and the estimate recovers at once if this computer's clock changes.
  offsetMs = serverMs - Date.now();
}

/** Current time on the server's clock, as far as we can tell. */
export function serverNowMs() {
  return Date.now() + offsetMs;
}
