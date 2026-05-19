'use strict';

const Input = (() => {
  const BUTTONS = [
    { name: 'Y',  bit: 0x0001 },
    { name: 'B',  bit: 0x0002 },
    { name: 'A',  bit: 0x0004 },
    { name: 'X',  bit: 0x0008 },
    { name: 'L',  bit: 0x0010 },
    { name: 'R',  bit: 0x0020 },
    { name: 'ZL', bit: 0x0040 },
    { name: 'ZR', bit: 0x0080 },
    { name: '-',  bit: 0x0100 },
    { name: '+',  bit: 0x0200 },
    { name: 'LC', bit: 0x0400 },
    { name: 'RC', bit: 0x0800 },
    { name: 'HOME', bit: 0x1000 },
    { name: 'CAP',  bit: 0x2000 },
  ];

  // Mappable targets: each Switch button + dpad + sticks
  const GP_TARGETS = [
    ...BUTTONS.map(b => ({ type: 'button', name: b.name, bit: b.bit })),
    { type: 'dpad-up',    name: 'D-Up' },
    { type: 'dpad-down',  name: 'D-Down' },
    { type: 'dpad-left',  name: 'D-Left' },
    { type: 'dpad-right', name: 'D-Right' },
  ];

  // Default: Standard Gamepad Layout mapping (gpButtonIndex -> target name)
  const DEFAULT_GP_MAP = {
    0: 'B', 1: 'A', 2: 'Y', 3: 'X',
    4: 'L', 5: 'R', 6: 'ZL', 7: 'ZR',
    8: '-', 9: '+', 10: 'LC', 11: 'RC',
    12: 'D-Up', 13: 'D-Down', 14: 'D-Left', 15: 'D-Right',
    16: 'HOME', 17: 'CAP',
  };

  const STORAGE_KEY = 'nxmc-gp-map';
  let gpMap = {};
  let gpAxisMap = { lx: 0, ly: 1, rx: 2, ry: 3 };

  function loadGpMap() {
    try {
      const saved = JSON.parse(localStorage.getItem(STORAGE_KEY));
      if (saved && saved.buttons) {
        gpMap = saved.buttons;
        if (saved.axes) gpAxisMap = saved.axes;
        return;
      }
    } catch { /* ignore */ }
    gpMap = { ...DEFAULT_GP_MAP };
  }

  function saveGpMap() {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ buttons: gpMap, axes: gpAxisMap }));
  }

  function setGpMapping(gpIndex, targetName) {
    gpMap[gpIndex] = targetName;
    saveGpMap();
  }

  function clearGpMapping(gpIndex) {
    delete gpMap[gpIndex];
    saveGpMap();
  }

  function resetGpMap() {
    gpMap = { ...DEFAULT_GP_MAP };
    gpAxisMap = { lx: 0, ly: 1, rx: 2, ry: 3 };
    saveGpMap();
  }

  function getGpMap() { return gpMap; }
  function getGpTargets() { return GP_TARGETS; }

  const KEY_MAP = {
    'KeyW':       { type: 'lstick', dir: 'up' },
    'KeyA':       { type: 'lstick', dir: 'left' },
    'KeyS':       { type: 'lstick', dir: 'down' },
    'KeyD':       { type: 'lstick', dir: 'right' },
    'ArrowUp':    { type: 'hat', hat: 0 },
    'ArrowRight': { type: 'hat', hat: 2 },
    'ArrowDown':  { type: 'hat', hat: 4 },
    'ArrowLeft':  { type: 'hat', hat: 6 },
    'KeyK':       { type: 'button', bit: 0x0004 },
    'KeyJ':       { type: 'button', bit: 0x0002 },
    'KeyI':       { type: 'button', bit: 0x0008 },
    'KeyU':       { type: 'button', bit: 0x0001 },
    'KeyQ':       { type: 'button', bit: 0x0010 },
    'KeyE':       { type: 'button', bit: 0x0020 },
    'KeyZ':       { type: 'button', bit: 0x0040 },
    'KeyC':       { type: 'button', bit: 0x0080 },
    'Minus':      { type: 'button', bit: 0x0100 },
    'Equal':      { type: 'button', bit: 0x0200 },
    'KeyF':       { type: 'button', bit: 0x0400 },
    'Semicolon':  { type: 'button', bit: 0x0800 },
    'KeyH':       { type: 'button', bit: 0x1000 },
    'KeyG':       { type: 'button', bit: 0x2000 },
    'KeyO':       { type: 'rstick', dir: 'up' },
    'KeyL':       { type: 'rstick', dir: 'right' },
    'Period':     { type: 'rstick', dir: 'down' },
    'Comma':      { type: 'rstick', dir: 'left' },
  };

  const keyState = {};
  let gamepadConnected = false;

  function hatFromDpad(up, right, down, left) {
    if (up && right) return 1;
    if (right && down) return 3;
    if (down && left) return 5;
    if (left && up) return 7;
    if (up) return 0;
    if (right) return 2;
    if (down) return 4;
    if (left) return 6;
    return 8;
  }

  function axisToStick(val) {
    return Math.max(0, Math.min(255, Math.round((val + 1) * 127.5)));
  }

  function init() {
    loadGpMap();
    document.addEventListener('keydown', (e) => {
      if (KEY_MAP[e.code]) { e.preventDefault(); keyState[e.code] = true; }
    });
    document.addEventListener('keyup', (e) => {
      if (KEY_MAP[e.code]) { e.preventDefault(); keyState[e.code] = false; }
    });
    window.addEventListener('gamepadconnected', () => { gamepadConnected = true; });
    window.addEventListener('gamepaddisconnected', () => {
      gamepadConnected = navigator.getGamepads().some(g => g !== null);
    });
  }

  function resolveGpTarget(targetName) {
    const btn = BUTTONS.find(b => b.name === targetName);
    if (btn) return { type: 'button', bit: btn.bit };
    if (targetName === 'D-Up') return { type: 'dpad', dir: 'up' };
    if (targetName === 'D-Down') return { type: 'dpad', dir: 'down' };
    if (targetName === 'D-Left') return { type: 'dpad', dir: 'left' };
    if (targetName === 'D-Right') return { type: 'dpad', dir: 'right' };
    return null;
  }

  function buildReport(touchState) {
    let buttons = 0, hat = 8;
    let lx = 128, ly = 128, rx = 128, ry = 128;

    // Keyboard
    let kbLU = false, kbLD = false, kbLL = false, kbLR = false;
    let kbRU = false, kbRD = false, kbRL = false, kbRR = false;
    let kbHatU = false, kbHatD = false, kbHatL = false, kbHatR = false;

    for (const [code, pressed] of Object.entries(keyState)) {
      if (!pressed) continue;
      const m = KEY_MAP[code];
      if (!m) continue;
      if (m.type === 'button') buttons |= m.bit;
      else if (m.type === 'hat') {
        if (m.hat === 0) kbHatU = true;
        if (m.hat === 2) kbHatR = true;
        if (m.hat === 4) kbHatD = true;
        if (m.hat === 6) kbHatL = true;
      } else if (m.type === 'lstick') {
        if (m.dir === 'up') kbLU = true;
        if (m.dir === 'down') kbLD = true;
        if (m.dir === 'left') kbLL = true;
        if (m.dir === 'right') kbLR = true;
      } else if (m.type === 'rstick') {
        if (m.dir === 'up') kbRU = true;
        if (m.dir === 'down') kbRD = true;
        if (m.dir === 'left') kbRL = true;
        if (m.dir === 'right') kbRR = true;
      }
    }

    hat = hatFromDpad(kbHatU, kbHatR, kbHatD, kbHatL);
    if (kbLL) lx = 0;   else if (kbLR) lx = 255;
    if (kbLU) ly = 0;   else if (kbLD) ly = 255;
    if (kbRL) rx = 0;   else if (kbRR) rx = 255;
    if (kbRU) ry = 0;   else if (kbRD) ry = 255;

    // Gamepad (custom mapping)
    let gpDpadU = false, gpDpadD = false, gpDpadL = false, gpDpadR = false;
    const gamepads = navigator.getGamepads();
    for (const gp of gamepads) {
      if (!gp) continue;
      for (let i = 0; i < gp.buttons.length; i++) {
        if (!gp.buttons[i].pressed) continue;
        const targetName = gpMap[i];
        if (!targetName) continue;
        const target = resolveGpTarget(targetName);
        if (!target) continue;
        if (target.type === 'button') buttons |= target.bit;
        else if (target.type === 'dpad') {
          if (target.dir === 'up') gpDpadU = true;
          if (target.dir === 'down') gpDpadD = true;
          if (target.dir === 'left') gpDpadL = true;
          if (target.dir === 'right') gpDpadR = true;
        }
      }
      const gpHat = hatFromDpad(gpDpadU, gpDpadR, gpDpadD, gpDpadL);
      if (gpHat !== 8) hat = gpHat;

      const deadzone = 0.15;
      const a = gpAxisMap;
      if (gp.axes.length > Math.max(a.lx, a.ly)) {
        if (Math.abs(gp.axes[a.lx]) > deadzone || Math.abs(gp.axes[a.ly]) > deadzone) {
          lx = axisToStick(gp.axes[a.lx]);
          ly = axisToStick(gp.axes[a.ly]);
        }
      }
      if (gp.axes.length > Math.max(a.rx, a.ry)) {
        if (Math.abs(gp.axes[a.rx]) > deadzone || Math.abs(gp.axes[a.ry]) > deadzone) {
          rx = axisToStick(gp.axes[a.rx]);
          ry = axisToStick(gp.axes[a.ry]);
        }
      }
      break;
    }

    // Touch (OR merge)
    if (touchState) {
      buttons |= touchState.buttons;
      if (touchState.hat !== 8) hat = touchState.hat;
      if (touchState.lx !== 128 || touchState.ly !== 128) { lx = touchState.lx; ly = touchState.ly; }
      if (touchState.rx !== 128 || touchState.ry !== 128) { rx = touchState.rx; ry = touchState.ry; }
    }

    return { buttons, hat, lx, ly, rx, ry };
  }

  function isGamepadConnected() { return gamepadConnected; }

  return {
    BUTTONS, KEY_MAP, GP_TARGETS,
    init, buildReport, isGamepadConnected,
    getGpMap, setGpMapping, clearGpMapping, resetGpMap, getGpTargets,
  };
})();
