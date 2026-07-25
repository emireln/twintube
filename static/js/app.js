/* TwinTube - Main Application Orchestrator */

import { UIManager } from './ui.js';
import { WSClient } from './ws.js';
import { VideoPlayer } from './player.js';
import { AuthManager } from './auth.js';
import { t, translateError } from './i18n.js';
import { getLocalBlobUrl, isLocalVideoId, registerLocalFileFromPicker, tryMatchLocalFile } from './localmedia.js';
import { VoiceChat } from './voice.js';

class TwinTubeApp {
  constructor() {
    this.ui = new UIManager();
    this.ws = new WSClient();
    this.auth = new AuthManager();
    this.player = null;
    this.voice = null;

    this.ui.bindSettingsAuth(this.auth);

    this.roomId = this.extractRoomId();
    this.nickname = localStorage.getItem('twintube_nickname') || '';
    this.currentClientID = '';
    this.isHost = false;
    this.isCohost = false;
    this.canControlPlayback = false;
    this.canModerateQueue = false;
    this.queueLocked = false;
    this.skipVotes = 0;
    this.skipNeeded = 1;
    this.hasSkipVoted = false;
    this.approvedMoments = [];
    this.pendingMoments = [];
    this.hasJoined = false;
    this.accessGranted = false;
    this.playlist = [];
    this.forceNextSync = false;
    this.lastUsers = [];
    this.lastVideoStatus = 'PAUSED';
    this.lastVideoTitle = '';
    this.pendingLocalMatchId = '';
    this.currentLocalVideoId = '';
    this.lastLocalStatusKey = '';
    this.mediaAddMode = 'link';

    document.body.classList.add('room-gated');
    this.init();
  }

  isVideoFile(file) {
    return !!(file && (file.type.startsWith('video/') || /\.(mp4|webm|ogg|mkv|mov)$/i.test(file.name || '')));
  }

  setupMediaModeToggle() {
    const btnLink = document.getElementById('btnMediaModeLink');
    const btnLocal = document.getElementById('btnMediaModeLocal');
    const input = document.getElementById('topVideoInput');
    const btnAddLink = document.getElementById('btnAddVideoLink');
    const btnAddLocal = document.getElementById('btnAddLocalVideo');

    const apply = (mode) => {
      this.mediaAddMode = mode;
      btnLink?.classList.toggle('active', mode === 'link');
      btnLocal?.classList.toggle('active', mode === 'local');
      if (input) {
        input.hidden = mode !== 'link';
        input.required = mode === 'link';
      }
      if (btnAddLink) btnAddLink.hidden = mode !== 'link';
      if (btnAddLocal) btnAddLocal.hidden = mode !== 'local';
      document.body.classList.toggle('local-add-mode', mode === 'local');
    };

    btnLink?.addEventListener('click', () => apply('link'));
    btnLocal?.addEventListener('click', () => apply('local'));
    apply('link');
  }

  setupPlayerDragDrop() {
    const wrapper = document.getElementById('playerWrapper');
    if (!wrapper) return;
    wrapper.dataset.dropLabel = t('local_file_drop_hint');

    window.addEventListener('twintube:languagechange', () => {
      wrapper.dataset.dropLabel = t('local_file_drop_hint');
    });

    const onDrag = (e) => {
      e.preventDefault();
      e.stopPropagation();
    };

    wrapper.addEventListener('dragenter', (e) => {
      onDrag(e);
      wrapper.classList.add('drag-over');
    });
    wrapper.addEventListener('dragover', (e) => {
      onDrag(e);
      wrapper.classList.add('drag-over');
    });
    wrapper.addEventListener('dragleave', (e) => {
      onDrag(e);
      if (!wrapper.contains(e.relatedTarget)) wrapper.classList.remove('drag-over');
    });
    wrapper.addEventListener('drop', async (e) => {
      onDrag(e);
      wrapper.classList.remove('drag-over');
      const file = e.dataTransfer?.files?.[0];
      if (!file) return;
      if (!this.isVideoFile(file)) {
        this.ui.showToast(t('local_file_invalid'));
        return;
      }
      if (this.pendingLocalMatchId) {
        this.ui.showToast(t('local_file_hashing'));
        try {
          const result = await tryMatchLocalFile(this.pendingLocalMatchId, file);
          if (!result.ok) {
            this.ui.showToast(t('local_file_mismatch'));
            return;
          }
          const matchedId = this.pendingLocalMatchId;
          this.pendingLocalMatchId = '';
          this.hideLocalFileOverlay();
          this.ui.showToast(t('local_file_matched'));
          this.sendLocalFileStatus(matchedId, true);
          if (this.player) this.player.resumePendingLocalFile();
        } catch (err) {
          console.error('[APP] Drop match failed:', err);
          this.ui.showToast(t('local_file_failed'));
        }
        return;
      }
      await this.addLocalVideoFile(file);
    });
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

    window.addEventListener('twintube:languagechange', () => {
      if (this.lastVideoTitle || this.lastVideoStatus) {
        this.updateVideoMeta(this.lastVideoTitle, this.lastVideoStatus);
      }
      this.renderQueue();
      if (this.lastUsers.length) {
        this.renderViewersList();
      }
    });

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
          this.ui.showToast(err.message || t('failed_new_room'));
        }
      });
    }

    // Manual Resync Button — force full video reload
    const btnResync = document.getElementById('btnResync');
    if (btnResync) {
      btnResync.addEventListener('click', () => {
        this.forceNextSync = true;
        this.ws.sendAction('SYNC_REQUEST');
        this.ui.showToast(t('resyncing'));
      });
    }

    const btnVoteSkip = document.getElementById('btnVoteSkip');
    if (btnVoteSkip) {
      btnVoteSkip.addEventListener('click', () => {
        this.hasSkipVoted = true;
        btnVoteSkip.disabled = true;
        this.ws.sendAction('VOTE_SKIP');
      });
    }
    const btnSkipNow = document.getElementById('btnSkipNow');
    if (btnSkipNow) {
      btnSkipNow.addEventListener('click', () => this.ws.sendAction('SKIP_NOW'));
    }
    const btnQueueLock = document.getElementById('btnQueueLock');
    if (btnQueueLock) {
      btnQueueLock.addEventListener('click', () => {
        this.ws.sendAction('SET_QUEUE_LOCK', { locked: !this.queueLocked });
      });
    }

    const btnSaveMoment = document.getElementById('btnSaveMoment');
    if (btnSaveMoment) {
      btnSaveMoment.addEventListener('click', () => {
        const atSeconds = this.player ? this.player.getCurrentTime() : 0;
        this.ws.sendAction('SUBMIT_MOMENT', { atSeconds });
      });
    }

    this.initVoiceControls();

    // Theater Mode Toggle
    const btnTheaterMode = document.getElementById('btnTheaterMode');
    if (btnTheaterMode) {
      btnTheaterMode.addEventListener('click', () => {
        document.body.classList.toggle('theater-mode');
        const isTheater = document.body.classList.contains('theater-mode');
        btnTheaterMode.classList.toggle('is-active', isTheater);
        this.ui.showToast(isTheater ? t('theater_mode_on') : t('theater_mode_off'));
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
        if (this.mediaAddMode === 'local') return;
        const url = topVideoInput.value.trim();
        if (url) {
          if (this.ws.sendAction('ADD_QUEUE', { url })) {
            topVideoInput.value = '';
            this.ui.showToast(t('adding_video'));
          } else {
            this.ui.showToast(t('not_connected'));
          }
        }
      });
    }

    this.setupMediaModeToggle();
    this.setupPlayerDragDrop();

    // Local file add (bytes stay on device; only a content id is shared for sync)
    const btnAddLocal = document.getElementById('btnAddLocalVideo');
    const localFileInput = document.getElementById('localVideoFileInput');
    if (btnAddLocal && localFileInput) {
      btnAddLocal.addEventListener('click', () => localFileInput.click());
      localFileInput.addEventListener('change', async () => {
        const file = localFileInput.files?.[0];
        localFileInput.value = '';
        if (!file) return;
        if (!this.isVideoFile(file)) {
          this.ui.showToast(t('local_file_invalid'));
          return;
        }
        await this.addLocalVideoFile(file);
      });
    }

    this.setupLocalFileMatchModal();

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
        const meta = this.player && this.player.getPlaybackMeta
          ? this.player.getPlaybackMeta()
          : {};
        this.ws.sendAction('STATE_CHANGE', {
          videoId,
          status,
          currentTime,
          platform: meta.platform || '',
          mediaKind: meta.mediaKind || 'vod',
          sourceUrl: meta.sourceUrl || '',
          seekable: meta.seekable !== false
        });
      },
      () => {
        if (!this.canControlPlayback) return;
        if (this.playlist && this.playlist.length > 0) {
          const nextItem = this.playlist[0];
          this.ws.sendAction('PLAY_QUEUE_ITEM', { itemId: nextItem.id });
        }
      },
      (videoId, title) => this.promptLocalFileMatch(videoId, title)
    );
  }

  async addLocalVideoFile(file) {
    this.ui.showToast(t('local_file_hashing'));
    try {
      const { videoId, entry } = await registerLocalFileFromPicker(file);
      const title = entry?.title || file.name || t('local_video');
      if (!this.ws.sendAction('ADD_QUEUE', { url: videoId, title })) {
        this.ui.showToast(t('not_connected'));
        return;
      }
      this.ui.showToast(t('local_file_added'));
    } catch (err) {
      console.error('[APP] Local file add failed:', err);
      this.ui.showToast(t('local_file_failed'));
    }
  }

  setupLocalFileMatchModal() {
    const input = document.getElementById('localOverlayFileInput');
    const btnPick = document.getElementById('btnPickLocalOverlay');

    if (btnPick && input) {
      btnPick.addEventListener('click', () => input.click());
    }

    if (input) {
      input.addEventListener('change', async () => {
        const file = input.files?.[0];
        input.value = '';
        if (!file || !this.pendingLocalMatchId) return;
        this.ui.showToast(t('local_file_hashing'));
        try {
          const result = await tryMatchLocalFile(this.pendingLocalMatchId, file);
          if (!result.ok) {
            this.ui.showToast(t('local_file_mismatch'));
            return;
          }
          const matchedId = this.pendingLocalMatchId;
          this.pendingLocalMatchId = '';
          this.hideLocalFileOverlay();
          this.ui.showToast(t('local_file_matched'));
          this.sendLocalFileStatus(matchedId, true);
          if (this.player) this.player.resumePendingLocalFile();
        } catch (err) {
          console.error('[APP] Local file match failed:', err);
          this.ui.showToast(t('local_file_failed'));
        }
      });
    }
  }

  promptLocalFileMatch(videoId, title) {
    if (!isLocalVideoId(videoId)) return;
    this.pendingLocalMatchId = videoId;
    this.sendLocalFileStatus(videoId, false);
    const nameEl = document.getElementById('localOverlayFileName');
    if (nameEl) {
      nameEl.textContent = title || '';
      nameEl.hidden = !title;
    }
    const overlay = document.getElementById('localFileOverlay');
    if (overlay) overlay.hidden = false;
  }

  hideLocalFileOverlay() {
    const overlay = document.getElementById('localFileOverlay');
    if (overlay) overlay.hidden = true;
  }

  // Report to the room whether this device has the current local file loaded.
  sendLocalFileStatus(videoId, ready) {
    const key = `${videoId}|${ready}`;
    if (this.lastLocalStatusKey === key) return;
    if (this.ws.sendAction('LOCAL_FILE_STATUS', { videoId, ready })) {
      this.lastLocalStatusKey = key;
    }
  }

  // Called on every INIT_STATE / STATE_UPDATE to keep overlay + readiness in sync.
  syncLocalPresence(videoId) {
    const isLocal = isLocalVideoId(videoId);
    this.currentLocalVideoId = isLocal ? videoId : '';

    if (!isLocal) {
      this.pendingLocalMatchId = '';
      this.hideLocalFileOverlay();
      this.renderViewersList();
      return;
    }

    if (getLocalBlobUrl(videoId)) {
      this.pendingLocalMatchId = '';
      this.hideLocalFileOverlay();
      this.sendLocalFileStatus(videoId, true);
    }
    this.renderViewersList();
    // Otherwise the player's onNeedLocalFile callback shows the overlay.
  }

  renderViewersList() {
    const localActive = !!this.currentLocalVideoId;
    const readyCount = localActive
      ? this.lastUsers.filter(u => u.localFileReady).length
      : 0;
    this.ui.renderViewers(
      this.lastUsers,
      this.currentClientID,
      {
        isHost: this.isHost,
        onTransferHost: (targetId) => this.ws.sendAction('TRANSFER_HOST', { targetId }),
        onGrantCohost: (targetId) => this.ws.sendAction('GRANT_COHOST', { targetId }),
        onRevokeCohost: (targetId) => this.ws.sendAction('REVOKE_COHOST', { targetId }),
        localVideoActive: localActive,
        readyCount,
        totalCount: this.lastUsers.length
      }
    );
  }

  applyRoleState(payload = {}) {
    if (typeof payload.isHost === 'boolean') this.isHost = payload.isHost;
    if (typeof payload.isCohost === 'boolean') this.isCohost = payload.isCohost;
    if (typeof payload.canControlPlayback === 'boolean') {
      this.canControlPlayback = payload.canControlPlayback;
    } else {
      this.canControlPlayback = !!(this.isHost || this.isCohost);
    }
    if (typeof payload.canModerateQueue === 'boolean') {
      this.canModerateQueue = payload.canModerateQueue;
    } else {
      this.canModerateQueue = !!(this.isHost || this.isCohost);
    }
    if (typeof payload.queueLocked === 'boolean') this.queueLocked = payload.queueLocked;
    if (typeof payload.skipVotes === 'number') this.skipVotes = payload.skipVotes;
    if (typeof payload.skipNeeded === 'number') this.skipNeeded = payload.skipNeeded;
    if (typeof payload.hasSkipVoted === 'boolean') this.hasSkipVoted = payload.hasSkipVoted;

    const me = this.lastUsers.find((u) => u.id === this.currentClientID);
    if (me) {
      this.isHost = !!me.isHost;
      this.isCohost = !!me.isCohost;
      this.canControlPlayback = this.isHost || this.isCohost;
      this.canModerateQueue = this.isHost || this.isCohost;
    }

    const hostBadge = document.getElementById('hostBadge');
    if (hostBadge) hostBadge.style.display = this.isHost ? 'inline-flex' : 'none';
    const cohostBadge = document.getElementById('cohostBadge');
    if (cohostBadge) cohostBadge.style.display = (!this.isHost && this.isCohost) ? 'inline-flex' : 'none';

    const btnSkipNow = document.getElementById('btnSkipNow');
    if (btnSkipNow) btnSkipNow.hidden = !this.canControlPlayback;

    const skipTally = document.getElementById('skipTally');
    if (skipTally) skipTally.textContent = `${this.skipVotes}/${this.skipNeeded || 1}`;

    const btnVoteSkip = document.getElementById('btnVoteSkip');
    if (btnVoteSkip) btnVoteSkip.disabled = !!this.hasSkipVoted;

    const btnQueueLock = document.getElementById('btnQueueLock');
    if (btnQueueLock) {
      btnQueueLock.hidden = !this.canModerateQueue;
      btnQueueLock.textContent = this.queueLocked ? t('unlock_queue') : t('lock_queue');
    }
    const queueLockHint = document.getElementById('queueLockHint');
    if (queueLockHint) {
      queueLockHint.hidden = !this.queueLocked;
    }

    this.renderQueue();
    this.renderMomentsList();
  }

  renderMomentsList() {
    this.ui.renderMoments(this.approvedMoments, this.pendingMoments, {
      canControlPlayback: this.canControlPlayback,
      onJump: (momentId) => this.ws.sendAction('JUMP_TO_MOMENT', { momentId }),
      onApprove: (momentId) => this.ws.sendAction('APPROVE_MOMENT', { momentId }),
      onReject: (momentId) => this.ws.sendAction('REJECT_MOMENT', { momentId })
    });
  }

  initVoiceControls() {
    this.voice = new VoiceChat({
      sendAction: (action, payload) => this.ws.sendAction(action, payload),
      getClientId: () => this.currentClientID,
      getPeers: () => this.lastUsers,
      onStatus: (status) => {
        if (status.error === 'mic_denied') {
          this.ui.showToast(t('voice_mic_denied'));
          return;
        }
        if (status.error === 'voice_full') {
          this.ui.showToast(t('voice_full'));
          return;
        }
        const btnToggle = document.getElementById('btnVoiceToggle');
        const btnPtt = document.getElementById('btnPushToTalk');
        const hint = document.getElementById('voiceHint');
        if (btnToggle) {
          btnToggle.textContent = status.joined ? t('voice_leave') : t('voice_join');
        }
        if (btnPtt) btnPtt.hidden = !status.joined;
        if (hint) hint.hidden = !status.joined;
        if (btnPtt) {
          btnPtt.classList.toggle('is-talking', !!status.speaking);
        }
      }
    });

    const btnToggle = document.getElementById('btnVoiceToggle');
    if (btnToggle) {
      btnToggle.addEventListener('click', async () => {
        if (this.voice.joined) {
          await this.voice.leave();
        } else {
          await this.voice.join();
        }
      });
    }
    const btnPtt = document.getElementById('btnPushToTalk');
    if (btnPtt) {
      const down = (e) => { e.preventDefault(); this.voice.setPTT(true); };
      const up = (e) => { e.preventDefault(); this.voice.setPTT(false); };
      btnPtt.addEventListener('mousedown', down);
      btnPtt.addEventListener('mouseup', up);
      btnPtt.addEventListener('mouseleave', up);
      btnPtt.addEventListener('touchstart', down, { passive: false });
      btnPtt.addEventListener('touchend', up);
    }
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
      this.playlist = payload.playlist || [];
      this.lastUsers = payload.users || [];
      this.approvedMoments = payload.moments || [];
      this.pendingMoments = payload.pendingMoments || [];
      this.applyRoleState(payload);

      if (payload.video) {
        this.updateVideoMeta(payload.video.title, payload.video.status);
        this.syncLocalPresence(payload.video.videoId || '');
        if (this.player) {
          this.player.applyServerState(
            payload.video,
            payload.video.serverTimestamp || timestamp,
            { force: true, timeAlreadyAbsolute: true }
          );
        }
      }

      this.renderQueue();
      this.renderViewersList();
      this.renderMomentsList();
    });

    this.ws.on('STATE_UPDATE', (payload, timestamp) => {
      this.updateVideoMeta(payload.title, payload.status);
      this.syncLocalPresence(payload.videoId || '');
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
      const prev = this.lastUsers || [];
      this.lastUsers = users || [];
      this.applyRoleState({});
      this.renderViewersList();

      if (this.voice && this.voice.joined) {
        const prevIds = new Set(prev.filter((u) => u.voiceJoined).map((u) => u.id));
        const nextIds = new Set(this.lastUsers.filter((u) => u.voiceJoined).map((u) => u.id));
        for (const id of nextIds) {
          if (!prevIds.has(id)) this.voice.handlePeerJoined(id);
        }
        for (const id of prevIds) {
          if (!nextIds.has(id)) this.voice.handlePeerLeft(id);
        }
      }
    });

    this.ws.on('RTC_OFFER', (payload) => {
      if (this.voice && payload?.fromId && payload?.sdp) {
        this.voice.handleOffer(payload.fromId, payload.sdp);
      }
    });
    this.ws.on('RTC_ANSWER', (payload) => {
      if (this.voice && payload?.fromId && payload?.sdp) {
        this.voice.handleAnswer(payload.fromId, payload.sdp);
      }
    });
    this.ws.on('RTC_ICE', (payload) => {
      if (this.voice && payload?.fromId && payload?.candidate) {
        this.voice.handleIce(payload.fromId, payload.candidate);
      }
    });

    this.ws.on('ROOM_META', (payload) => {
      const prevVotes = this.skipVotes;
      this.applyRoleState(payload || {});
      if ((payload?.skipVotes || 0) === 0) {
        this.hasSkipVoted = false;
        const btn = document.getElementById('btnVoteSkip');
        if (btn) btn.disabled = false;
      } else if (payload?.skipVotes > prevVotes && this.hasSkipVoted) {
        const btn = document.getElementById('btnVoteSkip');
        if (btn) btn.disabled = true;
      }
    });

    this.ws.on('MOMENT_SUBMITTED', () => {
      this.ui.showToast(t('moment_submitted'));
    });

    this.ws.on('MOMENT_PENDING', (moment) => {
      if (!moment || !moment.id) return;
      if (!this.pendingMoments.find((m) => m.id === moment.id)) {
        this.pendingMoments = [...this.pendingMoments, moment];
      }
      this.renderMomentsList();
    });

    this.ws.on('MOMENT_APPROVED', (moment) => {
      if (!moment || !moment.id) return;
      this.pendingMoments = this.pendingMoments.filter((m) => m.id !== moment.id);
      if (!this.approvedMoments.find((m) => m.id === moment.id)) {
        this.approvedMoments = [...this.approvedMoments, moment];
      }
      this.renderMomentsList();
    });

    this.ws.on('MOMENT_REJECTED', (payload) => {
      const id = payload?.momentId;
      if (!id) return;
      this.pendingMoments = this.pendingMoments.filter((m) => m.id !== id);
      this.renderMomentsList();
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
            this.redirectHome('too_many_attempts');
            return;
          }

          this.handleWrongRoomAccess(payload.message);
          return;
        }

        if (msg.includes('not found')) {
          this.redirectHome('room_not_found');
          return;
        }

        if (msg.includes('expired')) {
          sessionStorage.removeItem(this.joinTokenKey(this.roomId));
          this.redirectHome('room_expired');
          return;
        }

        this.ui.showToast(translateError(payload.message));
      }
    });

    this.ws.on('fatal', () => {
      this.redirectHome('could_not_verify_access');
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
        return {
          error: data?.error || 'room_not_found',
          exists: data?.exists === true,
          expired: !!data?.expired || data?.error === 'room_expired',
          name: data?.name || '',
          requiresPassword: !!data?.requiresPassword,
          isOwner: !!data?.isOwner
        };
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
      throw new Error(data.error || t('access_denied'));
    }
    if (data.joinToken) {
      sessionStorage.setItem(this.joinTokenKey(roomId), data.joinToken);
    }
    return data;
  }

  // Leave the room page immediately and show a toast on the home page.
  redirectHome(toastKey) {
    this.accessGranted = false;
    this.hasJoined = false;
    if (this.ws) this.ws.disconnect(true);
    try {
      sessionStorage.setItem('twintube_flash_toast', toastKey || 'room_not_found');
    } catch (_) { /* ignore */ }
    window.location.replace('/');
  }

  async resolveRoomAccess() {
    const info = await this.fetchRoomInfo();
    if (!info) {
      this.redirectHome('could_not_verify_access');
      return false;
    }

    if (info.error && !info.exists) {
      this.redirectHome(info.error === 'invalid_room_code' ? 'invalid_room_code' : 'room_not_found');
      return false;
    }

    if (info.expired) {
      this.redirectHome('room_expired');
      return false;
    }

    if (!info.exists) {
      this.redirectHome('room_not_found');
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
        window.location.replace('/');
        return false;
      }
      try {
        await this.requestRoomAccess(this.roomId, pwd);
      } catch (err) {
        this.ui.showToast(translateError(err.message, 'password_required'));
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
        label.textContent = t('room_password_protected', { name: roomName });
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
    // Fresh server-side client on (re)join — readiness must be re-announced.
    this.lastLocalStatusKey = '';

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
    this.ui.renderQueue(this.playlist, {
      canControlPlayback: this.canControlPlayback,
      canModerateQueue: this.canModerateQueue,
      onPlayItem: (itemId) => this.ws.sendAction('PLAY_QUEUE_ITEM', { itemId }),
      onRemoveItem: (itemId) => this.ws.sendAction('REMOVE_QUEUE_ITEM', { itemId }),
      onReorder: (order) => this.ws.sendAction('REORDER_QUEUE', { order })
    });
  }

  updateVideoMeta(title, status) {
    const videoTitle = document.getElementById('videoTitle');
    const statusBadge = document.getElementById('statusBadge');
    const statusText = document.getElementById('statusText');

    if (title) this.lastVideoTitle = title;
    if (status) this.lastVideoStatus = status;

    if (videoTitle && title) {
      videoTitle.textContent = title;
    } else if (videoTitle && !title && !this.lastVideoTitle) {
      videoTitle.textContent = t('sync_room_title');
    }

    if (statusBadge && statusText) {
      const playing = status === 'PLAYING';
      statusBadge.className = playing ? 'badge badge-playing' : 'badge badge-paused';
      statusText.textContent = playing ? t('status_playing') : t('status_paused');
    }
  }
}

document.addEventListener('DOMContentLoaded', () => {
  window.app = new TwinTubeApp();
});
