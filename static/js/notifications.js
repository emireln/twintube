/* TwinTube - Browser / desktop notification helper */

import { t } from './i18n.js';

const STORAGE_KEY = 'twintube_notify_enabled';
const MENTIONS_KEY = 'twintube_notify_mentions';
const COWATCH_KEY = 'twintube_notify_cowatch';

export class NotificationManager {
  constructor() {
    this.enabled = localStorage.getItem(STORAGE_KEY) !== '0';
    this.mentions = localStorage.getItem(MENTIONS_KEY) !== '0';
    this.cowatchers = localStorage.getItem(COWATCH_KEY) !== '0';
    this.permission = typeof Notification !== 'undefined' ? Notification.permission : 'denied';
  }

  isEnabled() {
    return this.enabled;
  }

  mentionsEnabled() {
    return this.enabled && this.mentions;
  }

  cowatchEnabled() {
    return this.enabled && this.cowatchers;
  }

  setEnabled(on) {
    this.enabled = !!on;
    localStorage.setItem(STORAGE_KEY, this.enabled ? '1' : '0');
  }

  setMentions(on) {
    this.mentions = !!on;
    localStorage.setItem(MENTIONS_KEY, this.mentions ? '1' : '0');
  }

  setCowatchers(on) {
    this.cowatchers = !!on;
    localStorage.setItem(COWATCH_KEY, this.cowatchers ? '1' : '0');
  }

  async ensurePermission() {
    if (!this.enabled) return false;
    if (typeof Notification === 'undefined') {
      if (window.twintubeDesktop?.showNotification) return true;
      return false;
    }
    if (Notification.permission === 'granted') {
      this.permission = 'granted';
      return true;
    }
    if (Notification.permission === 'denied') {
      this.permission = 'denied';
      return false;
    }
    try {
      this.permission = await Notification.requestPermission();
      return this.permission === 'granted';
    } catch (_) {
      return false;
    }
  }

  shouldNotify() {
    return document.hidden || !document.hasFocus();
  }

  notify(title, body, options = {}) {
    if (!this.enabled) return;
    const { tag = 'twintube', onClick, force = false } = options;
    if (!force && !this.shouldNotify()) return;

    if (window.twintubeDesktop?.showNotification) {
      window.twintubeDesktop.showNotification({ title, body, tag });
      if (typeof onClick === 'function') {
        window.addEventListener('twintube:notification-click', onClick, { once: true });
      }
      return;
    }

    if (typeof Notification === 'undefined' || Notification.permission !== 'granted') return;

    try {
      const n = new Notification(title, { body, tag, icon: '/static/desktop-logo.png' });
      if (typeof onClick === 'function') {
        n.onclick = () => {
          window.focus();
          onClick();
          n.close();
        };
      }
    } catch (_) {
      // ignore
    }
  }

  notifyMention(payload, roomId) {
    if (!this.mentionsEnabled()) return;
    const from = payload?.from || 'Someone';
    const snippet = payload?.content || '';
    this.notify(
      t('notify_mention_title').replace('{name}', from),
      snippet,
      {
        tag: `mention-${payload?.messageId || Date.now()}`,
        onClick: () => {
          if (roomId && !window.location.pathname.includes(roomId)) {
            window.location.href = `/room/${encodeURIComponent(roomId)}`;
          }
        }
      }
    );
  }

  notifyCowatcherRoom(payload) {
    if (!this.cowatchEnabled()) return;
    const host = payload?.hostNickname || 'Someone';
    const roomName = payload?.roomName || payload?.roomId || '';
    this.notify(
      t('notify_cowatch_title').replace('{name}', host),
      t('notify_cowatch_body').replace('{room}', roomName),
      {
        tag: `cowatch-${payload?.roomId || Date.now()}`,
        onClick: () => {
          const url = payload?.url || (payload?.roomId ? `/room/${payload.roomId}` : '/');
          window.location.href = url;
        }
      }
    );
  }
}

export const notifications = new NotificationManager();
