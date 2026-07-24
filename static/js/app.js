/* TwinTube - Main Application Orchestrator */

import { UIManager } from './ui.js';
import { WSClient } from './ws.js';
import { VideoPlayer } from './player.js';
import { AuthManager } from './auth.js';

class TwinTubeApp {
  constructor() {
    this.ui = new UIManager();
    this.ws = new WSClient();
    this.auth = new AuthManager();
    this.player = null;

    this.roomId = this.extractRoomId();
    this.nickname = localStorage.getItem('twintube_nickname') || '';
    this.currentClientID = '';
    this.isHost = false;
    this.playlist = [];

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

  async init() {
    this.setupUI();
    this.setupAuthUI();
    this.initPlayer();
    
    // Check saved authentication state
    await this.auth.checkAuth();
    this.updateAuthNavUI();

    this.initWebSocket();
  }

  setupUI() {
    // Render Room Code Chip
    const roomChip = document.getElementById('roomChip');
    const roomCodeText = document.getElementById('roomCodeText');
    if (roomChip && roomCodeText) {
      roomChip.style.display = 'flex';
      roomCodeText.textContent = this.roomId;
    }

    // Copy Room Link Button
    const btnCopy = document.getElementById('btnCopyRoom');
    if (btnCopy) {
      btnCopy.addEventListener('click', () => {
        const fullURL = `${window.location.origin}/room/${this.roomId}`;
        navigator.clipboard.writeText(fullURL).then(() => {
          this.ui.showToast('Room link copied to clipboard!');
        });
      });
    }

    // New Room Button
    const btnNewRoom = document.getElementById('btnNewRoom');
    if (btnNewRoom) {
      btnNewRoom.addEventListener('click', async () => {
        try {
          const resp = await fetch('/api/room/create');
          const data = await resp.json();
          if (data.url) {
            window.location.href = data.url;
          }
        } catch (err) {
          this.ui.showToast('Failed to create new room');
        }
      });
    }

    // Manual Resync Button
    const btnResync = document.getElementById('btnResync');
    if (btnResync) {
      btnResync.addEventListener('click', () => {
        this.ws.sendAction('SYNC_REQUEST');
        this.ui.showToast('Resync request sent to server.');
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

    // Queue Form Submission
    const queueForm = document.getElementById('queueForm');
    const queueInput = document.getElementById('queueInput');
    if (queueForm && queueInput) {
      queueForm.addEventListener('submit', (e) => {
        e.preventDefault();
        const url = queueInput.value.trim();
        if (url) {
          this.ws.sendAction('ADD_QUEUE', { url });
          queueInput.value = '';
          this.ui.showToast('Added video to queue.');
        }
      });
    }

    // Nickname Modal Handling (for guests)
    if (!this.nickname && !this.auth.isLoggedIn()) {
      this.ui.showNicknameModal();
    }

    const btnSaveNickname = document.getElementById('btnSaveNickname');
    const nicknameInput = document.getElementById('nicknameInput');
    if (btnSaveNickname && nicknameInput) {
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
          this.ui.showToast('Successfully signed in!');
          this.joinRoom();
        } catch (err) {
          this.ui.showToast(err.message || 'Login failed');
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
          this.ui.showToast('Account created successfully!');
          this.joinRoom();
        } catch (err) {
          this.ui.showToast(err.message || 'Registration failed');
        }
      });
    }

    // Dropdown Profile Toggle
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
        this.ui.showToast('Signed out.');
        window.location.reload();
      });
    }

    this.auth.onChange(() => this.updateAuthNavUI());
  }

  updateAuthNavUI() {
    const btnOpenAuth = document.getElementById('btnOpenAuth');
    const userProfileMenu = document.getElementById('userProfileMenu');
    const userAvatarText = document.getElementById('userAvatarText');
    const dropdownUsername = document.getElementById('dropdownUsername');
    const dropdownEmail = document.getElementById('dropdownEmail');

    if (this.auth.isLoggedIn()) {
      const user = this.auth.getUser();
      if (btnOpenAuth) btnOpenAuth.style.display = 'none';
      if (userProfileMenu) userProfileMenu.style.display = 'block';
      if (userAvatarText) userAvatarText.textContent = (user.username || 'U').charAt(0).toUpperCase();
      if (dropdownUsername) dropdownUsername.textContent = user.username;
      if (dropdownEmail) dropdownEmail.textContent = user.email;
    } else {
      if (btnOpenAuth) btnOpenAuth.style.display = 'inline-flex';
      if (userProfileMenu) userProfileMenu.style.display = 'none';
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
      this.joinRoom();
    });

    this.ws.on('INIT_STATE', (payload, timestamp) => {
      this.isHost = payload.isHost;
      this.playlist = payload.playlist || [];

      const hostBadge = document.getElementById('hostBadge');
      if (hostBadge) {
        hostBadge.style.display = this.isHost ? 'inline-flex' : 'none';
      }

      if (payload.video) {
        this.updateVideoMeta(payload.video.title, payload.video.status);
        this.player.applyServerState(payload.video, timestamp);
      }

      this.renderQueue();
      this.ui.renderViewers(payload.users || [], this.currentClientID, this.isHost, (targetId) => {
        this.ws.sendAction('TRANSFER_HOST', { targetId });
      });
    });

    this.ws.on('STATE_UPDATE', (payload, timestamp) => {
      this.updateVideoMeta(payload.title, payload.status);
      this.player.applyServerState(payload, timestamp);
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

    this.ws.connect();
  }

  joinRoom() {
    const payload = {
      roomId: this.roomId,
      nickname: this.nickname
    };

    if (this.auth.isLoggedIn()) {
      payload.token = this.auth.getToken();
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
