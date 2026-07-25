/* TwinTube - Landing Page Logic */

import { UIManager } from './ui.js';
import { AuthManager } from './auth.js';
import { t } from './i18n.js';

class LandingApp {
  constructor() {
    this.ui = new UIManager();
    this.auth = new AuthManager();
    this.pendingJoinCode = '';

    this.ui.bindSettingsAuth(this.auth);
    this.init();
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

  async createRoomRequest({ name = '', password = '' } = {}) {
    const resp = await fetch('/api/room/create', {
      method: 'POST',
      headers: this.authHeaders(true),
      body: JSON.stringify({ name, password })
    });
    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || 'Failed to create room');
    }
    if (password && data.roomCode) {
      sessionStorage.setItem(`twintube_room_pwd_${data.roomCode}`, password);
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
          this.ui.showToast(err.message || 'Failed to create room.');
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
          this.ui.showToast(err.message || 'Failed to create room.');
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

    if (joinForm && joinInput) {
      joinForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        let code = joinInput.value.trim();
        const match = code.match(/\/room\/([a-zA-Z0-9_-]+)/);
        if (match && match[1]) code = match[1];
        if (!code) return;

        try {
          const resp = await fetch(`/api/room/${encodeURIComponent(code)}/info`);
          const info = await resp.json();
          if (info.expired) {
            this.ui.showToast(t('room_expired'));
            return;
          }
          if (info.requiresPassword) {
            this.pendingJoinCode = code;
            const label = document.getElementById('joinPasswordRoomName');
            if (label) label.textContent = `"${info.name || code}" is password-protected.`;
            this.ui.showModal('joinPasswordModal');
            return;
          }
          window.location.href = `/room/${code}`;
        } catch (err) {
          window.location.href = `/room/${code}`;
        }
      });
    }

    if (formJoinPassword) {
      formJoinPassword.addEventListener('submit', (e) => {
        e.preventDefault();
        const pwd = document.getElementById('joinRoomPasswordInput')?.value || '';
        if (!this.pendingJoinCode || !pwd) return;
        sessionStorage.setItem(`twintube_room_pwd_${this.pendingJoinCode}`, pwd);
        window.location.href = `/room/${this.pendingJoinCode}`;
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
