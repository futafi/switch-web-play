'use strict';

const Touch = (() => {
  let state = { buttons: 0, hat: 8, lx: 128, ly: 128, rx: 128, ry: 128 };
  const activeTouches = {};
  const STICK_MAX_RADIUS = 50;
  const MOUSE_ID = 'mouse';

  function init() {
    const overlay = document.getElementById('overlay');

    // Touch events
    overlay.addEventListener('touchstart', onTouch, { passive: false });
    overlay.addEventListener('touchmove', onTouch, { passive: false });
    overlay.addEventListener('touchend', onTouchEnd, { passive: false });
    overlay.addEventListener('touchcancel', onTouchEnd, { passive: false });

    // Mouse events
    overlay.addEventListener('mousedown', onMouseDown);
    window.addEventListener('mousemove', onMouseMove);
    window.addEventListener('mouseup', onMouseUp);
  }

  // --- Mouse handlers ---
  function onMouseDown(e) {
    e.preventDefault();
    handlePointerDown(MOUSE_ID, e.clientX, e.clientY);
  }

  function onMouseMove(e) {
    if (!activeTouches[MOUSE_ID]) return;
    handlePointerMove(MOUSE_ID, e.clientX, e.clientY);
  }

  function onMouseUp(e) {
    if (!activeTouches[MOUSE_ID]) return;
    handlePointerUp(MOUSE_ID);
  }

  // --- Touch handlers ---
  function onTouch(e) {
    e.preventDefault();
    for (const t of e.changedTouches) {
      const existing = activeTouches[t.identifier];
      if (existing && existing.type === 'stick') {
        handlePointerMove(t.identifier, t.clientX, t.clientY);
        continue;
      }
      handlePointerDown(t.identifier, t.clientX, t.clientY);
    }
  }

  function onTouchEnd(e) {
    e.preventDefault();
    for (const t of e.changedTouches) {
      handlePointerUp(t.identifier);
    }
  }

  // --- Unified pointer logic ---
  function handlePointerDown(id, x, y) {
    const el = document.elementFromPoint(x, y);
    if (!el) return;
    const btn = el.closest('[data-btn]');
    const hatEl = el.closest('[data-hat]');
    const stickZone = el.closest('.stick-zone');

    if (btn) {
      activeTouches[id] = { type: 'button', bit: parseInt(btn.dataset.btn) };
    } else if (hatEl) {
      activeTouches[id] = { type: 'hat', hat: parseInt(hatEl.dataset.hat) };
    } else if (stickZone) {
      const isLeft = stickZone.id === 'lstick-zone';
      const rect = stickZone.getBoundingClientRect();
      const cx = rect.left + rect.width / 2;
      const cy = rect.top + rect.height / 2;
      activeTouches[id] = { type: 'stick', isLeft, cx, cy };
      updateStick(x, y, activeTouches[id]);
    }
    rebuildTouchState();
  }

  function handlePointerMove(id, x, y) {
    const info = activeTouches[id];
    if (!info) return;
    if (info.type === 'stick') {
      updateStick(x, y, info);
    }
    rebuildTouchState();
  }

  function handlePointerUp(id) {
    const info = activeTouches[id];
    if (info && info.type === 'stick') {
      resetStickKnob(info.isLeft);
    }
    delete activeTouches[id];
    rebuildTouchState();
  }

  // --- Stick math ---
  function updateStick(x, y, info) {
    const dx = x - info.cx;
    const dy = y - info.cy;
    const dist = Math.sqrt(dx * dx + dy * dy);
    const clamped = Math.min(dist, STICK_MAX_RADIUS);
    const angle = Math.atan2(dy, dx);
    const nx = (clamped / STICK_MAX_RADIUS) * Math.cos(angle);
    const ny = (clamped / STICK_MAX_RADIUS) * Math.sin(angle);

    const val = (n) => Math.max(0, Math.min(255, Math.round((n + 1) * 127.5)));
    info.sx = val(nx);
    info.sy = val(ny);
    moveStickKnob(info.isLeft, nx, ny);
  }

  function moveStickKnob(isLeft, nx, ny) {
    const knob = document.getElementById(isLeft ? 'lstick-knob' : 'rstick-knob');
    knob.style.left = ((nx + 1) / 2 * 100) + '%';
    knob.style.top = ((ny + 1) / 2 * 100) + '%';
  }

  function resetStickKnob(isLeft) {
    const knob = document.getElementById(isLeft ? 'lstick-knob' : 'rstick-knob');
    knob.style.left = '50%';
    knob.style.top = '50%';
  }

  // --- State rebuild ---
  function rebuildTouchState() {
    let buttons = 0, hat = 8;
    let lx = 128, ly = 128, rx = 128, ry = 128;
    const pressedBtns = new Set();
    const pressedHats = new Set();

    for (const info of Object.values(activeTouches)) {
      if (info.type === 'button') {
        buttons |= info.bit;
        pressedBtns.add(info.bit);
      } else if (info.type === 'hat') {
        hat = info.hat;
        pressedHats.add(info.hat);
      } else if (info.type === 'stick' && info.sx !== undefined) {
        if (info.isLeft) { lx = info.sx; ly = info.sy; }
        else { rx = info.sx; ry = info.sy; }
      }
    }

    state = { buttons, hat, lx, ly, rx, ry };

    document.querySelectorAll('[data-btn]').forEach(el => {
      el.classList.toggle('pressed', pressedBtns.has(parseInt(el.dataset.btn)));
    });
    document.querySelectorAll('[data-hat]').forEach(el => {
      el.classList.toggle('pressed', pressedHats.has(parseInt(el.dataset.hat)));
    });
  }

  function getState() { return state; }

  return { init, getState };
})();
