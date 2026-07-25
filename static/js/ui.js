import { getLanguage, getLanguageLabel, setLanguage, updateDOMTranslations, t } from './i18n.js';

function queueThumbUrl(item) {
  if (item.thumbnailUrl) return item.thumbnailUrl;
  const id = item.videoId || '';
  if (id.startsWith('local:') || id.startsWith('http://') || id.startsWith('https://')) {
    return '/static/favicon.svg';
  }
  if (/^[a-zA-Z0-9_-]{11}$/.test(id)) {
    return `https://img.youtube.com/vi/${id}/hqdefault.jpg`;
  }
  return '/static/favicon.svg';
}

export class UIManager {
  constructor() {
    this.toastContainer = document.getElementById('toastContainer');
    this.cachedRooms = [];
    this.activeDeleteRoomHandler = null;
    this.settingsAuth = null;
    this.initTheme();
    this.initTabs();
    this.initSettingsLangPicker();
    this.initSettings();
  }

  bindSettingsAuth(auth) {
    this.settingsAuth = auth;
  }

  closeSettingsLangPicker() {
    const picker = document.getElementById('settingsLangPicker');
    const btn = document.getElementById('btnSettingsLangPicker');
    if (picker) picker.classList.remove('open');
    if (btn) btn.setAttribute('aria-expanded', 'false');
  }

  syncSettingsLangPicker() {
    const label = document.getElementById('settingsLangPickerLabel');
    if (label) label.textContent = getLanguageLabel();
    document.querySelectorAll('#settingsLangPickerMenu .lang-picker-option').forEach(btn => {
      btn.classList.toggle('active', btn.getAttribute('data-lang') === getLanguage());
    });
  }

  initSettingsLangPicker() {
    const picker = document.getElementById('settingsLangPicker');
    const btn = document.getElementById('btnSettingsLangPicker');
    const menu = document.getElementById('settingsLangPickerMenu');
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
        this.syncSettingsLangPicker();
        this.closeSettingsLangPicker();
      });
    });

    document.addEventListener('click', (e) => {
      if (picker.contains(e.target)) return;
      this.closeSettingsLangPicker();
    });
  }

  async openSettings() {
    if (this.settingsAuth) {
      await this.settingsAuth.checkAuth();
    }
    this.populateSettingsForm(this.settingsAuth);
    this.syncSettingsLangPicker();
    this.closeSettingsLangPicker();
    this.showModal('settingsModal');
  }

  initSettings() {
    const btnClose = document.getElementById('btnCloseSettings');
    const btnSaveProfile = document.getElementById('btnSaveProfile');
    const btnSaveEmail = document.getElementById('btnSaveEmail');
    const btnChangePassword = document.getElementById('btnChangePassword');
    const btnRandomAvatar = document.getElementById('btnRandomAvatar');

    if (btnClose) {
      btnClose.addEventListener('click', () => {
        this.closeSettingsLangPicker();
        this.hideModal('settingsModal');
      });
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
        const auth = this.settingsAuth;
        if (!auth?.isLoggedIn()) {
          this.showToast(t('profile_login_hint'));
          return;
        }
        const username = document.getElementById('settingProfileUsername')?.value.trim();
        const avatarUrl = document.getElementById('settingProfileAvatar')?.value.trim();
        const user = auth.getUser();
        if (!username) {
          this.showToast(t('profile_update_fail'));
          return;
        }
        btnSaveProfile.disabled = true;
        try {
          await auth.updateProfile({
            username,
            email: user.email,
            avatarUrl: avatarUrl || user.avatarUrl || ''
          });
          this.updateUserNavUI(auth);
          this.showToast(t('profile_updated'));
        } catch (err) {
          this.showToast(err.message || t('profile_update_fail'));
        } finally {
          btnSaveProfile.disabled = false;
        }
      });
    }

    if (btnSaveEmail) {
      btnSaveEmail.addEventListener('click', async () => {
        const auth = this.settingsAuth;
        if (!auth?.isLoggedIn()) {
          this.showToast(t('profile_login_hint'));
          return;
        }
        const email = document.getElementById('settingProfileEmail')?.value.trim().toLowerCase();
        const confirm = document.getElementById('settingConfirmEmail')?.value.trim().toLowerCase();
        if (!email) {
          this.showToast(t('profile_update_fail'));
          return;
        }
        if (email !== confirm) {
          this.showToast(t('email_mismatch'));
          return;
        }
        const user = auth.getUser();
        if (email === (user.email || '').toLowerCase()) {
          this.showToast(t('email_unchanged'));
          return;
        }
        btnSaveEmail.disabled = true;
        try {
          await auth.updateProfile({
            username: user.username,
            email,
            avatarUrl: user.avatarUrl || ''
          });
          this.updateUserNavUI(auth);
          document.getElementById('settingConfirmEmail').value = email;
          this.showToast(t('email_updated'));
        } catch (err) {
          this.showToast(err.message || t('profile_update_fail'));
        } finally {
          btnSaveEmail.disabled = false;
        }
      });
    }

    if (btnChangePassword) {
      btnChangePassword.addEventListener('click', async () => {
        const auth = this.settingsAuth;
        if (!auth?.isLoggedIn()) {
          this.showToast(t('profile_login_hint'));
          return;
        }
        const current = document.getElementById('settingCurrentPassword')?.value || '';
        const next = document.getElementById('settingNewPassword')?.value || '';
        const confirm = document.getElementById('settingConfirmPassword')?.value || '';
        if (!current || !next) {
          this.showToast(t('password_change_fail'));
          return;
        }
        if (next !== confirm) {
          this.showToast(t('password_mismatch'));
          return;
        }
        btnChangePassword.disabled = true;
        try {
          await auth.changePassword(current, next);
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
        if (url) avatarPreview.src = url;
      });
      avatarPreview.addEventListener('error', () => {
        avatarPreview.src = '';
      });
    }

    updateDOMTranslations();

    window.addEventListener('twintube:languagechange', () => {
      const myRoomsModal = document.getElementById('myRoomsModal');
      if (myRoomsModal?.classList.contains('active') && this.cachedRooms?.length) {
        const query = document.getElementById('roomsSearchInput')?.value.trim().toLowerCase() || '';
        if (query) this.filterModalRooms(query);
        else this.displayModalRooms(this.cachedRooms);
      }
    });
  }

  populateSettingsForm(auth) {
    const profileSection = document.getElementById('settingsProfileSection');
    const emailSection = document.getElementById('settingsEmailSection');
    const passwordSection = document.getElementById('settingsPasswordSection');
    const loginHint = document.getElementById('settingsLoginHint');
    const loggedIn = auth?.isLoggedIn();

    if (profileSection) profileSection.hidden = !loggedIn;
    if (emailSection) emailSection.hidden = !loggedIn;
    if (passwordSection) passwordSection.hidden = !loggedIn;
    if (loginHint) loginHint.hidden = loggedIn;

    if (!loggedIn) return;

    const user = auth.getUser();
    const usernameEl = document.getElementById('settingProfileUsername');
    const emailEl = document.getElementById('settingProfileEmail');
    const confirmEmailEl = document.getElementById('settingConfirmEmail');
    const avatarEl = document.getElementById('settingProfileAvatar');
    const previewEl = document.getElementById('settingProfileAvatarPreview');

    if (usernameEl) usernameEl.value = user.username || '';
    if (emailEl) emailEl.value = user.email || '';
    if (confirmEmailEl) confirmEmailEl.value = user.email || '';
    if (avatarEl) avatarEl.value = user.avatarUrl || '';
    if (previewEl) {
      previewEl.src = user.avatarUrl || '';
      previewEl.alt = user.username || '';
    }

    ['settingCurrentPassword', 'settingNewPassword', 'settingConfirmPassword'].forEach(id => {
      const el = document.getElementById(id);
      if (el) el.value = '';
    });
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
        await this.openSettings();
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
      list.innerHTML = `<div style="padding: 20px; text-align: center; color: var(--md-on-surface-variant); font-size: 13px;">${this.escapeHTML(err.message || t('failed_load_rooms'))}</div>`;
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
          <div class="room-item-title">${this.escapeHTML(room.name || t('room_label', { id: room.id }))}</div>
          <div class="room-item-meta">
            <code>${this.escapeHTML(room.id)}</code>
            <span>•</span>
            <span>${room.hasPassword ? '🔒 ' + t('private', 'Private') : '🌐 ' + t('public', 'Public')}</span>
            <span>•</span>
            <span>${expiresLabel}</span>
          </div>
        </div>
        <div class="room-item-actions">
          <button class="btn btn-secondary btn-copy" style="font-size: 11px; padding: 4px 10px;" title="${t('copy_link_btn')}">${t('copy')}</button>
          <button class="btn btn-primary btn-open" style="font-size: 11px; padding: 4px 10px;" title="${t('open_room_btn')}">${t('open')}</button>
          <button class="btn btn-secondary btn-del" style="font-size: 11px; padding: 4px 10px; color: #e53935;" title="${t('delete_room_btn')}">${t('delete')}</button>
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
  renderQueue(playlist, options = {}) {
    const container = document.getElementById('queueList');
    const counter = document.getElementById('queueCounter');
    if (!container) return;

    const canControl = !!options.canControlPlayback;
    const canModerate = !!options.canModerateQueue;
    const onPlayItem = options.onPlayItem || (() => {});
    const onRemoveItem = options.onRemoveItem || (() => {});
    const onReorder = options.onReorder || null;

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

    playlist.forEach((item, index) => {
      const el = document.createElement('div');
      el.className = 'queue-item';
      el.dataset.itemId = item.id;
      if (canModerate) el.draggable = true;

      const actions = [];
      if (canControl) {
        actions.push(`<button class="nav-icon-btn btn-play" title="${t('play_video')}" style="width:30px; height:30px;">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="currentColor"><polygon points="5 3 19 12 5 21 5 3"/></svg>
          </button>`);
      }
      if (canModerate) {
        actions.push(`<button class="nav-icon-btn btn-remove" title="${t('remove_from_queue')}" style="width:30px; height:30px;">
            <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3 6 5 6 21 6"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>
          </button>`);
      }

      el.innerHTML = `
        ${canModerate ? `<span class="queue-drag" title="${t('reorder_queue')}" aria-hidden="true">⋮⋮</span>` : ''}
        <img class="queue-thumb" src="${queueThumbUrl(item)}" alt="Thumb">
        <div class="queue-details">
          <div class="queue-title">${this.escapeHTML(item.title)}</div>
          <div class="queue-meta">${t('queue_added_by')} ${this.escapeHTML(item.addedBy)}</div>
        </div>
        <div class="queue-actions">${actions.join('')}</div>
      `;

      const playBtn = el.querySelector('.btn-play');
      if (playBtn) playBtn.addEventListener('click', () => onPlayItem(item.id));
      const removeBtn = el.querySelector('.btn-remove');
      if (removeBtn) removeBtn.addEventListener('click', () => onRemoveItem(item.id));

      if (canModerate && onReorder) {
        el.addEventListener('dragstart', (e) => {
          e.dataTransfer.setData('text/plain', String(index));
          el.classList.add('is-dragging');
        });
        el.addEventListener('dragend', () => el.classList.remove('is-dragging'));
        el.addEventListener('dragover', (e) => {
          e.preventDefault();
          el.classList.add('drag-over');
        });
        el.addEventListener('dragleave', () => el.classList.remove('drag-over'));
        el.addEventListener('drop', (e) => {
          e.preventDefault();
          el.classList.remove('drag-over');
          const from = Number(e.dataTransfer.getData('text/plain'));
          const to = index;
          if (Number.isNaN(from) || from === to) return;
          const next = playlist.slice();
          const [moved] = next.splice(from, 1);
          next.splice(to, 0, moved);
          onReorder(next.map((q) => q.id));
        });
      }

      container.appendChild(el);
    });
  }

  // Audience Viewers Rendering
  renderViewers(users, currentClientID, options = {}) {
    const container = document.getElementById('viewersList');
    const counter = document.getElementById('viewersCounter');
    const banner = document.getElementById('localReadyBanner');
    if (!container) return;

    const isHost = !!options.isHost;
    const onTransferHost = options.onTransferHost || (() => {});
    const onGrantCohost = options.onGrantCohost || (() => {});
    const onRevokeCohost = options.onRevokeCohost || (() => {});
    const localVideoActive = !!options.localVideoActive;
    const readyCount = options.readyCount || 0;
    const totalCount = options.totalCount || users.length;

    if (banner) {
      if (localVideoActive) {
        banner.hidden = false;
        banner.className = 'local-ready-banner' + (readyCount === totalCount && totalCount > 0 ? ' is-complete' : '');
        banner.textContent = t('local_ready_summary', { ready: readyCount, total: totalCount });
      } else {
        banner.hidden = true;
        banner.textContent = '';
      }
    }

    if (counter) counter.textContent = users.length;
    container.innerHTML = '';

    users.forEach(user => {
      const el = document.createElement('div');
      el.className = 'viewer-item';
      const isYou = user.id === currentClientID;
      const initial = (user.nickname || 'G').charAt(0).toUpperCase();

      let localBadge = '';
      if (localVideoActive) {
        localBadge = user.localFileReady
          ? `<span class="badge badge-file-ready" title="${t('local_ready_title')}"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12"/></svg> ${t('local_ready_badge')}</span>`
          : `<span class="badge badge-file-missing" title="${t('local_missing_title')}"><svg viewBox="0 0 24 24" width="11" height="11" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg> ${t('local_missing_badge')}</span>`;
      }

      let roleActions = '';
      if (isHost && !user.isHost && !isYou) {
        roleActions += `<button class="btn btn-secondary btn-transfer" style="font-size: 11px; padding: 4px 10px;">${t('make_host')}</button>`;
        if (user.isCohost) {
          roleActions += `<button class="btn btn-secondary btn-revoke-cohost" style="font-size: 11px; padding: 4px 10px;">${t('remove_cohost')}</button>`;
        } else {
          roleActions += `<button class="btn btn-secondary btn-grant-cohost" style="font-size: 11px; padding: 4px 10px;">${t('make_cohost')}</button>`;
        }
      }

      el.innerHTML = `
        <div class="viewer-info">
          <div class="msg-avatar">${initial}</div>
          <div>
            <span style="font-weight: 500; font-size: 14px;">${this.escapeHTML(user.nickname)}</span>
            ${isYou ? `<span style="font-size: 11px; opacity: 0.7;"> ${t('viewer_you')}</span>` : ''}
          </div>
        </div>
        <div style="display: flex; gap: 6px; align-items: center; flex-wrap: wrap; justify-content: flex-end;">
          ${localBadge}
          ${user.isGuest ? `<span class="badge badge-guest" style="font-size: 10px; padding: 2px 6px;">${t('guest_badge')}</span>` : ''}
          ${user.isHost ? `<span class="badge badge-host"><svg viewBox="0 0 24 24" width="12" height="12" fill="currentColor"><path d="M5 16L3 5l5.5 5L12 4l3.5 6L21 5l-2 11H5zm14 3c0 .6-.4 1-1 1H6c-.6 0-1-.4-1-1v-1h14v1z"/></svg> ${t('host_badge')}</span>` : ''}
          ${user.isCohost && !user.isHost ? `<span class="badge badge-cohost">${t('cohost_badge')}</span>` : ''}
          ${user.voiceJoined ? `<span class="badge badge-voice ${user.voiceSpeaking ? 'is-speaking' : ''}" title="${t('voice_connected')}">${user.voiceSpeaking ? t('voice_speaking') : (user.voiceMuted ? t('voice_muted') : t('voice_connected'))}</span>` : ''}
          ${roleActions}
        </div>
      `;

      const btnTransfer = el.querySelector('.btn-transfer');
      if (btnTransfer) btnTransfer.addEventListener('click', () => onTransferHost(user.id));
      const btnGrant = el.querySelector('.btn-grant-cohost');
      if (btnGrant) btnGrant.addEventListener('click', () => onGrantCohost(user.id));
      const btnRevoke = el.querySelector('.btn-revoke-cohost');
      if (btnRevoke) btnRevoke.addEventListener('click', () => onRevokeCohost(user.id));

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

  formatMomentClock(seconds) {
    const total = Math.max(0, Math.round(Number(seconds) || 0));
    const m = Math.floor(total / 60);
    const s = total % 60;
    return `${m}:${String(s).padStart(2, '0')}`;
  }

  renderMoments(approved = [], pending = [], options = {}) {
    const approvedEl = document.getElementById('momentsApproved');
    const pendingEl = document.getElementById('momentsPending');
    if (!approvedEl || !pendingEl) return;

    const canControl = !!options.canControlPlayback;
    const onJump = options.onJump || (() => {});
    const onApprove = options.onApprove || (() => {});
    const onReject = options.onReject || (() => {});

    approvedEl.innerHTML = '';
    approved.forEach((m) => {
      const chip = document.createElement('button');
      chip.type = 'button';
      chip.className = 'moment-chip';
      chip.title = canControl ? t('jump_to_moment') : t('moment_host_only');
      const label = m.label ? ` · ${this.escapeHTML(m.label)}` : '';
      chip.innerHTML = `<span class="moment-time">${this.formatMomentClock(m.atSeconds)}</span>${label}`;
      chip.addEventListener('click', () => {
        if (!canControl) {
          this.showToast(t('moment_host_only'));
          return;
        }
        onJump(m.id);
      });
      approvedEl.appendChild(chip);
    });

    pendingEl.innerHTML = '';
    if (!canControl || !pending.length) {
      pendingEl.hidden = true;
      return;
    }
    pendingEl.hidden = false;
    pending.forEach((m) => {
      const row = document.createElement('div');
      row.className = 'moment-pending-row';
      row.innerHTML = `
        <span>${this.escapeHTML(m.creatorNickname)} · ${this.formatMomentClock(m.atSeconds)}${m.label ? ` · ${this.escapeHTML(m.label)}` : ''}</span>
        <span class="moment-pending-actions">
          <button type="button" class="btn btn-primary btn-sm btn-approve">${t('approve_moment')}</button>
          <button type="button" class="btn btn-secondary btn-sm btn-reject">${t('reject_moment')}</button>
        </span>
      `;
      row.querySelector('.btn-approve').addEventListener('click', () => onApprove(m.id));
      row.querySelector('.btn-reject').addEventListener('click', () => onReject(m.id));
      pendingEl.appendChild(row);
    });
  }
}
