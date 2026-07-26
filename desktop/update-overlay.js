const titleEl = document.getElementById('updateTitle');
const bodyEl = document.getElementById('updateBody');
const versionEl = document.getElementById('updateVersion');
const iconEl = document.getElementById('updateIcon');
const progressBlock = document.getElementById('progressBlock');
const progressFill = document.getElementById('progressFill');
const progressLabel = document.getElementById('progressLabel');
const progressPercent = document.getElementById('progressPercent');
const btnPrimary = document.getElementById('btnPrimary');
const btnSecondary = document.getElementById('btnSecondary');
const actionsBlock = document.getElementById('actionsBlock');

const ICONS = {
  download: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>',
  restart: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="23 4 23 10 17 10"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>',
  error: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>'
};

let currentPhase = 'available';
let primaryAction = 'download';
let secondaryAction = 'later';

function setVisible(el, visible) {
  el.classList.toggle('hidden', !visible);
}

function applyState(state) {
  if (!state || typeof state !== 'object') return;

  currentPhase = state.phase || 'available';
  document.documentElement.setAttribute('data-theme', state.theme === 'dark' ? 'dark' : 'light');

  titleEl.textContent = state.title || '';
  bodyEl.textContent = state.body || '';

  if (state.version) {
    versionEl.textContent = `v${state.version}`;
    setVisible(versionEl, true);
  } else {
    setVisible(versionEl, false);
  }

  iconEl.classList.toggle('is-error', currentPhase === 'error');
  if (currentPhase === 'error') {
    iconEl.innerHTML = ICONS.error;
  } else if (currentPhase === 'ready') {
    iconEl.innerHTML = ICONS.restart;
  } else {
    iconEl.innerHTML = ICONS.download;
  }

  const showProgress = !!state.showProgress;
  setVisible(progressBlock, showProgress);
  if (showProgress) {
    const pct = Math.max(0, Math.min(100, Number(state.progress) || 0));
    progressFill.style.width = `${pct}%`;
    progressLabel.textContent = state.progressLabel || '';
    progressPercent.textContent = `${Math.round(pct)}%`;
  }

  btnPrimary.textContent = state.primaryLabel || '';
  btnSecondary.textContent = state.secondaryLabel || '';
  setVisible(btnSecondary, state.showSecondary !== false);
  setVisible(actionsBlock, state.showActions !== false);

  primaryAction = state.primaryAction || 'download';
  secondaryAction = state.secondaryAction || 'later';

  const busy = currentPhase === 'downloading';
  btnPrimary.disabled = busy && primaryAction !== 'dismiss';
  btnSecondary.disabled = busy;
}

btnPrimary.addEventListener('click', () => {
  window.updateOverlay?.sendAction(primaryAction);
});

btnSecondary.addEventListener('click', () => {
  window.updateOverlay?.sendAction(secondaryAction);
});

window.updateOverlay?.onState(applyState);
window.updateOverlay?.notifyReady();
