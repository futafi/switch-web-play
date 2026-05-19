'use strict';

const WHEP = (() => {
  let pc = null;
  let sessionUrl = null;

  async function start(videoElement) {
    stop();

    pc = new RTCPeerConnection();
    pc.addTransceiver('video', { direction: 'recvonly' });
    pc.addTransceiver('audio', { direction: 'recvonly' });

    pc.ontrack = (ev) => {
      if (!videoElement.srcObject) {
        videoElement.srcObject = ev.streams[0];
      }
    };

    pc.oniceconnectionstatechange = () => {
      if (pc && (pc.iceConnectionState === 'failed' || pc.iceConnectionState === 'disconnected')) {
        setTimeout(() => reconnect(videoElement), 2000);
      }
    };

    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);

    const whepUrl = `${location.protocol}//${location.hostname}:8889/cam/whep`;
    const res = await fetch(whepUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: pc.localDescription.sdp,
    });

    if (!res.ok) throw new Error(`WHEP ${res.status}`);

    const answerSdp = await res.text();
    await pc.setRemoteDescription({ type: 'answer', sdp: answerSdp });

    const loc = res.headers.get('Location');
    if (loc) {
      sessionUrl = loc.startsWith('http') ? loc : new URL(loc, whepUrl).href;
    }
  }

  function stop() {
    if (pc) {
      pc.close();
      pc = null;
    }
    if (sessionUrl) {
      fetch(sessionUrl, { method: 'DELETE' }).catch(() => {});
      sessionUrl = null;
    }
  }

  async function reconnect(videoElement) {
    stop();
    try {
      await start(videoElement);
    } catch {
      setTimeout(() => reconnect(videoElement), 3000);
    }
  }

  return { start, stop, reconnect };
})();
