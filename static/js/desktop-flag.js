/* TwinTube - marks the desktop shell before first paint.
 * Kept as an external file so the Content-Security-Policy can drop 'unsafe-inline' for scripts. */
(function () {
  try {
    if (new URLSearchParams(location.search).get('desktop') === '1') {
      document.documentElement.classList.add('desktop-app');
    }
  } catch (_) {}
})();
