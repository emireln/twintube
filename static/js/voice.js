/* TwinTube - Push-to-talk WebRTC mesh voice */

const MAX_VOICE_PEERS = 6;

export class VoiceChat {
  constructor({ sendAction, onStatus, getClientId, getPeers }) {
    this.sendAction = sendAction;
    this.onStatus = onStatus || (() => {});
    this.getClientId = getClientId;
    this.getPeers = getPeers; // () => [{id, nickname}]
    this.enabled = false;
    this.joined = false;
    this.muted = true;
    this.speaking = false;
    this.pttHeld = false;
    this.localStream = null;
    this.peers = new Map(); // clientId -> { pc, audio, pendingIce }
    this.iceServers = [{ urls: ['stun:stun.l.google.com:19302'] }];
    this.boundKeyDown = (e) => this.onKeyDown(e);
    this.boundKeyUp = (e) => this.onKeyUp(e);
  }

  async loadConfig() {
    try {
      const res = await fetch('/api/rtc/config');
      if (!res.ok) return;
      const data = await res.json();
      if (Array.isArray(data.iceServers) && data.iceServers.length) {
        this.iceServers = data.iceServers;
      }
    } catch (_) { /* STUN fallback already set */ }
  }

  async join() {
    if (this.joined) return true;
    await this.loadConfig();
    try {
      this.localStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true
        },
        video: false
      });
      this.localStream.getAudioTracks().forEach((t) => { t.enabled = false; });
    } catch (err) {
      this.onStatus({ error: 'mic_denied', message: err?.message || 'mic' });
      return false;
    }

    this.joined = true;
    this.muted = true;
    this.speaking = false;
    this.sendAction('VOICE_JOIN', {});
    window.addEventListener('keydown', this.boundKeyDown);
    window.addEventListener('keyup', this.boundKeyUp);
    this.emit();

    const peers = (this.getPeers() || []).filter((p) => p.id !== this.getClientId() && p.voiceJoined);
    if (peers.length >= MAX_VOICE_PEERS) {
      this.onStatus({ error: 'voice_full' });
      await this.leave();
      return false;
    }
    for (const peer of peers) {
      // One deterministic offerer prevents both sides entering have-local-offer.
      if (this.getClientId() < peer.id) {
        try {
          await this.ensurePeer(peer.id, true);
        } catch (_) {
          this.closePeer(peer.id);
        }
      }
    }
    return true;
  }

  async leave() {
    window.removeEventListener('keydown', this.boundKeyDown);
    window.removeEventListener('keyup', this.boundKeyUp);
    this.pttHeld = false;
    this.speaking = false;
    for (const [id] of this.peers) {
      this.closePeer(id);
    }
    if (this.localStream) {
      this.localStream.getTracks().forEach((t) => t.stop());
      this.localStream = null;
    }
    if (this.joined) {
      this.sendAction('VOICE_LEAVE', {});
    }
    this.joined = false;
    this.muted = true;
    this.emit();
  }

  setPTT(held) {
    if (!this.joined || !this.localStream) return;
    if (held) this.resumeRemoteAudio();
    this.pttHeld = held;
    this.speaking = held;
    this.muted = !held;
    this.localStream.getAudioTracks().forEach((t) => { t.enabled = held; });
    this.sendAction('VOICE_STATUS', { muted: !held, speaking: held });
    this.emit();
  }

  onKeyDown(e) {
    if (e.code !== 'Space' || e.repeat) return;
    const tag = (e.target && e.target.tagName) || '';
    if (tag === 'INPUT' || tag === 'TEXTAREA' || e.target?.isContentEditable) return;
    e.preventDefault();
    this.setPTT(true);
  }

  onKeyUp(e) {
    if (e.code !== 'Space') return;
    this.setPTT(false);
  }

  async ensurePeer(remoteId, isInitiator) {
    if (!this.joined || !remoteId || remoteId === this.getClientId()) return;
    if (this.peers.has(remoteId)) return;
    if (this.peers.size >= MAX_VOICE_PEERS) return;

    const pc = new RTCPeerConnection({ iceServers: this.iceServers });
    const entry = { pc, audio: null, pendingIce: [] };
    this.peers.set(remoteId, entry);

    if (this.localStream) {
      this.localStream.getTracks().forEach((track) => pc.addTrack(track, this.localStream));
    }

    pc.onicecandidate = (ev) => {
      if (!ev.candidate) return;
      this.sendAction('RTC_ICE', {
        targetId: remoteId,
        candidate: ev.candidate
      });
    };

    pc.ontrack = (ev) => {
      let audio = entry.audio;
      if (!audio) {
        audio = document.createElement('audio');
        audio.autoplay = true;
        audio.playsInline = true;
        audio.dataset.peerId = remoteId;
        document.body.appendChild(audio);
        entry.audio = audio;
      }
      audio.srcObject = ev.streams[0];
      const playPromise = audio.play();
      if (playPromise && typeof playPromise.catch === 'function') {
        playPromise.catch(() => {
          // A later push-to-talk gesture retries playback.
        });
      }
    };

    pc.onconnectionstatechange = () => {
      if (pc.connectionState === 'failed' || pc.connectionState === 'closed') {
        this.closePeer(remoteId);
      }
      this.emit();
    };

    if (isInitiator) {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      this.sendAction('RTC_OFFER', {
        targetId: remoteId,
        sdp: pc.localDescription
      });
    }
  }

  closePeer(remoteId) {
    const entry = this.peers.get(remoteId);
    if (!entry) return;
    try { entry.pc.close(); } catch (_) { /* ignore */ }
    if (entry.audio) {
      entry.audio.srcObject = null;
      entry.audio.remove();
    }
    this.peers.delete(remoteId);
  }

  async handleOffer(fromId, sdp) {
    if (!this.joined) return;
    await this.ensurePeer(fromId, false);
    const entry = this.peers.get(fromId);
    if (!entry) return;
    await entry.pc.setRemoteDescription(sdp);
    await this.flushPendingIce(entry);
    const answer = await entry.pc.createAnswer();
    await entry.pc.setLocalDescription(answer);
    this.sendAction('RTC_ANSWER', {
      targetId: fromId,
      sdp: entry.pc.localDescription
    });
  }

  async handleAnswer(fromId, sdp) {
    const entry = this.peers.get(fromId);
    if (!entry) return;
    await entry.pc.setRemoteDescription(sdp);
    await this.flushPendingIce(entry);
  }

  async handleIce(fromId, candidate) {
    const entry = this.peers.get(fromId);
    if (!entry || !candidate) return;
    if (!entry.pc.remoteDescription) {
      entry.pendingIce.push(candidate);
      return;
    }
    try {
      await entry.pc.addIceCandidate(candidate);
    } catch (_) { /* ignore stale */ }
  }

  async flushPendingIce(entry) {
    if (!entry?.pc?.remoteDescription || !entry.pendingIce?.length) return;
    const queued = entry.pendingIce.splice(0);
    for (const candidate of queued) {
      try {
        await entry.pc.addIceCandidate(candidate);
      } catch (_) { /* ignore stale */ }
    }
  }

  handlePeerJoined(remoteId) {
    if (!this.joined || remoteId === this.getClientId()) return;
    // Only the lexicographically smaller id initiates to avoid glare.
    if (this.getClientId() < remoteId) {
      this.ensurePeer(remoteId, true).catch(() => this.closePeer(remoteId));
    }
  }

  handlePeerLeft(remoteId) {
    this.closePeer(remoteId);
  }

  resumeRemoteAudio() {
    for (const { audio } of this.peers.values()) {
      if (audio) {
        const playPromise = audio.play();
        if (playPromise && typeof playPromise.catch === 'function') {
          playPromise.catch(() => {});
        }
      }
    }
  }

  emit() {
    this.onStatus({
      joined: this.joined,
      muted: this.muted,
      speaking: this.speaking,
      peerCount: this.peers.size
    });
  }
}
