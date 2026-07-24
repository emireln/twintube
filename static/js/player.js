/* TwinTube - YouTube iFrame Player Controller & Drift Sync Engine */

export class VideoPlayer {
  constructor(onLocalStateChange, onVideoEnded) {
    this.player = null;
    this.currentVideoId = '';
    this.currentStatus = 'PAUSED';
    this.onLocalStateChange = onLocalStateChange;
    this.onVideoEnded = onVideoEnded;
    this.isRemoteUpdate = false;
    this.isReady = false;
    this.syncThreshold = 1.5; // Drift tolerance in seconds

    this.initYouTubeAPI();
  }

  initYouTubeAPI() {
    if (window.YT && window.YT.Player) {
      this.createPlayer();
    } else {
      window.onYouTubeIframeAPIReady = () => this.createPlayer();
    }
  }

  createPlayer() {
    this.player = new window.YT.Player('ytPlayer', {
      height: '100%',
      width: '100%',
      videoId: 'dQw4w9WgXcQ',
      playerVars: {
        autoplay: 0,
        controls: 1,
        rel: 0,
        modestbranding: 1,
        enablejsapi: 1,
        origin: window.location.origin
      },
      events: {
        onReady: (event) => this.onPlayerReady(event),
        onStateChange: (event) => this.onPlayerStateChange(event)
      }
    });
  }

  onPlayerReady(event) {
    this.isReady = true;
    console.log('[PLAYER] YouTube Player is ready.');
  }

  onPlayerStateChange(event) {
    if (!this.isReady || this.isRemoteUpdate) return;

    const state = event.data;
    const currentTime = this.player.getCurrentTime();

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

  // Authoritative Server State Sync Engine
  applyServerState(state, serverTimestamp) {
    if (!this.isReady || !this.player) return;

    this.isRemoteUpdate = true;

    const now = Date.now();
    let expectedTime = state.currentTime;

    if (state.status === 'PLAYING' && serverTimestamp) {
      const elapsedSec = (now - serverTimestamp) / 1000.0;
      expectedTime += elapsedSec;
    }

    const playerTime = this.player.getCurrentTime();
    const drift = Math.abs(playerTime - expectedTime);

    // 1. If Video ID changed
    if (this.currentVideoId !== state.videoId) {
      this.currentVideoId = state.videoId;
      this.currentStatus = state.status;
      
      if (state.status === 'PLAYING') {
        this.player.loadVideoById({ videoId: state.videoId, startSeconds: expectedTime });
      } else {
        this.player.cueVideoById({ videoId: state.videoId, startSeconds: expectedTime });
      }
    } else {
      // 2. Check Drift (> 1.5s tolerance)
      if (drift > this.syncThreshold) {
        console.log(`[PLAYER] Drift detected (${drift.toFixed(2)}s). Resyncing to ${expectedTime.toFixed(2)}s...`);
        this.player.seekTo(expectedTime, true);
      }

      // 3. Status adjustment
      if (state.status === 'PLAYING') {
        const playerState = this.player.getPlayerState();
        if (playerState !== window.YT.PlayerState.PLAYING && playerState !== window.YT.PlayerState.BUFFERING) {
          this.player.playVideo();
        }
      } else if (state.status === 'PAUSED') {
        const playerState = this.player.getPlayerState();
        if (playerState === window.YT.PlayerState.PLAYING) {
          this.player.pauseVideo();
        }
      }
    }

    // Reset remote update guard after small delay to avoid event loopback
    setTimeout(() => {
      this.isRemoteUpdate = false;
    }, 500);
  }

  getCurrentTime() {
    return this.player && this.player.getCurrentTime ? this.player.getCurrentTime() : 0.0;
  }
}
