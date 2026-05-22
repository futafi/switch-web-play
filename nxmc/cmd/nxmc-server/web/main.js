'use strict';

(function () {
  const video = document.getElementById('video');
  const statusDot = document.getElementById('status-indicator');
  let ws = null;
  let lastSent = '';

  // WebSocket
  function connect() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    ws = new WebSocket(`${proto}//${location.host}/ws`);
    ws.onopen = () => {
      statusDot.className = 'status-dot connected';
      statusDot.title = 'connected';
    };
    ws.onclose = () => {
      statusDot.className = 'status-dot disconnected';
      statusDot.title = 'disconnected';
      setTimeout(connect, 1000);
    };
  }

  // Auto-hide gear & status in fullscreen
  const chrome = [document.getElementById('settings-toggle'), document.getElementById('status-indicator')];
  let hideTimer = null;

  function showChrome() {
    chrome.forEach(el => el.classList.remove('chrome-hidden'));
    clearTimeout(hideTimer);
    if (document.fullscreenElement) {
      hideTimer = setTimeout(() => chrome.forEach(el => el.classList.add('chrome-hidden')), 3000);
    }
  }

  document.addEventListener('fullscreenchange', () => {
    if (document.fullscreenElement) {
      showChrome();
    } else {
      clearTimeout(hideTimer);
      chrome.forEach(el => el.classList.remove('chrome-hidden'));
    }
  });

  document.addEventListener('mousemove', showChrome);
  document.addEventListener('touchstart', showChrome);

  // Double-tap/click fullscreen
  let lastTap = 0;
  video.addEventListener('dblclick', toggleFullscreen);
  video.addEventListener('touchend', (e) => {
    const now = Date.now();
    if (now - lastTap < 300) {
      e.preventDefault();
      toggleFullscreen();
    }
    lastTap = now;
  });

  function toggleFullscreen() {
    if (document.fullscreenElement) {
      document.exitFullscreen();
    } else {
      document.documentElement.requestFullscreen().catch(() => {});
    }
  }

  // Gamepad auto-hide
  window.addEventListener('gamepadconnected', () => {
    Settings.onGamepadChange(true);
  });

  // Main loop
  function loop() {
    const touchState = Touch.getState();
    const report = Input.buildReport(touchState);

    const json = JSON.stringify(report);
    if (json !== lastSent && ws && ws.readyState === WebSocket.OPEN) {
      ws.send(json);
      lastSent = json;
    }

    requestAnimationFrame(loop);
  }

  // Init
  Input.init();
  Touch.init();
  Settings.init();
  connect();
  WHEP.start(video).catch(() => {
    setTimeout(() => WHEP.reconnect(video), 3000);
  });
  requestAnimationFrame(loop);
})();
