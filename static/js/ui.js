/* TwinTube - UI Component Manager */

export class UIManager {
  constructor() {
    this.toastContainer = document.getElementById('toastContainer');
    this.initTheme();
    this.initTabs();
  }

  // Theme Management
  initTheme() {
    const savedTheme = localStorage.getItem('twintube_theme') || 
      (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
    this.setTheme(savedTheme);

    const btnTheme = document.getElementById('btnThemeToggle');
    if (btnTheme) {
      btnTheme.addEventListener('click', () => {
        const current = document.documentElement.getAttribute('data-theme');
        const next = current === 'dark' ? 'light' : 'dark';
        this.setTheme(next);
      });
    }
  }

  setTheme(theme) {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('twintube_theme', theme);
    const btnTheme = document.getElementById('btnThemeToggle');
    if (btnTheme) {
      const moon = btnTheme.querySelector('.icon-moon');
      const sun = btnTheme.querySelector('.icon-sun');
      if (moon && sun) {
        moon.style.display = theme === 'dark' ? 'none' : 'block';
        sun.style.display = theme === 'dark' ? 'block' : 'none';
      }
    }
  }

  // Tab Navigation
  initTabs() {
    const tabButtons = document.querySelectorAll('.tab-btn[data-tab]');
    tabButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        const tabId = btn.getAttribute('data-tab');
        
        tabButtons.forEach(b => b.classList.remove('active'));
        document.querySelectorAll('.tab-content').forEach(c => c.classList.remove('active'));

        btn.classList.add('active');
        const targetContent = document.getElementById(tabId);
        if (targetContent) {
          targetContent.classList.add('active');
        }
      });
    });
  }

  // Toast Notifications
  showToast(message, duration = 3000) {
    const toast = document.createElement('div');
    toast.className = 'toast';
    toast.innerHTML = `<svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg> <span>${this.escapeHTML(message)}</span>`;
    
    if (!this.toastContainer) {
      this.toastContainer = document.getElementById('toastContainer');
    }
    if (this.toastContainer) {
      this.toastContainer.appendChild(toast);
    }

    setTimeout(() => {
      toast.style.opacity = '0';
      toast.style.transform = 'translateY(100%)';
      toast.style.transition = 'all 0.3s ease';
      setTimeout(() => toast.remove(), 300);
    }, duration);
  }

  // Modals
  showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) modal.classList.add('active');
  }

  hideModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) modal.classList.remove('active');
  }

  showNicknameModal() { this.showModal('nicknameModal'); }
  hideNicknameModal() { this.hideModal('nicknameModal'); }

  showAuthModal() { this.showModal('authModal'); }
  hideAuthModal() { this.hideModal('authModal'); }

  // Chat Rendering
  renderChatMessage(msg) {
    const container = document.getElementById('chatMessages');
    if (!container) return;

    const el = document.createElement('div');
    
    if (msg.isSystem) {
      el.className = 'msg-system';
      el.textContent = msg.content;
    } else {
      el.className = 'msg-item';
      const initial = (msg.nickname || 'G').charAt(0).toUpperCase();
      el.innerHTML = `
        <div class="msg-avatar">${initial}</div>
        <div class="msg-body">
          <div class="msg-header">
            <span class="msg-author">${this.escapeHTML(msg.nickname)}</span>
            <span class="msg-time">${msg.timestamp || ''}</span>
          </div>
          <div class="msg-text">${this.escapeHTML(msg.content)}</div>
        </div>
      `;
    }

    container.appendChild(el);
    container.scrollTop = container.scrollHeight;
  }

  // Playlist Queue Rendering
  renderQueue(playlist, isHost, onPlayItem, onRemoveItem) {
    const container = document.getElementById('queueList');
    const counter = document.getElementById('queueCounter');
    if (!container) return;

    if (counter) counter.textContent = playlist.length;
    container.innerHTML = '';

    if (playlist.length === 0) {
      container.innerHTML = `
        <div style="padding: 24px; text-align: center; color: var(--md-on-surface-variant); font-size: 13px;">
          The queue is currently empty.<br>Paste a YouTube link in the top bar to add a video!
        </div>
      `;
      return;
    }

    playlist.forEach((item) => {
      const el = document.createElement('div');
      el.className = 'queue-item';
      el.innerHTML = `
        <img class="queue-thumb" src="${item.thumbnailUrl || 'https://img.youtube.com/vi/' + item.videoId + '/hqdefault.jpg'}" alt="Thumb">
        <div class="queue-details">
          <div class="queue-title">${this.escapeHTML(item.title)}</div>
          <div class="queue-meta">Added by ${this.escapeHTML(item.addedBy)}</div>
        </div>
        <div class="queue-actions">
          <button class="nav-icon-btn btn-play" title="Play Video" style="width:30px; height:30px;">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          </button>
          <button class="nav-icon-btn btn-remove" title="Remove from Queue" style="width:30px; height:30px;">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          </button>
        </div>
      `;

      el.querySelector('.btn-play').addEventListener('click', () => onPlayItem(item.id));
      el.querySelector('.btn-remove').addEventListener('click', () => onRemoveItem(item.id));

      container.appendChild(el);
    });
  }

  // Audience Viewers Rendering
  renderViewers(users, currentClientID, isHost, onTransferHost) {
    const container = document.getElementById('viewersList');
    const counter = document.getElementById('viewersCounter');
    if (!container) return;

    if (counter) counter.textContent = users.length;
    container.innerHTML = '';

    users.forEach(user => {
      const el = document.createElement('div');
      el.className = 'viewer-item';
      const isYou = user.id === currentClientID;
      const initial = (user.nickname || 'G').charAt(0).toUpperCase();

      el.innerHTML = `
        <div class="viewer-info">
          <div class="msg-avatar">${initial}</div>
          <div>
            <span style="font-weight: 500; font-size: 14px;">${this.escapeHTML(user.nickname)}</span>
            ${isYou ? '<span style="font-size: 11px; opacity: 0.7;"> (You)</span>' : ''}
          </div>
        </div>
        <div style="display: flex; gap: 6px; align-items: center;">
          ${user.isGuest ? '<span class="badge badge-guest" style="font-size: 10px; padding: 2px 6px;">Guest</span>' : ''}
          ${user.isHost ? '<span class="badge badge-host"><svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><path d="M5 16L3 5l5.5 5L12 4l3.5 6L21 5l-2 11H5zm14 3c0 .6-.4 1-1 1H6c-.6 0-1-.4-1-1v-1h14v1z"/></svg> Host</span>' : ''}
          ${isHost && !user.isHost ? `<button class="btn btn-secondary btn-transfer" style="font-size: 11px; padding: 4px 10px;">Make Host</button>` : ''}
        </div>
      `;

      const btnTransfer = el.querySelector('.btn-transfer');
      if (btnTransfer) {
        btnTransfer.addEventListener('click', () => onTransferHost(user.id));
      }

      container.appendChild(el);
    });
  }

  escapeHTML(str) {
    if (!str) return '';
    return str.replace(/[&<>"']/g, match => {
      const map = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' };
      return map[match];
    });
  }
}
