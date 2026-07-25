import { getLanguage, setLanguage, updateDOMTranslations, t } from './i18n.js';

export class UIManager {
  constructor() {
    this.toastContainer = document.getElementById('toastContainer');
    this.cachedRooms = [];
    this.activeDeleteRoomHandler = null;
    this.settingsAuth = null;
    this.initTheme();
    this.initTabs();
    this.initLangPicker();
    this.initSettings();
  }

  bindSettingsAuth(auth) {
    this.settingsAuth = auth;
  }

  initLangPicker() {
    const picker = document.getElementById('langPicker');
    const btn = document.getElementById('btnLangPicker');
    const menu = document.getElementById('langPickerMenu');

    if (!picker || !btn || !menu) return;

    btn.addEventListener('click', (e) => {
      e.stopPropagation();
      const open = picker.classList.toggle('open');
      btn.setAttribute('aria-expanded', open ? 'true' : 'false');
    });

    menu.querySelectorAll('.lang-picker-option').forEach(option => {
      option.addEventListener('click', (e) => {
        e.stopPropagation();
        const lang = option.getAttribute('data-lang');
        if (lang) setLanguage(lang);
        picker.classList.remove('open');
        btn.setAttribute('aria-expanded', 'false');
      });
    });

    document.addEventListener('click', () => {
      picker.classList.remove('open');
      btn.setAttribute('aria-expanded', 'false');
    });
  }

  initSettings() {
    const btnClose = document.getElementById('btnCloseSettings');
    const langSelect = document.getElementById('settingLangSelect');
    const btnSaveProfile = document.getElementById('btnSaveProfile');
    const btnChangePassword = document.getElementById('btnChangePassword');
    const btnRandomAvatar = document.getElementById('btnRandomAvatar');

    if (btnClose) {
      btnClose.addEventListener('click', () => this.hideModal('settingsModal'));
    }

    if (langSelect) {
      langSelect.value = getLanguage();
      langSelect.addEventListener('change', (e) => setLanguage(e.target.value));
    }

    if (btnRandomAvatar) {
      btnRandomAvatar.addEventListener('click', () => {
        const username = document.getElementById('settingProfileUsername')?.value.trim() || 'user';
        const seed = username + '_' + Math.random().toString(36).slice(2, 8);
        const url = `https://api.dicebear.com/7.x/bottts/svg?seed=${encodeURIComponent(seed)}`;
        const avatarInput = document.getElementById('settingProfileAvatar');
        const preview = document.getElementById('settingProfileAvatarPreview');
        if (avatarInput) avatarInput.value = url;
        if (preview) preview.src = url;
      });
    }

    if (btnSaveProfile) {
      btnSaveProfile.addEventListener('click', async () => {
        if (!this.settingsAuth?.isLoggedIn()) return;
        const username = document.getElementById('settingProfileUsername')?.value.trim();
        const email = document.getElementById('settingProfileEmail')?.value.trim();
        const avatarUrl = document.getElementById('settingProfileAvatar')?.value.trim();
        btnSaveProfile.disabled = true;
        try {
          await this.settingsAuth.updateProfile({ username, email, avatarUrl });
          this.updateUserNavUI(this.settingsAuth);
          this.showToast(t('profile_updated'));
        } catch (err) {
          this.showToast(err.message || t('profile_update_fail'));
        } finally {
          btnSaveProfile.disabled = false;
        }
      });
    }

    if (btnChangePassword) {
      btnChangePassword.addEventListener('click', async () => {
        if (!this.settingsAuth?.isLoggedIn()) return;
        const current = document.getElementById('settingCurrentPassword')?.value || '';
        const next = document.getElementById('settingNewPassword')?.value || '';
        const confirm = document.getElementById('settingConfirmPassword')?.value || '';
        if (next !== confirm) {
          this.showToast(t('password_mismatch'));
          return;
        }
        btnChangePassword.disabled = true;
        try {
          await this.settingsAuth.changePassword(current, next);
          document.getElementById('settingCurrentPassword').value = '';
          document.getElementById('settingNewPassword').value = '';
          document.getElementById('settingConfirmPassword').value = '';
          this.showToast(t('password_changed'));
        } catch (err) {
          this.showToast(err.message || t('password_change_fail'));
        } finally {
          btnChangePassword.disabled = false;
        }
      });
    }

    const avatarInput = document.getElementById('settingProfileAvatar');
    const avatarPreview = document.getElementById('settingProfileAvatarPreview');
    if (avatarInput && avatarPreview) {
      avatarInput.addEventListener('input', () => {
        const url = avatarInput.value.trim();
        avatarPreview.src = url || '';
      });
    }

    updateDOMTranslations();
  }

  populateSettingsForm(auth) {
    const profileSection = document.getElementById('settingsProfileSection');
    const passwordSection = document.getElementById('settingsPasswordSection');
    const passwordDivider = document.getElementById('settingsPasswordDivider');
    const loginHint = document.getElementById('settingsLoginHint');
    const loggedIn = auth?.isLoggedIn();

    if (profileSection) profileSection.style.display = loggedIn ? 'flex' : 'none';
    if (passwordSection) passwordSection.style.display = loggedIn ? 'flex' : 'none';
    if (passwordDivider) passwordDivider.style.display = loggedIn ? 'block' : 'none';
    if (loginHint) loginHint.style.display = loggedIn ? 'none' : 'block';

    if (!loggedIn) return;

    const user = auth.getUser();
    const usernameEl = document.getElementById('settingProfileUsername');
    const emailEl = document.getElementById('settingProfileEmail');
    const avatarEl = document.getElementById('settingProfileAvatar');
    const previewEl = document.getElementById('settingProfileAvatarPreview');

    if (usernameEl) usernameEl.value = user.username || '';
    if (emailEl) emailEl.value = user.email || '';
    if (avatarEl) avatarEl.value = user.avatarUrl || '';
    if (previewEl && user.avatarUrl) previewEl.src = user.avatarUrl;
  }

  updateUserNavUI(auth) {
    const btnOpenAuthLogin = document.getElementById('btnOpenAuthLogin');
    const btnOpenAuthRegister = document.getElementById('btnOpenAuthRegister');
    const btnOpenAuth = document.getElementById('btnOpenAuth');
    const userProfileMenu = document.getElementById('userProfileMenu');
    const userAvatarText = document.getElementById('userAvatarText');
    const userAvatarImg = document.getElementById('userAvatarImg');
    const dropdownUsername = document.getElementById('dropdownUsername');
    const dropdownEmail = document.getElementById('dropdownEmail');

    if (auth?.isLoggedIn()) {
      const user = auth.getUser();
      if (btnOpenAuthLogin) btnOpenAuthLogin.style.display = 'none';
      if (btnOpenAuthRegister) btnOpenAuthRegister.style.display = 'none';
      if (btnOpenAuth) btnOpenAuth.style.display = 'none';
      if (userProfileMenu) userProfileMenu.style.display = 'block';
      if (dropdownUsername) dropdownUsername.textContent = user.username;
      if (dropdownEmail) dropdownEmail.textContent = user.email;

      if (user.avatarUrl && userAvatarImg) {
        userAvatarImg.src = user.avatarUrl;
        userAvatarImg.hidden = false;
        if (userAvatarText) userAvatarText.style.display = 'none';
      } else {
        if (userAvatarImg) userAvatarImg.hidden = true;
        if (userAvatarText) {
          userAvatarText.style.display = '';
          userAvatarText.textContent = (user.username || 'U').charAt(0).toUpperCase();
        }
      }
    } else {
      if (btnOpenAuthLogin) btnOpenAuthLogin.style.display = 'inline-flex';
      if (btnOpenAuthRegister) btnOpenAuthRegister.style.display = 'inline-flex';
      if (btnOpenAuth) btnOpenAuth.style.display = 'inline-flex';
      if (userProfileMenu) userProfileMenu.style.display = 'none';
    }
  }

  initMyRoomsModal(fetchRooms, deleteRoom, createRoomCallback) {
    const btnClose = document.getElementById('btnCloseMyRooms');
    const btnCreate = document.getElementById('btnCreateRoomFromModal');
    const searchInput = document.getElementById('roomsSearchInput');

    document.addEventListener('click', async (e) => {
      const btnSettings = e.target.closest('#btnSettings');
      if (btnSettings) {
        e.preventDefault();
        const dropdown = document.getElementById('profileDropdown');
        if (dropdown) dropdown.classList.remove('active');
        const langSelect = document.getElementById('settingLangSelect');
        if (langSelect) langSelect.value = getLanguage();
        this.populateSettingsForm(this.settingsAuth);
        this.showModal('settingsModal');
        return;
      }

      const btnMyRooms = e.target.closest('#btnMyRooms');
      if (btnMyRooms) {
        e.preventDefault();
        const dropdown = document.getElementById('profileDropdown');
        if (dropdown) dropdown.classList.remove('active');
        this.showModal('myRoomsModal');
        if (fetchRooms) {
          await this.renderModalRooms(fetchRooms, deleteRoom);
        }
        return;
      }
    });

    if (btnClose) {
      btnClose.addEventListener('click', () => this.hideModal('myRoomsModal'));
    }

    if (btnCreate) {
      btnCreate.addEventListener('click', () => {
        this.hideModal('myRoomsModal');
        if (createRoomCallback) createRoomCallback();
        else this.showModal('createRoomModal');
      });
    }

    if (searchInput) {
      searchInput.addEventListener('input', () => {
        this.filterModalRooms(searchInput.value.trim().toLowerCase());
      });
    }
  }

  async renderModalRooms(fetchRooms, deleteRoom) {
    const list = document.getElementById('modalRoomsList');
    if (!list) return;

    list.innerHTML = `<div style="padding: 20px; text-align: center; color: var(--md-on-surface-variant); font-size: 13px;">${t('loading_rooms')}</div>`;

    try {
      const rooms = await fetchRooms();
      this.cachedRooms = rooms || [];
      this.activeDeleteRoomHandler = deleteRoom;
      this.displayModalRooms(this.cachedRooms);
    } catch (err) {
      list.innerHTML = `<div style="padding: 20px; text-align: center; color: var(--md-on-surface-variant); font-size: 13px;">${this.escapeHTML(err.message || 'Failed to load rooms')}</div>`;
    }
  }

  filterModalRooms(query) {
    if (!this.cachedRooms) return;
    if (!query) {
      this.displayModalRooms(this.cachedRooms);
      return;
    }
    const filtered = this.cachedRooms.filter(r => 
      (r.name || '').toLowerCase().includes(query) || 
      (r.id || '').toLowerCase().includes(query)
    );
    this.displayModalRooms(filtered);
  }

  displayModalRooms(rooms) {
    const list = document.getElementById('modalRoomsList');
    if (!list) return;

    list.innerHTML = '';

    if (rooms.length === 0) {
      list.innerHTML = `
        <div style="padding: 30px; text-align: center; color: var(--md-on-surface-variant); font-size: 13px;">
          ${t('no_rooms_found', 'No rooms found.')}
        </div>
      `;
      return;
    }

    rooms.forEach(room => {
      const card = document.createElement('div');
      card.className = 'room-item-card';

      const expires = room.expiresAt ? new Date(room.expiresAt) : null;
      let expiresLabel = '';
      if (expires) {
        const diffMs = expires - new Date();
        const diffHours = Math.max(0, Math.floor(diffMs / (1000 * 60 * 60)));
        const diffDays = Math.floor(diffHours / 24);
        const remHours = diffHours % 24;
        expiresLabel = `${t('expires', 'Expires')} ${diffDays}d ${remHours}h`;
      } else {
        expiresLabel = t('saved_badge', 'Saved for 7 days');
      }

      card.innerHTML = `
        <div class="room-item-info">
          <div class="room-item-title">${this.escapeHTML(room.name || ('Room ' + room.id))}</div>
          <div class="room-item-meta">
            <code>${this.escapeHTML(room.id)}</code>
            <span>•</span>
            <span>${room.hasPassword ? '🔒 ' + t('private', 'Private') : '🌐 ' + t('public', 'Public')}</span>
            <span>•</span>
            <span>${expiresLabel}</span>
          </div>
        </div>
        <div class="room-item-actions">
          <button class="btn btn-secondary btn-copy" style="font-size: 11px; padding: 4px 10px;" title="Copy Link">${t('copy', 'Copy')}</button>
          <button class="btn btn-primary btn-open" style="font-size: 11px; padding: 4px 10px;" title="Open Room">${t('open', 'Open')}</button>
          <button class="btn btn-secondary btn-del" style="font-size: 11px; padding: 4px 10px; color: #e53935;" title="Delete Room">${t('delete', 'Delete')}</button>
        </div>
      `;

      card.querySelector('.btn-copy').addEventListener('click', () => {
        const url = `${window.location.origin}/room/${room.id}`;
        navigator.clipboard.writeText(url).then(() => {
          this.showToast(t('link_copied', 'Room link copied to clipboard!'));
        });
      });

      card.querySelector('.btn-open').addEventListener('click', () => {
        window.location.href = `/room/${room.id}`;
      });

      card.querySelector('.btn-del').addEventListener('click', async () => {
        if (!confirm(`${t('delete_room_confirm')} "${room.name || room.id}"?`)) return;
        if (this.activeDeleteRoomHandler) {
          try {
            await this.activeDeleteRoomHandler(room.id);
            this.showToast(t('room_deleted', 'Room deleted successfully'));
            this.cachedRooms = this.cachedRooms.filter(r => r.id !== room.id);
            this.displayModalRooms(this.cachedRooms);
          } catch (err) {
            this.showToast(t('room_delete_fail', err.message || 'Failed to delete room'));
          }
        }
      });

      list.appendChild(card);
    });
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
          ${t('empty_queue')}
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
