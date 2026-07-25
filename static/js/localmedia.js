/* TwinTube - Local file registry (client-only bytes; server stores hash for sync) */

const HEAD_BYTES = 2 * 1024 * 1024;
const TAIL_BYTES = 64 * 1024;

/** @type {Map<string, { file: File, blobUrl: string, title: string, size: number }>} */
const registry = new Map();

function toHex(buffer) {
  return [...new Uint8Array(buffer)].map(b => b.toString(16).padStart(2, '0')).join('');
}

/**
 * Content fingerprint from size + head + tail. Fast for large files; same file → same id.
 * Video bytes never leave the browser.
 */
export async function fingerprintLocalFile(file) {
  if (!file || !(file instanceof Blob)) {
    throw new Error('Invalid file');
  }
  const size = file.size;
  const headLen = Math.min(size, HEAD_BYTES);
  const head = new Uint8Array(await file.slice(0, headLen).arrayBuffer());

  let tail = new Uint8Array(0);
  if (size > HEAD_BYTES + TAIL_BYTES) {
    tail = new Uint8Array(await file.slice(size - TAIL_BYTES).arrayBuffer());
  } else if (size > headLen) {
    tail = new Uint8Array(await file.slice(headLen).arrayBuffer());
  }

  const meta = new TextEncoder().encode(`twintube-local|${size}`);
  const total = new Uint8Array(meta.length + head.length + tail.length);
  total.set(meta, 0);
  total.set(head, meta.length);
  total.set(tail, meta.length + head.length);

  const digest = await crypto.subtle.digest('SHA-256', total);
  return `local:${toHex(digest)}`;
}

export function isLocalVideoId(videoId) {
  return typeof videoId === 'string' && videoId.startsWith('local:');
}

export function getLocalBlobUrl(videoId) {
  return registry.get(videoId)?.blobUrl || '';
}

export function getLocalEntry(videoId) {
  return registry.get(videoId) || null;
}

export function registerLocalFile(videoId, file, blobUrl = '') {
  if (!isLocalVideoId(videoId) || !file) return null;
  const existing = registry.get(videoId);
  if (existing?.blobUrl && existing.blobUrl !== blobUrl) {
    try { URL.revokeObjectURL(existing.blobUrl); } catch (_) { /* ignore */ }
  }
  const url = blobUrl || URL.createObjectURL(file);
  const entry = {
    file,
    blobUrl: url,
    title: file.name || 'Local video',
    size: file.size
  };
  registry.set(videoId, entry);
  return entry;
}

export async function registerLocalFileFromPicker(file) {
  const videoId = await fingerprintLocalFile(file);
  return { videoId, entry: registerLocalFile(videoId, file) };
}

export async function tryMatchLocalFile(expectedVideoId, file) {
  const videoId = await fingerprintLocalFile(file);
  if (videoId !== expectedVideoId) {
    return { ok: false, videoId };
  }
  registerLocalFile(videoId, file);
  return { ok: true, videoId };
}
