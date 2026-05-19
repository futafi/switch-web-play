'use strict';

const Settings = (() => {
  const STORAGE_KEY = 'nxmc-settings';
  const defaults = { overlay: false, opacity: 50, resolution: '1080', bitrate: '12000' };
  let current = { ...defaults };

  function load() {
    try {
      const saved = JSON.parse(localStorage.getItem(STORAGE_KEY));
      if (saved) Object.assign(current, saved);
    } catch { /* ignore */ }
  }

  function save() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(current));
  }

  function init() {
    load();

    const overlayEl = document.getElementById('overlay');
    const panel = document.getElementById('settings-panel');
    const toggle = document.getElementById('settings-toggle');
    const close = document.getElementById('settings-close');
    const optOverlay = document.getElementById('opt-overlay');
    const optOpacity = document.getElementById('opt-opacity');
    const optOpacityVal = document.getElementById('opt-opacity-val');

    optOverlay.checked = current.overlay;
    optOpacity.value = current.opacity;
    optOpacityVal.textContent = current.opacity + '%';
    applyOverlay(overlayEl);
    applyOpacity(overlayEl);
    applyRadio('resolution', current.resolution);
    applyRadio('bitrate', current.bitrate);

    toggle.addEventListener('click', () => panel.classList.toggle('hidden'));
    close.addEventListener('click', () => panel.classList.add('hidden'));

    optOverlay.addEventListener('change', () => {
      current.overlay = optOverlay.checked;
      applyOverlay(overlayEl);
      save();
    });

    optOpacity.addEventListener('input', () => {
      current.opacity = parseInt(optOpacity.value);
      optOpacityVal.textContent = current.opacity + '%';
      applyOpacity(overlayEl);
      save();
    });

    document.querySelectorAll('input[name="resolution"]').forEach(r => {
      r.addEventListener('change', () => { current.resolution = r.value; save(); changeStream(); });
    });
    document.querySelectorAll('input[name="bitrate"]').forEach(r => {
      r.addEventListener('change', () => { current.bitrate = r.value; save(); changeStream(); });
    });

    initKeyhelpModal();
    initGpMapModal();
  }

  function initKeyhelpModal() {
    const btn = document.getElementById('keyhelp-btn');
    const modal = document.getElementById('keyhelp-modal');
    const close = document.getElementById('keyhelp-close');
    btn.addEventListener('click', () => modal.classList.remove('hidden'));
    close.addEventListener('click', () => modal.classList.add('hidden'));
    modal.addEventListener('click', (e) => { if (e.target === modal) modal.classList.add('hidden'); });
  }

  // --- Gamepad mapping modal ---
  let listenTarget = null;
  let listenRaf = null;

  function initGpMapModal() {
    const btn = document.getElementById('gpmap-btn');
    const modal = document.getElementById('gpmap-modal');
    const close = document.getElementById('gpmap-close');
    const resetBtn = document.getElementById('gpmap-reset');
    const listenEl = document.getElementById('gpmap-listen');
    const listenCancel = document.getElementById('gpmap-listen-cancel');
    const listenClear = document.getElementById('gpmap-listen-clear');

    btn.addEventListener('click', () => { modal.classList.remove('hidden'); renderGpMap(); });
    close.addEventListener('click', () => { stopListen(); modal.classList.add('hidden'); });
    modal.addEventListener('click', (e) => { if (e.target === modal) { stopListen(); modal.classList.add('hidden'); } });

    resetBtn.addEventListener('click', () => {
      Input.resetGpMap();
      renderGpMap();
    });

    listenCancel.addEventListener('click', stopListen);
    listenClear.addEventListener('click', () => {
      if (listenTarget !== null) {
        // Find and clear any mapping to this target
        const map = Input.getGpMap();
        for (const [idx, name] of Object.entries(map)) {
          if (name === listenTarget) Input.clearGpMapping(parseInt(idx));
        }
        stopListen();
        renderGpMap();
      }
    });
  }

  function renderGpMap() {
    const list = document.getElementById('gpmap-list');
    const targets = Input.getGpTargets();
    const map = Input.getGpMap();

    // Invert: target -> gpIndex
    const inverse = {};
    for (const [idx, name] of Object.entries(map)) {
      inverse[name] = parseInt(idx);
    }

    list.innerHTML = '';
    for (const t of targets) {
      const row = document.createElement('div');
      row.className = 'gpmap-row';
      const assigned = inverse[t.name];
      row.innerHTML =
        `<span class="gp-target">${t.name}</span>` +
        `<span class="gp-index">${assigned !== undefined ? 'Button ' + assigned : '—'}</span>`;
      row.addEventListener('click', () => startListen(t.name));
      list.appendChild(row);
    }

    updateGamepadInfo();
  }

  function updateGamepadInfo() {
    const el = document.getElementById('gamepad-info');
    const gamepads = navigator.getGamepads();
    for (const gp of gamepads) {
      if (gp) { el.textContent = gp.id; return; }
    }
    el.textContent = 'No gamepad';
  }

  function startListen(targetName) {
    listenTarget = targetName;
    const listenEl = document.getElementById('gpmap-listen');
    document.getElementById('gpmap-listen-target').textContent = targetName;
    listenEl.classList.remove('hidden');
    pollForButton();
  }

  function stopListen() {
    listenTarget = null;
    if (listenRaf) { cancelAnimationFrame(listenRaf); listenRaf = null; }
    document.getElementById('gpmap-listen').classList.add('hidden');
  }

  function pollForButton() {
    if (!listenTarget) return;
    const gamepads = navigator.getGamepads();
    for (const gp of gamepads) {
      if (!gp) continue;
      for (let i = 0; i < gp.buttons.length; i++) {
        if (gp.buttons[i].pressed) {
          Input.setGpMapping(i, listenTarget);
          stopListen();
          renderGpMap();
          return;
        }
      }
    }
    listenRaf = requestAnimationFrame(pollForButton);
  }

  function applyOverlay(el) { el.classList.toggle('hidden', !current.overlay); }
  function applyOpacity(el) { el.style.opacity = current.opacity / 100; }
  function applyRadio(name, value) {
    const el = document.querySelector(`input[name="${name}"][value="${value}"]`);
    if (el) el.checked = true;
  }

  const RESOLUTION_MAP = {
    '1080': { width: 1920, height: 1080 },
    '720':  { width: 1280, height: 720 },
    '480':  { width: 854,  height: 480 },
    '360':  { width: 640,  height: 360 },
  };

  async function changeStream() {
    const res = RESOLUTION_MAP[current.resolution] || RESOLUTION_MAP['1080'];
    const body = { width: res.width, height: res.height, bitrate: parseInt(current.bitrate) };
    try {
      await fetch('/api/stream', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      });
    } catch { /* mock */ }
  }

  function onGamepadChange(connected) {
    if (connected && current.overlay) {
      current.overlay = false;
      document.getElementById('opt-overlay').checked = false;
      applyOverlay(document.getElementById('overlay'));
      save();
    }
  }

  function get() { return current; }

  return { init, get, onGamepadChange };
})();
