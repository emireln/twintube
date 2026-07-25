/* TwinTube - Universal Video Player Controller & Sync Engine */

export class VideoPlayer {
  constructor(onLocalStateChange, onVideoEnded) {
    this.ytPlayer = null;
    this.html5Player = document.getElementById('html5Player');
    this.genericPlayer = document.getElementById('genericPlayer');
    this.ytContainer = document.getElementById('ytPlayer');

    this.activePlatform = 'youtube'; // 'youtube', 'direct', 'vimeo', 'twitch', 'generic'
    this.currentVideoId = 'dQw4w9WgXcQ';
    this.currentStatus = 'PAUSED';
    this.onLocalStateChange = onLocalStateChange;
    this.onVideoEnded = onVideoEnded;
    this.isRemoteUpdate = false;
    this.isReady = false;
    this.syncThreshold = 1.5; // Drift tolerance in seconds
    this.pendingServerState = null;
    this.remoteUpdateTimer = null;

    this.initYouTubeAPI();
    this.initHTML5Player();
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
      if (this.activePlatform !== 'direct' || this.isRemoteUpdate) return;
      this.currentStatus = 'PLAYING';
      this.onLocalStateChange('PLAYING', this.html5Player.currentTime, this.currentVideoId);
    });

    this.html5Player.addEventListener('pause', () => {
      if (this.activePlatform !== 'direct' || this.isRemoteUpdate) return;
      this.currentStatus = 'PAUSED';
      this.onLocalStateChange('PAUSED', this.html5Player.currentTime, this.currentVideoId);
    });

    this.html5Player.addEventListener('ended', () => {
      if (this.activePlatform !== 'direct') return;
      this.currentStatus = 'PAUSED';
      this.onVideoEnded();
    });
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

    const ytEl = document.getElementById('ytPlayer');
    if (ytEl) ytEl.style.display = platform === 'youtube' ? 'block' : 'none';
    if (this.html5Player) this.html5Player.style.display = platform === 'direct' ? 'block' : 'none';
    if (this.genericPlayer) this.genericPlayer.style.display = (platform !== 'youtube' && platform !== 'direct') ? 'block' : 'none';
  }

  detectPlatform(videoId) {
    if (videoId.startsWith('http://') || videoId.startsWith('https://') || videoId.endsWith('.mp4') || videoId.endsWith('.webm')) {
      if (videoId.includes('vimeo.com')) return 'vimeo';
      if (videoId.includes('twitch.tv')) return 'twitch';
      return 'direct';
    }
    return 'youtube';
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

    const platform = this.detectPlatform(videoId);

    if (platform === 'youtube' && (!this.ytPlayer || !this.isReady)) {
      this.pendingServerState = {
        videoState: { ...videoState, videoId, status, currentTime: expectedTime, serverTimestamp: stamp },
        serverTimestamp: stamp,
        options: { force: true, timeAlreadyAbsolute: true }
      };
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
      if (needsLoad) {
        this.ytPlayer.loadVideoById({
          videoId,
          startSeconds: Math.max(0, expectedTime)
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

        if (drift > this.syncThreshold) {
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
    } else if (platform === 'direct' && this.html5Player) {
      if (this.html5Player.src !== videoId) {
        this.html5Player.src = videoId;
      }

      const drift = Math.abs(this.html5Player.currentTime - expectedTime);
      if (drift > this.syncThreshold) {
        this.html5Player.currentTime = expectedTime;
      }

      if (status === 'PLAYING') {
        this.html5Player.play().catch(() => {});
      } else {
        this.html5Player.pause();
      }
    } else if (this.genericPlayer) {
      let embedUrl = videoId;
      if (platform === 'vimeo') {
        const match = videoId.match(/vimeo\.com\/(?:video\/)?([0-9]+)/);
        const vimeoId = match ? match[1] : videoId;
        embedUrl = `https://player.vimeo.com/video/${vimeoId}?autoplay=1`;
      } else if (platform === 'twitch') {
        embedUrl = `https://player.twitch.tv/?video=${videoId}&parent=${window.location.hostname}`;
      }

      if (this.genericPlayer.src !== embedUrl) {
        this.genericPlayer.src = embedUrl;
      }
    }
  }

  getCurrentTime() {
    if (this.activePlatform === 'youtube' && this.ytPlayer && this.ytPlayer.getCurrentTime) {
      return this.ytPlayer.getCurrentTime();
    }
    if (this.activePlatform === 'direct' && this.html5Player) {
      return this.html5Player.currentTime || 0.0;
    }
    return 0.0;
  }
}
