/* TwinTube - Universal Video Player Controller & Sync Engine */

import { getLocalBlobUrl, isLocalVideoId } from './localmedia.js';

export class VideoPlayer {
  constructor(onLocalStateChange, onVideoEnded, onNeedLocalFile = null) {
    this.ytPlayer = null;
    this.html5Player = document.getElementById('html5Player');
    this.genericPlayer = document.getElementById('genericPlayer');
    this.ytContainer = document.getElementById('ytPlayer');

    this.activePlatform = 'youtube';
    this.mediaKind = 'vod';
    this.sourceUrl = '';
    this.seekable = true;
    this.currentVideoId = 'dQw4w9WgXcQ';
    this.currentStatus = 'PAUSED';
    this.onLocalStateChange = onLocalStateChange;
    this.onVideoEnded = onVideoEnded;
    this.onNeedLocalFile = onNeedLocalFile;
    this.isRemoteUpdate = false;
    this.isReady = false;
    this.syncThreshold = 1.5; // Drift tolerance in seconds
    this.pendingServerState = null;
    this.remoteUpdateTimer = null;
    this.pendingLocalState = null;
    this.hls = null;

    this.initYouTubeAPI();
    this.initHTML5Player();
  }

  usesHtml5Surface(platform = this.activePlatform) {
    return platform === 'direct' || platform === 'local' || platform === 'hls';
  }

  getPlaybackMeta() {
    return {
      platform: this.activePlatform,
      mediaKind: this.mediaKind || 'vod',
      sourceUrl: this.sourceUrl || '',
      seekable: this.seekable !== false
    };
  }

  initYouTubeAPI() {
    if (window.YT && window.YT.Player) {
      this.createYTPlayer();
    } else {
      window.onYouTubeIframeAPIReady = () => this.createYTPlayer();
    }
  }

  createYTPlayer() {
    try {
      this.ytPlayer = new window.YT.Player('ytPlayer', {
        height: '100%',
        width: '100%',
        videoId: this.currentVideoId,
        playerVars: {
          autoplay: 0,
          controls: 1,
          rel: 0,
          modestbranding: 1,
          enablejsapi: 1,
          origin: window.location.origin
        },
        events: {
          onReady: () => {
            this.isReady = true;
            console.log('[PLAYER] YouTube iFrame API ready.');
            if (this.pendingServerState) {
              const pending = this.pendingServerState;
              this.pendingServerState = null;
              this.applyServerState(
                pending.videoState,
                pending.serverTimestamp,
                pending.options || { force: true, timeAlreadyAbsolute: true }
              );
            }
          },
          onStateChange: (event) => this.onYTStateChange(event)
        }
      });
    } catch (err) {
      console.warn('[PLAYER] Failed to instantiate YouTube player:', err);
    }
  }

  initHTML5Player() {
    if (!this.html5Player) return;

    this.html5Player.addEventListener('play', () => {
      if (!this.usesHtml5Surface() || this.isRemoteUpdate) return;
      this.currentStatus = 'PLAYING';
      this.onLocalStateChange('PLAYING', this.html5Player.currentTime, this.currentVideoId);
    });

    this.html5Player.addEventListener('pause', () => {
      if (!this.usesHtml5Surface() || this.isRemoteUpdate) return;
      this.currentStatus = 'PAUSED';
      this.onLocalStateChange('PAUSED', this.html5Player.currentTime, this.currentVideoId);
    });

    this.html5Player.addEventListener('ended', () => {
      if (!this.usesHtml5Surface() || this.mediaKind === 'live') return;
      this.currentStatus = 'PAUSED';
      this.onVideoEnded();
    });
  }

  destroyHls() {
    if (this.hls) {
      try { this.hls.destroy(); } catch (_) { /* ignore */ }
      this.hls = null;
    }
  }

  markRemoteUpdate(durationMs = 1200) {
    this.isRemoteUpdate = true;
    if (this.remoteUpdateTimer) {
      clearTimeout(this.remoteUpdateTimer);
    }
    this.remoteUpdateTimer = setTimeout(() => {
      this.isRemoteUpdate = false;
      this.remoteUpdateTimer = null;
    }, durationMs);
  }

  onYTStateChange(event) {
    if (!this.isReady || this.activePlatform !== 'youtube' || this.isRemoteUpdate) return;

    const state = event.data;
    const currentTime = this.ytPlayer.getCurrentTime ? this.ytPlayer.getCurrentTime() : 0;

    // Keep currentVideoId in sync with the actual loaded video
    try {
      const data = this.ytPlayer.getVideoData && this.ytPlayer.getVideoData();
      if (data && data.video_id) {
        this.currentVideoId = data.video_id;
      }
    } catch (_) { /* ignore */ }

    if (state === window.YT.PlayerState.PLAYING) {
      this.currentStatus = 'PLAYING';
      this.onLocalStateChange('PLAYING', currentTime, this.currentVideoId);
    } else if (state === window.YT.PlayerState.PAUSED) {
      this.currentStatus = 'PAUSED';
      this.onLocalStateChange('PAUSED', currentTime, this.currentVideoId);
    } else if (state === window.YT.PlayerState.ENDED) {
      this.currentStatus = 'PAUSED';
      this.onVideoEnded();
    }
  }

  switchPlatform(platform) {
    this.activePlatform = platform;
    const html5 = this.usesHtml5Surface(platform);

    const ytEl = document.getElementById('ytPlayer');
    if (ytEl) ytEl.style.display = platform === 'youtube' ? 'block' : 'none';
    if (this.html5Player) this.html5Player.style.display = html5 ? 'block' : 'none';
    if (this.genericPlayer) {
      this.genericPlayer.style.display = (!html5 && platform !== 'youtube') ? 'block' : 'none';
    }
    if (!html5) this.destroyHls();
  }

  detectPlatform(videoId) {
    if (isLocalVideoId(videoId)) return 'local';
    if (typeof videoId === 'string' && videoId.includes('.m3u8')) return 'hls';
    if (typeof videoId === 'string' && (videoId.startsWith('vod:') || videoId.startsWith('channel:'))) return 'twitch';
    if (typeof videoId === 'string' && videoId.includes('|')) return 'peertube';
    if (typeof videoId === 'string' && (videoId.startsWith('http://') || videoId.startsWith('https://'))) {
      if (videoId.includes('vimeo.com')) return 'vimeo';
      if (videoId.includes('twitch.tv')) return 'twitch';
      if (videoId.includes('dailymotion.com') || videoId.includes('dai.ly')) return 'dailymotion';
      if (videoId.includes('streamable.com')) return 'streamable';
      if (/\.(mp4|webm|ogg)(\?|$)/i.test(videoId)) return 'direct';
      return 'direct';
    }
    return 'youtube';
  }

  resolvePlatform(videoState, videoId) {
    const fromServer = (videoState && videoState.platform) || '';
    if (fromServer) return fromServer;
    return this.detectPlatform(videoId);
  }

  resolveDirectSrc(videoId, sourceUrl = '') {
    if (isLocalVideoId(videoId)) {
      return getLocalBlobUrl(videoId);
    }
    if (sourceUrl && (sourceUrl.startsWith('http://') || sourceUrl.startsWith('https://') || sourceUrl.startsWith('blob:'))) {
      return sourceUrl;
    }
    return videoId;
  }

  buildEmbedUrl(platform, videoId, sourceUrl = '') {
    const parent = encodeURIComponent(window.location.hostname || 'localhost');
    if (sourceUrl && sourceUrl.includes('HOSTNAME')) {
      return sourceUrl.replace(/HOSTNAME/g, window.location.hostname || 'localhost');
    }
    if (platform === 'vimeo') {
      const match = String(videoId).match(/(?:vimeo\.com\/(?:video\/)?)?([0-9]+)/);
      const vimeoId = match ? match[1] : videoId;
      return `https://player.vimeo.com/video/${vimeoId}?autoplay=1`;
    }
    if (platform === 'twitch') {
      if (String(videoId).startsWith('channel:')) {
        return `https://player.twitch.tv/?channel=${encodeURIComponent(videoId.slice(8))}&parent=${parent}`;
      }
      const vodId = String(videoId).startsWith('vod:') ? videoId.slice(4) : videoId;
      return `https://player.twitch.tv/?video=${encodeURIComponent(vodId)}&parent=${parent}`;
    }
    if (platform === 'dailymotion') {
      return `https://www.dailymotion.com/embed/video/${encodeURIComponent(videoId)}?autoplay=1`;
    }
    if (platform === 'streamable') {
      return `https://streamable.com/e/${encodeURIComponent(videoId)}`;
    }
    if (platform === 'peertube') {
      const parts = String(videoId).split('|');
      if (parts.length === 2) {
        return `https://${parts[0]}/videos/embed/${parts[1]}`;
      }
    }
    return sourceUrl || videoId;
  }

  applyHtml5Playback(src, status, expectedTime, needsLoad, seekable) {
    if (!this.html5Player || !src) return;

    const apply = () => {
      try {
        if (seekable !== false && this.mediaKind !== 'live') {
          const drift = Math.abs((this.html5Player.currentTime || 0) - expectedTime);
          if (drift > this.syncThreshold || needsLoad) {
            this.html5Player.currentTime = Math.max(0, expectedTime);
          }
        } else if (this.mediaKind === 'live' && typeof this.html5Player.seekable?.end === 'function') {
          try {
            const end = this.html5Player.seekable.end(0);
            if (Number.isFinite(end) && end > 0) {
              this.html5Player.currentTime = Math.max(0, end - 1);
            }
          } catch (_) { /* ignore */ }
        }
      } catch (_) { /* ignore seek before ready */ }

      if (status === 'PLAYING') {
        this.html5Player.play().catch(() => {});
      } else {
        this.html5Player.pause();
      }
    };

    if (this.activePlatform === 'hls' && window.Hls && window.Hls.isSupported()) {
      if (needsLoad || !this.hls || this._hlsSrc !== src) {
        this.destroyHls();
        this.hls = new window.Hls();
        this._hlsSrc = src;
        this.hls.loadSource(src);
        this.hls.attachMedia(this.html5Player);
        this.hls.on(window.Hls.Events.MANIFEST_PARSED, apply);
      } else {
        apply();
      }
      return;
    }

    if (this.html5Player.src !== src) {
      this.destroyHls();
      this.html5Player.src = src;
      this.html5Player.addEventListener('loadedmetadata', apply, { once: true });
    } else {
      apply();
    }
  }

  /** Called after a peer selects the matching local file. */
  resumePendingLocalFile() {
    if (!this.pendingLocalState) return;
    const pending = this.pendingLocalState;
    this.pendingLocalState = null;
    const vs = { ...(pending.videoState || {}) };
    const stamp = pending.serverTimestamp || vs.serverTimestamp || Date.now();
    let expectedTime = typeof vs.currentTime === 'number' ? vs.currentTime : 0;
    // Always recompute absolute time so late matchers catch up to live playhead.
    if ((vs.status || 'PAUSED') === 'PLAYING' && stamp) {
      const elapsedSec = (Date.now() - stamp) / 1000.0;
      if (elapsedSec > 0 && elapsedSec < 3600) expectedTime += elapsedSec;
    }
    vs.currentTime = expectedTime;
    vs.serverTimestamp = stamp;
    this.applyServerState(vs, stamp, { force: true, timeAlreadyAbsolute: true });
  }

  getLoadedVideoId() {
    if (!this.ytPlayer || !this.isReady) return '';
    try {
      const data = this.ytPlayer.getVideoData && this.ytPlayer.getVideoData();
      if (data && data.video_id) return data.video_id;
    } catch (_) { /* ignore */ }
    return '';
  }

  applyServerState(videoState, serverTimestamp, options = {}) {
    if (!videoState) return;

    const force = !!options.force;
    const timeAlreadyAbsolute = !!options.timeAlreadyAbsolute;
    const videoId = videoState.videoId || 'dQw4w9WgXcQ';
    const status = videoState.status || 'PAUSED';
    let expectedTime = typeof videoState.currentTime === 'number' ? videoState.currentTime : 0.0;

    const stamp = videoState.serverTimestamp || serverTimestamp;
    if (!timeAlreadyAbsolute && status === 'PLAYING' && stamp) {
      const elapsedSec = (Date.now() - stamp) / 1000.0;
      if (elapsedSec > 0 && elapsedSec < 3600) {
        expectedTime += elapsedSec;
      }
    }

    const platform = this.resolvePlatform(videoState, videoId);
    this.mediaKind = videoState.mediaKind || (platform === 'twitch' && String(videoId).startsWith('channel:') ? 'live' : 'vod');
    this.sourceUrl = videoState.sourceUrl || '';
    this.seekable = videoState.seekable !== false && this.mediaKind !== 'live';

    if (platform === 'youtube' && (!this.ytPlayer || !this.isReady)) {
      this.pendingServerState = {
        videoState: { ...videoState, videoId, status, currentTime: expectedTime, serverTimestamp: stamp, platform, mediaKind: this.mediaKind, sourceUrl: this.sourceUrl, seekable: this.seekable },
        serverTimestamp: stamp,
        options: { force: true, timeAlreadyAbsolute: true }
      };
      return;
    }

    if (isLocalVideoId(videoId) && !getLocalBlobUrl(videoId)) {
      this.pendingLocalState = {
        videoState: { ...videoState, videoId, status, currentTime: expectedTime, serverTimestamp: stamp, platform: 'local', mediaKind: this.mediaKind, seekable: this.seekable },
        serverTimestamp: stamp,
        options: { force: true, timeAlreadyAbsolute: true }
      };
      this.switchPlatform('local');
      this.currentVideoId = videoId;
      this.currentStatus = status;
      if (typeof this.onNeedLocalFile === 'function') {
        this.onNeedLocalFile(videoId, videoState.title || '');
      }
      return;
    }

    if (this.activePlatform !== platform) {
      this.switchPlatform(platform);
    }

    this.currentStatus = status;
    const loadedId = platform === 'youtube' ? this.getLoadedVideoId() : this.currentVideoId;
    const needsLoad = force || !loadedId || loadedId !== videoId;

    this.markRemoteUpdate(needsLoad ? 2500 : 1200);
    this.currentVideoId = videoId;

    if (platform === 'youtube' && this.ytPlayer && this.isReady) {
      const allowSeek = this.seekable;
      if (needsLoad) {
        this.ytPlayer.loadVideoById({
          videoId,
          startSeconds: allowSeek ? Math.max(0, expectedTime) : 0
        });
        if (status !== 'PLAYING') {
          setTimeout(() => {
            this.markRemoteUpdate(800);
            if (this.ytPlayer && this.ytPlayer.pauseVideo) {
              this.ytPlayer.pauseVideo();
            }
          }, 400);
        }
      } else {
        const playerTime = this.ytPlayer.getCurrentTime ? this.ytPlayer.getCurrentTime() : 0;
        const drift = Math.abs(playerTime - expectedTime);

        if (allowSeek && drift > this.syncThreshold) {
          this.ytPlayer.seekTo(expectedTime, true);
        }

        if (status === 'PLAYING') {
          const state = this.ytPlayer.getPlayerState ? this.ytPlayer.getPlayerState() : -1;
          if (state !== window.YT.PlayerState.PLAYING) {
            this.ytPlayer.playVideo();
          }
        } else {
          const state = this.ytPlayer.getPlayerState ? this.ytPlayer.getPlayerState() : -1;
          if (state === window.YT.PlayerState.PLAYING) {
            this.ytPlayer.pauseVideo();
          }
        }
      }
    } else if (this.usesHtml5Surface(platform) && this.html5Player) {
      const src = this.resolveDirectSrc(videoId, this.sourceUrl);
      if (!src) return;
      this.applyHtml5Playback(src, status, expectedTime, needsLoad, this.seekable);
    } else if (this.genericPlayer) {
      const embedUrl = this.buildEmbedUrl(platform, videoId, this.sourceUrl);
      if (embedUrl && this.genericPlayer.src !== embedUrl) {
        this.genericPlayer.src = embedUrl;
      }
    }

    this.updateLiveBadge();
  }

  updateLiveBadge() {
    const badge = document.getElementById('liveBadge');
    if (!badge) return;
    const live = this.mediaKind === 'live';
    badge.style.display = live ? 'inline-flex' : 'none';
  }

  getCurrentTime() {
    if (this.activePlatform === 'youtube' && this.ytPlayer && this.ytPlayer.getCurrentTime) {
      return this.ytPlayer.getCurrentTime();
    }
    if (this.usesHtml5Surface() && this.html5Player) {
      return this.html5Player.currentTime || 0.0;
    }
    return 0.0;
  }
}
