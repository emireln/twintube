/* TwinTube - Main Application Orchestrator */

import { UIManager } from './ui.js';
import { WSClient } from './ws.js';
import { VideoPlayer } from './player.js';
import { AuthManager } from './auth.js';
import { t } from './i18n.js';

class TwinTubeApp {
  constructor() {
    this.ui = new UIManager();
    this.ws = new WSClient();
    this.auth = new AuthManager();
    this.player = null;

    this.ui.bindSettingsAuth(this.auth);

    this.roomId = this.extractRoomId();
    this.nickname = localStorage.getItem('twintube_nickname') || '';
    this.currentClientID = '';
    this.isHost = false;
    this.hasJoined = false;
    this.accessGranted = false;
    this.playlist = [];
    this.forceNextSync = false;

    document.body.classList.add('room-gated');
    this.init();
  }

  extractRoomId() {
    const path = window.location.pathname;
    const match = path.match(/\/room\/([a-zA-Z0-9_-]+)/);
    if (match && match[1]) {
      return match[1];
    }
    return 'main';
  }

  authHeaders(json = false) {
    const headers = {};
    if (json) headers['Content-Type'] = 'application/json';
    const token = this.auth.getToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    return headers;
  }

  async init() {
    // 1. Check authentication state first
    await this.auth.checkAuth();
    if (this.auth.isLoggedIn()) {
      const user = this.auth.getUser();
      if (user && user.username) {
        this.nickname = user.username;
      }
    }

    // 2. Default guest nickname if empty so guest can interact immediately
    if (!this.nickname) {
      this.nickname = localStorage.getItem('twintube_nickname') || ('Guest_' + Math.floor(1000 + Math.random() * 9000));
      localStorage.setItem('twintube_nickname', this.nickname);
    }

    this.setupUI();
    this.setupAuthUI();
    this.updateAuthNavUI();

    this.ui.initMyRoomsModal(
      async () => {
        const resp = await fetch('/api/rooms/mine', { headers: this.authHeaders() });
        if (!resp.ok) {
          const body = await resp.json();
          throw new Error(body.error || 'Failed to load rooms');
        }
        const data = await resp.json();
        return data.rooms || [];
      },
      async (roomId) => {
        const del = await fetch(`/api/rooms/${encodeURIComponent(roomId)}`, {
          method: 'DELETE',
          headers: this.authHeaders()
        });
        const body = await del.json();
        if (!del.ok) throw new Error(body.error || 'Delete failed');
      },
      () => {
        window.location.href = '/';
      }
    );

    const accessOk = await this.resolveRoomAccess();
    if (!accessOk) return;

    this.accessGranted = true;

    // Connect WebSocket after password gate passes
    this.initWebSocket();

    // Initialize player inside try/catch so player errors never block socket logic
    try {
      this.initPlayer();
    } catch (err) {
      console.error('[APP] Player init error:', err);
    }
  }

  setupUI() {
    // Render Room Code Chip
    // Room Code Chip & Copy Link Button
    const btnCopy = document.getElementById('btnCopyRoom');
    const roomCodeText = document.getElementById('roomCodeText');
    if (btnCopy && roomCodeText) {
      btnCopy.style.display = 'inline-flex';
      roomCodeText.textContent = this.roomId;
      btnCopy.addEventListener('click', () => {
        const fullURL = `${window.location.origin}/room/${this.roomId}`;
        navigator.clipboard.writeText(fullURL).then(() => {
          this.ui.showToast(t('link_copied'));
        });
      });
    }

    // New Room Button
    const btnNewRoom = document.getElementById('btnNewRoom');
    if (btnNewRoom) {
      btnNewRoom.addEventListener('click', async () => {
        try {
          const resp = await fetch('/api/room/create', {
            method: 'POST',
            headers: this.authHeaders(true),
            body: JSON.stringify({})
          });
          const data = await resp.json();
          if (!resp.ok) throw new Error(data.error || 'Failed');
          if (data.url) {
            window.location.href = data.url;
          }
        } catch (err) {
          this.ui.showToast(err.message || 'Failed to create new room');
        }
      });
    }

    // Manual Resync Button — force full video reload
    const btnResync = document.getElementById('btnResync');
    if (btnResync) {
      btnResync.addEventListener('click', () => {
        this.forceNextSync = true;
        this.ws.sendAction('SYNC_REQUEST');
        this.ui.showToast('Resyncing video…');
      });
    }

    // Theater Mode Toggle
    const btnTheaterMode = document.getElementById('btnTheaterMode');
    if (btnTheaterMode) {
      btnTheaterMode.addEventListener('click', () => {
        document.body.classList.toggle('theater-mode');
        const isTheater = document.body.classList.contains('theater-mode');
        btnTheaterMode.classList.toggle('is-active', isTheater);
        this.ui.showToast(isTheater ? 'Theater mode enabled' : 'Theater mode disabled');
      });
    }

    // Video Noto GIF Reactions
    const reactionButtons = document.querySelectorAll('#videoReactionBar .reaction-btn');
    reactionButtons.forEach(btn => {
      btn.addEventListener('click', () => {
        btn.classList.add('is-active');
        setTimeout(() => btn.classList.remove('is-active'), 250);
        const gif = btn.getAttribute('data-gif');
        if (gif) {
          this.ws.sendAction('VIDEO_REACTION', { reaction: gif });
        }
      });
    });

    // Top Video Link Form Submission
    const topVideoForm = document.getElementById('topVideoForm');
    const topVideoInput = document.getElementById('topVideoInput');
    if (topVideoForm && topVideoInput) {
      topVideoForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const url = topVideoInput.value.trim();
        if (url) {
          if (this.ws.sendAction('ADD_QUEUE', { url })) {
            topVideoInput.value = '';
            this.ui.showToast('Adding video to queue…');
          } else {
            this.ui.showToast('Not connected — try again in a moment.');
          }
        }
      });
    }

    // Chat Form Submission
    const chatForm = document.getElementById('chatForm');
    const chatInput = document.getElementById('chatInput');
    if (chatForm && chatInput) {
      chatForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const content = chatInput.value.trim();
        if (content) {
          this.ws.sendAction('CHAT_MESSAGE', { content });
          chatInput.value = '';
        }
      });
    }

    // Emoji Picker Toggle & Selection
    const btnEmojiPicker = document.getElementById('btnEmojiPicker');
    const emojiPicker = document.getElementById('emojiPicker');
    if (btnEmojiPicker && emojiPicker && chatInput) {
      btnEmojiPicker.addEventListener('click', (e) => {
        e.stopPropagation();
        const isOpen = emojiPicker.classList.toggle('active');
        btnEmojiPicker.classList.toggle('is-active', isOpen);
      });

      emojiPicker.addEventListener('click', (e) => {
        if (e.target.tagName === 'SPAN') {
          const emoji = e.target.textContent;
          chatInput.value += emoji;
          chatInput.focus();
          emojiPicker.classList.remove('active');
          btnEmojiPicker.classList.remove('is-active');
        }
      });

      document.addEventListener('click', () => {
        emojiPicker.classList.remove('active');
        btnEmojiPicker.classList.remove('is-active');
      });
    }

    const btnSaveNickname = document.getElementById('btnSaveNickname');
    const nicknameInput = document.getElementById('nicknameInput');
    if (btnSaveNickname && nicknameInput) {
      if (this.nickname) {
        nicknameInput.value = this.nickname;
      }
      btnSaveNickname.addEventListener('click', () => {
        const val = nicknameInput.value.trim();
        if (val) {
          this.nickname = val;
          localStorage.setItem('twintube_nickname', val);
          this.ui.hideNicknameModal();
          this.joinRoom();
        }
      });
    }

    const btnOpenAuthFromNick = document.getElementById('btnOpenAuthFromNick');
    if (btnOpenAuthFromNick) {
      btnOpenAuthFromNick.addEventListener('click', () => {
        this.ui.hideNicknameModal();
        this.ui.showAuthModal();
      });
    }
  }

  setupAuthUI() {
    const btnOpenAuth = document.getElementById('btnOpenAuth');
    const btnCloseAuth = document.getElementById('btnCloseAuth');
    if (btnOpenAuth) btnOpenAuth.addEventListener('click', () => this.ui.showAuthModal());
    if (btnCloseAuth) btnCloseAuth.addEventListener('click', () => this.ui.hideAuthModal());

    // Auth Modal Tabs (Login / Register)
    const btnTabLogin = document.getElementById('btnTabLogin');
    const btnTabRegister = document.getElementById('btnTabRegister');
    const formLogin = document.getElementById('formLogin');
    const formRegister = document.getElementById('formRegister');

    if (btnTabLogin && btnTabRegister) {
      btnTabLogin.addEventListener('click', () => {
        btnTabLogin.classList.add('active');
        btnTabRegister.classList.remove('active');
        formLogin.style.display = 'flex';
        formRegister.style.display = 'none';
      });

      btnTabRegister.addEventListener('click', () => {
        btnTabRegister.classList.add('active');
        btnTabLogin.classList.remove('active');
        formRegister.style.display = 'flex';
        formLogin.style.display = 'none';
      });
    }

    // Submit Login Form
    if (formLogin) {
      formLogin.addEventListener('submit', async (e) => {
        e.preventDefault();
        const usernameOrEmail = document.getElementById('loginUser').value.trim();
        const password = document.getElementById('loginPass').value.trim();

        try {
          await this.auth.login(usernameOrEmail, password);
          this.ui.hideAuthModal();
          this.ui.showToast(t('signed_in_success'));
          this.updateAuthNavUI();
          this.joinRoom();
        } catch (err) {
          this.ui.showToast(err.message || t('login_failed'));
        }
      });
    }

    // Submit Register Form
    if (formRegister) {
      formRegister.addEventListener('submit', async (e) => {
        e.preventDefault();
        const username = document.getElementById('regUser').value.trim();
        const email = document.getElementById('regEmail').value.trim();
        const password = document.getElementById('regPass').value.trim();

        try {
          await this.auth.register(username, email, password);
          this.ui.hideAuthModal();
          this.ui.showToast(t('account_created'));
          this.updateAuthNavUI();
          this.joinRoom();
        } catch (err) {
          this.ui.showToast(err.message || t('register_failed'));
        }
      });
    }

    const btnUserMenu = document.getElementById('btnUserMenu');
    const profileDropdown = document.getElementById('profileDropdown');
    const btnLogout = document.getElementById('btnLogout');

    if (btnUserMenu && profileDropdown) {
      btnUserMenu.addEventListener('click', (e) => {
        e.stopPropagation();
        profileDropdown.classList.toggle('active');
      });

      document.addEventListener('click', () => {
        profileDropdown.classList.remove('active');
      });
    }

    if (btnLogout) {
      btnLogout.addEventListener('click', () => {
        this.auth.logout();
        this.ui.showToast(t('signed_out'));
        this.updateAuthNavUI();
      });
    }

    this.auth.onChange(() => this.updateAuthNavUI());
  }

  updateAuthNavUI() {
    this.ui.updateUserNavUI(this.auth);
    if (this.auth.isLoggedIn()) {
      const user = this.auth.getUser();
      if (user?.username) {
        this.nickname = user.username;
        localStorage.setItem('twintube_nickname', this.nickname);
      }
    }
  }

  initPlayer() {
    this.player = new VideoPlayer(
      (status, currentTime, videoId) => {
        this.ws.sendAction('STATE_CHANGE', {
          videoId,
          status,
          currentTime
        });
      },
      () => {
        if (this.playlist && this.playlist.length > 0) {
          const nextItem = this.playlist[0];
          this.ws.sendAction('PLAY_QUEUE_ITEM', { itemId: nextItem.id });
        }
      }
    );
  }

  initWebSocket() {
    this.ws.on('open', () => {
      if (this.accessGranted) {
        this.joinRoom();
      }
    });

    this.ws.on('INIT_STATE', (payload, timestamp) => {
      this.hasJoined = true;
      document.body.classList.remove('room-gated');
      this.currentClientID = payload.clientId || '';
      this.isHost = payload.isHost;
      this.playlist = payload.playlist || [];

      const hostBadge = document.getElementById('hostBadge');
      if (hostBadge) {
        hostBadge.style.display = this.isHost ? 'inline-flex' : 'none';
      }

      if (payload.video) {
        this.updateVideoMeta(payload.video.title, payload.video.status);
        if (this.player) {
          this.player.applyServerState(
            payload.video,
            payload.video.serverTimestamp || timestamp,
            { force: true, timeAlreadyAbsolute: true }
          );
        }
      }

      this.renderQueue();
      this.ui.renderViewers(payload.users || [], this.currentClientID, this.isHost, (targetId) => {
        this.ws.sendAction('TRANSFER_HOST', { targetId });
      });
    });

    this.ws.on('STATE_UPDATE', (payload, timestamp) => {
      this.updateVideoMeta(payload.title, payload.status);
      if (this.player) {
        const force = this.forceNextSync || !!payload.forceReload;
        this.forceNextSync = false;
        this.player.applyServerState(
          payload,
          payload.serverTimestamp || timestamp,
          { force, timeAlreadyAbsolute: force || !!payload.forceReload }
        );
      }
    });

    this.ws.on('CHAT_MESSAGE', (msg) => {
      this.ui.renderChatMessage(msg);
    });

    this.ws.on('CHAT_HISTORY', (history) => {
      if (Array.isArray(history)) {
        history.forEach(msg => this.ui.renderChatMessage(msg));
      }
    });

    this.ws.on('QUEUE_UPDATE', (playlist) => {
      this.playlist = playlist || [];
      this.renderQueue();
    });

    this.ws.on('USER_LIST', (users) => {
      this.ui.renderViewers(users || [], this.currentClientID, this.isHost, (targetId) => {
        this.ws.sendAction('TRANSFER_HOST', { targetId });
      });
    });

    this.ws.on('VIDEO_REACTION', (payload) => {
      if (payload && payload.reaction) {
        this.renderFloatingReaction(payload.reaction, payload.nickname);
      }
    });

    this.ws.on('ERROR', (payload) => {
      if (payload && payload.message) {
        const msg = String(payload.message).toLowerCase();

        if (msg.includes('password') || msg.includes('access denied') || msg.includes('verify the password')) {
          sessionStorage.removeItem(this.joinTokenKey(this.roomId));
          this.hasJoined = false;
          document.body.classList.add('room-gated');

          if (msg.includes('too many')) {
            this.ui.showToast(payload.message);
            setTimeout(() => { window.location.href = '/'; }, 1200);
            return;
          }

          this.handleWrongRoomAccess(payload.message);
          return;
        }

        if (msg.includes('not found')) {
          this.ui.showToast(payload.message || t('room_not_found'));
          setTimeout(() => { window.location.href = '/'; }, 1200);
          return;
        }

        this.ui.showToast(payload.message);
        if (msg.includes('expired')) {
          sessionStorage.removeItem(this.joinTokenKey(this.roomId));
          setTimeout(() => { window.location.href = '/'; }, 1200);
        }
      }
    });

    this.ws.connect();
  }

  async fetchRoomInfo() {
    try {
      const resp = await fetch(`/api/room/${encodeURIComponent(this.roomId)}/info`, {
        headers: this.authHeaders()
      });
      let data = null;
      try {
        data = await resp.json();
      } catch {
        data = null;
      }
      if (!resp.ok) {
        return { error: data?.error || 'Room not found.', exists: false };
      }
      return data;
    } catch {
      return null;
    }
  }

  joinTokenKey(roomId = this.roomId) {
    return `twintube_join_token_${roomId}`;
  }

  async requestRoomAccess(roomId, password = '') {
    const resp = await fetch(`/api/room/${encodeURIComponent(roomId)}/access`, {
      method: 'POST',
      headers: this.authHeaders(true),
      body: JSON.stringify(password ? { password } : {})
    });
    let data = {};
    try {
      data = await resp.json();
    } catch {
      data = {};
    }
    if (!resp.ok) {
      throw new Error(data.error || 'Access denied');
    }
    if (data.joinToken) {
      sessionStorage.setItem(this.joinTokenKey(roomId), data.joinToken);
    }
    return data;
  }

  async resolveRoomAccess() {
    const info = await this.fetchRoomInfo();
    if (!info) {
      this.ui.showToast('Could not verify room access.');
      setTimeout(() => { window.location.href = '/'; }, 1200);
      return false;
    }

    if (info.error && !info.exists) {
      this.ui.showToast(info.error || t('room_not_found'));
      setTimeout(() => { window.location.href = '/'; }, 1200);
      return false;
    }

    if (info.expired) {
      this.ui.showToast(t('room_expired') || 'This room has expired.');
      setTimeout(() => { window.location.href = '/'; }, 1200);
      return false;
    }

    if (!info.exists) {
      this.ui.showToast(t('room_not_found'));
      setTimeout(() => { window.location.href = '/'; }, 1200);
      return false;
    }

    if (info.isOwner) {
      sessionStorage.removeItem(this.joinTokenKey());
      return true;
    }

    if (!info.requiresPassword) {
      sessionStorage.removeItem(this.joinTokenKey());
      return true;
    }

    let joinToken = sessionStorage.getItem(this.joinTokenKey());
    if (!joinToken) {
      const pwd = await this.promptRoomPassword(info.name || this.roomId);
      if (!pwd) {
        window.location.href = '/';
        return false;
      }
      try {
        await this.requestRoomAccess(this.roomId, pwd);
      } catch (err) {
        this.ui.showToast(err.message || t('password_required'));
        return this.resolveRoomAccess();
      }
    }

    return true;
  }

  async handleWrongRoomAccess(message) {
    this.ui.showToast(message || t('password_required'));

    const info = await this.fetchRoomInfo();
    const roomName = info?.name || this.roomId;
    const pwd = await this.promptRoomPassword(roomName);
    if (!pwd) {
      window.location.href = '/';
      return;
    }

    try {
      await this.requestRoomAccess(this.roomId, pwd);
      this.joinRoom();
    } catch (err) {
      this.ui.showToast(err.message || t('password_required'));
      await this.handleWrongRoomAccess(err.message);
    }
  }

  promptRoomPassword(roomName) {
    return new Promise((resolve) => {
      const modal = document.getElementById('joinPasswordModal');
      const label = document.getElementById('joinPasswordRoomName');
      const input = document.getElementById('joinRoomPasswordInput');
      const form = document.getElementById('formJoinPassword');
      const btnClose = document.getElementById('btnCloseJoinPassword');

      if (!modal || !form || !input) {
        resolve('');
        return;
      }

      if (label) {
        label.textContent = `"${roomName}" is password-protected.`;
      }
      input.value = '';

      const cleanup = () => {
        form.removeEventListener('submit', onSubmit);
        if (btnClose) btnClose.removeEventListener('click', onCancel);
      };

      const onSubmit = (e) => {
        e.preventDefault();
        const pwd = input.value.trim();
        if (pwd.length < 4) return;
        this.ui.hideModal('joinPasswordModal');
        cleanup();
        resolve(pwd);
      };

      const onCancel = () => {
        this.ui.hideModal('joinPasswordModal');
        cleanup();
        resolve('');
      };

      form.addEventListener('submit', onSubmit);
      if (btnClose) btnClose.addEventListener('click', onCancel);
      this.ui.showModal('joinPasswordModal');
      input.focus();
    });
  }

  renderFloatingReaction(reaction, nickname) {
    const allowed = new Set(['happy.webp', 'energetic.webp', 'stressed.webp', 'tired.webp']);
    if (!allowed.has(reaction)) return;

    const overlay = document.getElementById('reactionOverlay');
    if (!overlay) return;

    const el = document.createElement('div');
    el.className = 'floating-reaction';
    el.style.left = `${Math.floor(Math.random() * 70) + 15}%`;

    el.innerHTML = `
      <img src="/static/gifs/${this.ui.escapeHTML(reaction)}" alt="Reaction">
      <span class="reaction-user">${this.ui.escapeHTML(nickname || 'Guest')}</span>
    `;

    overlay.appendChild(el);
    setTimeout(() => el.remove(), 2500);
  }

  joinRoom() {
    const payload = {
      roomId: this.roomId,
      nickname: this.nickname
    };

    if (this.auth.isLoggedIn()) {
      payload.token = this.auth.getToken();
    }

    const joinToken = sessionStorage.getItem(this.joinTokenKey());
    if (joinToken) {
      payload.joinToken = joinToken;
    }

    this.ws.sendAction('JOIN_ROOM', payload);
  }

  renderQueue() {
    this.ui.renderQueue(
      this.playlist,
      this.isHost,
      (itemId) => this.ws.sendAction('PLAY_QUEUE_ITEM', { itemId }),
      (itemId) => this.ws.sendAction('REMOVE_QUEUE_ITEM', { itemId })
    );
  }

  updateVideoMeta(title, status) {
    const videoTitle = document.getElementById('videoTitle');
    const statusBadge = document.getElementById('statusBadge');
    const statusText = document.getElementById('statusText');

    if (videoTitle && title) {
      videoTitle.textContent = title;
    }

    if (statusBadge && statusText) {
      if (status === 'PLAYING') {
        statusBadge.className = 'badge badge-playing';
        statusText.textContent = 'PLAYING';
      } else {
        statusBadge.className = 'badge badge-paused';
        statusText.textContent = 'PAUSED';
      }
    }
  }
}

document.addEventListener('DOMContentLoaded', () => {
  window.app = new TwinTubeApp();
});
