/* TwinTube - Landing Page Logic */

import { UIManager } from './ui.js';
import { AuthManager } from './auth.js';
import { t, translateError } from './i18n.js';

class LandingApp {
  constructor() {
    this.ui = new UIManager();
    this.auth = new AuthManager();
    this.pendingJoinCode = '';

    this.ui.bindSettingsAuth(this.auth);
    this.init();
  }

  consumeFlashToast() {
    let key = '';
    try {
      key = sessionStorage.getItem('twintube_flash_toast') || '';
      if (key) sessionStorage.removeItem('twintube_flash_toast');
    } catch (_) {
      return;
    }
    if (!key) return;
    // Defer so the toast container is ready after auth/UI init.
    setTimeout(() => {
      this.ui.showToast(translateError(key, key));
    }, 50);
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
    this.setupJoinRoom();
    this.setupAuthUI();
    await this.auth.checkAuth();
    this.setupCreateRoom();
    this.updateAuthNavUI();
    this.consumeFlashToast();

    this.ui.initMyRoomsModal(
      async () => {
        const resp = await fetch('/api/rooms/mine', { headers: this.authHeaders() });
        if (!resp.ok) {
          const body = await resp.json();
          throw new Error(body.error || t('failed_load_rooms'));
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
        if (!del.ok) throw new Error(body.error || t('failed_delete'));
      },
      () => this.openCreateRoomModal()
    );
  }

  openCreateRoomModal() {
    if (this.auth.isLoggedIn()) {
      this.ui.showModal('createRoomModal');
    } else {
      this.createRoomRequest({});
    }
  }

  joinTokenKey(roomId) {
    return `twintube_join_token_${roomId}`;
  }

  async requestRoomAccess(roomId, password = '') {
    const resp = await fetch(`/api/room/${encodeURIComponent(roomId)}/access`, {
      method: 'POST',
      headers: this.authHeaders(true),
      body: JSON.stringify(password ? { password } : {})
    });
    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || t('access_denied'));
    }
    if (data.joinToken) {
      sessionStorage.setItem(this.joinTokenKey(roomId), data.joinToken);
    }
    return data;
  }

  async createRoomRequest({ name = '', password = '' } = {}) {
    const resp = await fetch('/api/room/create', {
      method: 'POST',
      headers: this.authHeaders(true),
      body: JSON.stringify({ name, password })
    });
    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || t('failed_create_room'));
    }
    if (password && data.roomCode) {
      await this.requestRoomAccess(data.roomCode, password);
    }
    if (data.url) {
      window.location.href = data.url;
    } else {
      throw new Error('Invalid room response');
    }
  }

  setupCreateRoom() {
    const btnCreate = document.getElementById('btnLandingCreateRoom');
    const formCreate = document.getElementById('formCreateRoom');
    const btnCloseCreate = document.getElementById('btnCloseCreateRoom');

    if (btnCreate) {
      btnCreate.addEventListener('click', async () => {
        if (this.auth.isLoggedIn()) {
          this.openCreateRoomModal();
          return;
        }
        btnCreate.disabled = true;
        try {
          await this.createRoomRequest({});
        } catch (err) {
          this.ui.showToast(err.message || t('failed_create_room'));
          btnCreate.disabled = false;
        }
      });
    }

    if (btnCloseCreate) {
      btnCloseCreate.addEventListener('click', () => this.ui.hideModal('createRoomModal'));
    }

    if (formCreate) {
      formCreate.addEventListener('submit', async (e) => {
        e.preventDefault();
        const name = document.getElementById('createRoomName')?.value.trim() || '';
        const password = document.getElementById('createRoomPassword')?.value || '';
        const btn = document.getElementById('btnConfirmCreateRoom');
        if (btn) btn.disabled = true;
        try {
          await this.createRoomRequest({ name, password });
        } catch (err) {
          this.ui.showToast(err.message || t('failed_create_room'));
          if (btn) btn.disabled = false;
        }
      });
    }
  }

  setupJoinRoom() {
    const joinForm = document.getElementById('joinRoomForm');
    const joinInput = document.getElementById('joinRoomCodeInput');
    const formJoinPassword = document.getElementById('formJoinPassword');
    const btnCloseJoinPassword = document.getElementById('btnCloseJoinPassword');
    const roomCodePattern = /^[a-zA-Z0-9_-]{4,32}$/;

    if (joinForm && joinInput) {
      joinForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        let code = joinInput.value.trim();
        const match = code.match(/\/room\/([a-zA-Z0-9_-]+)/);
        if (match && match[1]) code = match[1];
        if (!code) return;

        if (!roomCodePattern.test(code)) {
          this.ui.showToast(t('invalid_room_code'));
          return;
        }

        try {
          const resp = await fetch(`/api/room/${encodeURIComponent(code)}/info`, {
            headers: this.authHeaders()
          });
          let info = {};
          try {
            info = await resp.json();
          } catch {
            info = {};
          }

          if (!resp.ok) {
            this.ui.showToast(translateError(info.error, 'room_not_found'));
            return;
          }

          if (info.expired) {
            this.ui.showToast(t('room_expired'));
            return;
          }

          if (!info.exists) {
            this.ui.showToast(t('room_not_found'));
            return;
          }

          if (info.isOwner || !info.requiresPassword) {
            window.location.href = `/room/${code}`;
            return;
          }

          this.pendingJoinCode = code;
          const label = document.getElementById('joinPasswordRoomName');
          if (label) label.textContent = t('room_password_protected', { name: info.name || code });
          this.ui.showModal('joinPasswordModal');
        } catch (err) {
          this.ui.showToast(err.message ? translateError(err.message, 'room_not_found') : t('room_not_found'));
        }
      });
    }

    if (formJoinPassword) {
      formJoinPassword.addEventListener('submit', async (e) => {
        e.preventDefault();
        const pwd = document.getElementById('joinRoomPasswordInput')?.value || '';
        if (!this.pendingJoinCode || !pwd) return;
        try {
          await this.requestRoomAccess(this.pendingJoinCode, pwd);
          window.location.href = `/room/${this.pendingJoinCode}`;
        } catch (err) {
          this.ui.showToast(err.message || t('password_required'));
        }
      });
    }

    if (btnCloseJoinPassword) {
      btnCloseJoinPassword.addEventListener('click', () => {
        this.ui.hideModal('joinPasswordModal');
        this.pendingJoinCode = '';
      });
    }
  }

  setupAuthUI() {
    const btnOpenLogin = document.getElementById('btnOpenAuthLogin');
    const btnOpenRegister = document.getElementById('btnOpenAuthRegister');
    const btnCloseAuth = document.getElementById('btnCloseAuth');

    const btnTabLogin = document.getElementById('btnTabLogin');
    const btnTabRegister = document.getElementById('btnTabRegister');
    const formLogin = document.getElementById('formLogin');
    const formRegister = document.getElementById('formRegister');

    if (btnOpenLogin) {
      btnOpenLogin.addEventListener('click', () => {
        if (btnTabLogin) btnTabLogin.click();
        this.ui.showAuthModal();
      });
    }

    if (btnOpenRegister) {
      btnOpenRegister.addEventListener('click', () => {
        if (btnTabRegister) btnTabRegister.click();
        this.ui.showAuthModal();
      });
    }

    if (btnCloseAuth) btnCloseAuth.addEventListener('click', () => this.ui.hideAuthModal());

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
        } catch (err) {
          this.ui.showToast(err.message || t('login_failed'));
        }
      });
    }

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
  }
}

document.addEventListener('DOMContentLoaded', () => {
  window.landingApp = new LandingApp();
});
