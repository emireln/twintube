/* TwinTube - Landing Page Logic */

import { UIManager } from './ui.js';
import { AuthManager } from './auth.js';

class LandingApp {
  constructor() {
    this.ui = new UIManager();
    this.auth = new AuthManager();

    this.init();
  }

  async init() {
    this.setupCreateRoom();
    this.setupAuthUI();
    await this.auth.checkAuth();
    this.updateAuthNavUI();
  }

  setupCreateRoom() {
    const btnCreate = document.getElementById('btnLandingCreateRoom');
    if (btnCreate) {
      btnCreate.addEventListener('click', async () => {
        btnCreate.disabled = true;
        btnCreate.textContent = 'Creating Room...';

        try {
          const resp = await fetch('/api/room/create');
          const data = await resp.json();
          if (data.url) {
            window.location.href = data.url;
          } else {
            throw new Error('Invalid room response');
          }
        } catch (err) {
          this.ui.showToast('Failed to create room. Please try again.');
          btnCreate.disabled = false;
          btnCreate.textContent = 'Create Room';
        }
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
          this.ui.showToast('Successfully signed in!');
        } catch (err) {
          this.ui.showToast(err.message || 'Login failed');
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
          this.ui.showToast('Account created successfully!');
        } catch (err) {
          this.ui.showToast(err.message || 'Registration failed');
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
        this.ui.showToast('Signed out.');
        window.location.reload();
      });
    }

    this.auth.onChange(() => this.updateAuthNavUI());
  }

  updateAuthNavUI() {
    const btnOpenAuthLogin = document.getElementById('btnOpenAuthLogin');
    const btnOpenAuthRegister = document.getElementById('btnOpenAuthRegister');
    const userProfileMenu = document.getElementById('userProfileMenu');
    const userAvatarText = document.getElementById('userAvatarText');
    const dropdownUsername = document.getElementById('dropdownUsername');
    const dropdownEmail = document.getElementById('dropdownEmail');

    if (this.auth.isLoggedIn()) {
      const user = this.auth.getUser();
      if (btnOpenAuthLogin) btnOpenAuthLogin.style.display = 'none';
      if (btnOpenAuthRegister) btnOpenAuthRegister.style.display = 'none';
      if (userProfileMenu) userProfileMenu.style.display = 'block';
      if (userAvatarText) userAvatarText.textContent = (user.username || 'U').charAt(0).toUpperCase();
      if (dropdownUsername) dropdownUsername.textContent = user.username;
      if (dropdownEmail) dropdownEmail.textContent = user.email;
    } else {
      if (btnOpenAuthLogin) btnOpenAuthLogin.style.display = 'inline-flex';
      if (btnOpenAuthRegister) btnOpenAuthRegister.style.display = 'inline-flex';
      if (userProfileMenu) userProfileMenu.style.display = 'none';
    }
  }
}

document.addEventListener('DOMContentLoaded', () => {
  window.landingApp = new LandingApp();
});
