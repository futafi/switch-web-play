'use strict';

const WHEP = (() => {
  let pc = null;
  let sessionUrl = null;
  let videoElement = null;
  let reconnectTimer = null;
  let generation = 0;
  let stopped = true;

  function sleep(ms) {
    return new Promise(resolve => setTimeout(resolve, ms));
  }

  function clearReconnectTimer() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function closeCurrent() {
    const oldPc = pc;
    const oldSessionUrl = sessionUrl;
    pc = null;
    sessionUrl = null;

    if (oldPc) oldPc.close();
    if (oldSessionUrl) {
      fetch(oldSessionUrl, { method: 'DELETE' }).catch(() => {});
    }
  }

  async function connect(token) {
    if (!videoElement) throw new Error('missing video element');
    closeCurrent();

    const nextPc = new RTCPeerConnection();
    pc = nextPc;
    nextPc.addTransceiver('video', { direction: 'recvonly' });
    nextPc.addTransceiver('audio', { direction: 'recvonly' });

    nextPc.ontrack = (ev) => {
      if (token === generation && nextPc === pc) {
        videoElement.srcObject = ev.streams[0];
      }
    };

    nextPc.oniceconnectionstatechange = () => {
      if (
        token === generation &&
        nextPc === pc &&
        (nextPc.iceConnectionState === 'failed' || nextPc.iceConnectionState === 'disconnected')
      ) {
        scheduleReconnect(2000);
      }
    };

    const offer = await nextPc.createOffer();
    await nextPc.setLocalDescription(offer);

    const whepUrl = `${location.protocol}//${location.hostname}:8889/cam/whep`;
    const res = await fetch(whepUrl, {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: nextPc.localDescription.sdp,
    });

    if (!res.ok) throw new Error(`WHEP ${res.status}`);

    const answerSdp = await res.text();
    const loc = res.headers.get('Location');
    const nextSessionUrl = loc ? (loc.startsWith('http') ? loc : new URL(loc, whepUrl).href) : null;
    if (token !== generation || nextPc !== pc) {
      nextPc.close();
      if (nextSessionUrl) {
        fetch(nextSessionUrl, { method: 'DELETE' }).catch(() => {});
      }
      return;
    }
    await nextPc.setRemoteDescription({ type: 'answer', sdp: answerSdp });

    sessionUrl = nextSessionUrl;
  }

  async function start(video) {
    videoElement = video;
    stopped = false;
    clearReconnectTimer();
    const token = ++generation;
    await connect(token);
  }

  function stop() {
    stopped = true;
    generation++;
    clearReconnectTimer();
    closeCurrent();
  }

  function scheduleReconnect(delayMs) {
    if (stopped) return;
    clearReconnectTimer();
    const token = generation;
    reconnectTimer = setTimeout(() => {
      reconnect(null, 0, token);
    }, delayMs);
  }

  async function reconnect(video, delayMs = 0, existingToken = null) {
    if (video) videoElement = video;
    if (!videoElement) return;

    stopped = false;
    clearReconnectTimer();
    const token = existingToken === generation ? existingToken : ++generation;

    if (delayMs > 0) await sleep(delayMs);

    while (!stopped && token === generation) {
      try {
        await connect(token);
        return;
      } catch {
        if (token === generation && !stopped) {
          await sleep(1000);
        }
      }
    }
  }

  return { start, stop, reconnect };
})();
