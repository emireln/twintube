/* TwinTube - Authentication Manager */

export class AuthManager {
  constructor() {
    this.token = localStorage.getItem('twintube_token') || null;
    this.user = null;
    try {
      const stored = localStorage.getItem('twintube_user');
      if (stored) this.user = JSON.parse(stored);
    } catch (e) {}

    this.listeners = [];
  }

  isLoggedIn() {
    return !!this.token && !!this.user;
  }

  getToken() {
    return this.token;
  }

  getUser() {
    return this.user;
  }

  async register(username, email, password) {
    const resp = await fetch('/api/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ username, email, password })
    });

    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || 'Registration failed');
    }

    this.setAuthData(data.token, data.user);
    return data;
  }

  async login(usernameOrEmail, password) {
    const resp = await fetch('/api/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ usernameOrEmail, password })
    });

    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || 'Login failed');
    }

    this.setAuthData(data.token, data.user);
    return data;
  }

  async checkAuth() {
    if (!this.token) return null;

    try {
      const resp = await fetch('/api/auth/me', {
        headers: { 'Authorization': `Bearer ${this.token}` }
      });

      if (!resp.ok) {
        this.logout();
        return null;
      }

      const user = await resp.json();
      this.user = user;
      localStorage.setItem('twintube_user', JSON.stringify(user));
      this.notifyListeners();
      return user;
    } catch (e) {
      return null;
    }
  }

  setAuthData(token, user) {
    this.token = token;
    this.user = user;
    localStorage.setItem('twintube_token', token);
    localStorage.setItem('twintube_user', JSON.stringify(user));
    this.notifyListeners();
  }

  logout() {
    this.token = null;
    this.user = null;
    localStorage.removeItem('twintube_token');
    localStorage.removeItem('twintube_user');
    this.notifyListeners();
  }

  onChange(callback) {
    this.listeners.push(callback);
  }

  notifyListeners() {
    this.listeners.forEach(cb => cb(this.isLoggedIn(), this.user));
  }
}
