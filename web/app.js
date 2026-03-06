'use strict';

// ── CONFIG ──────────────────────────────────────────────────
const ORT_CDN = 'https://cdn.jsdelivr.net/npm/onnxruntime-web@1.18.0/dist/';
const MODEL_PATH = '/models/model.onnx';

// MIDI constants
const WALDORF_ID  = 0x3E;
const BLOFELD_ID  = 0x13;
const DEVICE_ID   = 0x00;
const MSG_SNDD    = 0x10;

// Rhythm timing (ms) at 120 BPM
const Q = 500;   // quarter note
const E = 250;   // eighth
const S = 125;   // sixteenth
const T = Q / 3; // triplet eighth ≈ 167ms

const PATTERNS = {
  '4x4':   Array.from({length: 16}, (_, i) => ({ t: i * Q, note: 36 })),
  'clave':  [0, 3, 6, 8, 11, 16, 19, 22, 24, 27].map(p => ({ t: p * S, note: 60 })),
  'gallop': (() => {
    const hits = [];
    for (let b = 0; b < 16; b++) hits.push(
      { t: b*Q,       note: 36 },
      { t: b*Q + T,   note: 36 },
      { t: b*Q + 2*T, note: 36 }
    );
    return hits;
  })(),
};

const CHORD_DURATION = 10000; // ms

// ── STATE ───────────────────────────────────────────────────
let midiAccess  = null;
let midiOutput  = null;
let onnxSession = null;
let blofeldMap  = null;
let dimensions  = null;

const dims   = new Float32Array(16).fill(0.5);  // semantic inputs
let   params = new Float32Array(85).fill(0.5);  // model output

let rhythmTimers  = [];
let activePattern = null;
let activeChord   = null;
let chordTimer    = null;

// ── INIT ────────────────────────────────────────────────────
async function init() {
  // Load assets and build UI first — independent of ONNX
  try {
    [blofeldMap, dimensions] = await Promise.all([
      fetch('/assets/blofeld_map.json').then(r => r.json()),
      fetch('/assets/dimensions.json').then(r => r.json()),
    ]);
  } catch (e) {
    setStatus('ASSET LOAD ERROR: ' + e.message, 'error');
    console.error(e);
    return;
  }

  buildKnobGrid();
  bindControls();
  setStatus('LOADING MODEL…', null);

  // Load ONNX separately so knobs are visible even if it fails
  try {
    ort.env.wasm.wasmPaths = ORT_CDN;
    ort.env.wasm.numThreads = 1;
    onnxSession = await ort.InferenceSession.create(MODEL_PATH);
    await runInference();
    setStatus('READY', null);
  } catch (e) {
    setStatus('MODEL ERROR: ' + e.message, 'error');
    console.error('ONNX load failed:', e);
  }
}

// ── STATUS ──────────────────────────────────────────────────
function setStatus(text, state) {
  document.getElementById('status-text').textContent = text;
  const dot = document.getElementById('status-dot');
  dot.className = state || '';
}

// ── MIDI ────────────────────────────────────────────────────
async function connectMIDI() {
  setStatus('REQUESTING MIDI…', null);
  try {
    midiAccess = await navigator.requestMIDIAccess({ sysex: true });
    findBlofeld();
    midiAccess.onstatechange = findBlofeld;
  } catch (e) {
    setStatus('MIDI DENIED', 'error');
  }
}

function findBlofeld() {
  midiOutput = null;
  for (const [, out] of midiAccess.outputs) {
    if (out.name.toLowerCase().includes('blofeld')) {
      midiOutput = out;
      break;
    }
  }
  // Fall back to first available output if none named "blofeld"
  if (!midiOutput && midiAccess.outputs.size > 0) {
    midiOutput = midiAccess.outputs.values().next().value;
  }
  if (midiOutput) {
    setStatus('CONNECTED: ' + midiOutput.name, 'connected');
    document.getElementById('status-dot').classList.add('connected');
  } else {
    setStatus('NO MIDI OUTPUT', 'error');
  }
}

function midiChannel() {
  return parseInt(document.getElementById('inp-channel').value, 10) - 1; // 0-based
}

function midiSend(bytes) {
  const hex = bytes.map(b => b.toString(16).padStart(2, '0')).join(' ');
  console.log('[MIDI →]', hex);
  midiOutput.send(bytes);
}

function noteOn(note, vel = 100) {
  if (!midiOutput) return;
  midiSend([0x90 | midiChannel(), note, vel]);
}

function noteOff(note) {
  if (!midiOutput) return;
  midiSend([0x80 | midiChannel(), note, 0]);
}

// ── INFERENCE ───────────────────────────────────────────────
async function runInference() {
  if (!onnxSession) return;
  const tensor = new ort.Tensor('float32', dims.slice(), [1, 16]);
  const out = await onnxSession.run({ dims: tensor });
  params = out.params.data;
}

// ── SYSEX ───────────────────────────────────────────────────
function buildSData(paramValues) {
  const sdata = new Uint8Array(blofeldMap.default_sdata);
  blofeldMap.param_indices.forEach((sdataIdx, i) => {
    sdata[sdataIdx] = Math.min(127, Math.max(0, Math.round(paramValues[i] * 127)));
  });
  return sdata;
}

function sendPatch() {
  if (!midiOutput) { alert('Not connected to MIDI'); return; }

  const bank  = document.getElementById('sel-bank').value;
  const patch = parseInt(document.getElementById('inp-patch').value, 10);
  const bankByte = bank.charCodeAt(0) - 65;   // A=0 … H=7
  const progByte = patch - 1;

  const sdata = buildSData(params);
  let checksum = 0;
  for (const b of sdata) checksum = (checksum + b) & 0x7F;

  const msg = [
    0xF0, WALDORF_ID, BLOFELD_ID, DEVICE_ID, MSG_SNDD,
    bankByte, progByte,
    ...sdata,
    checksum,
    0xF7,
  ];
  // Log header + first/last few bytes (full msg is 392 bytes)
  const header = msg.slice(0, 7).map(b => b.toString(16).padStart(2, '0')).join(' ');
  const tail   = msg.slice(-3).map(b => b.toString(16).padStart(2, '0')).join(' ');
  console.log(`[MIDI → SysEx] ${header} ... <${msg.length - 10} sdata bytes> ... ${tail}`);
  console.log('[MIDI → SysEx] bank=%s prog=%d checksum=0x%s', bank, patch, msg[msg.length - 2].toString(16));
  midiSend(msg);
}

// ── KNOBS ───────────────────────────────────────────────────
const R = 28; // SVG knob radius

function ptOnArc(deg) {
  const rad = deg * Math.PI / 180;
  return [R * Math.cos(rad), R * Math.sin(rad)];
}

function arcPath(startDeg, spanDeg) {
  if (spanDeg < 1) return '';
  const endDeg = startDeg + spanDeg;
  const [x1, y1] = ptOnArc(startDeg);
  const [x2, y2] = ptOnArc(endDeg);
  const large = spanDeg > 180 ? 1 : 0;
  return `M ${x1.toFixed(2)} ${y1.toFixed(2)} A ${R} ${R} 0 ${large} 1 ${x2.toFixed(2)} ${y2.toFixed(2)}`;
}

let _knobGradId = 0;

function makeKnobSVG(idx) {
  const ns  = 'http://www.w3.org/2000/svg';
  const gid = `kg${_knobGradId++}`;

  const svg = document.createElementNS(ns, 'svg');
  svg.setAttribute('viewBox', '-36 -36 72 72');
  svg.setAttribute('width', '72');
  svg.setAttribute('height', '72');

  // ── Defs: metallic radial gradient + drop shadow ──
  const defs = document.createElementNS(ns, 'defs');

  const grad = document.createElementNS(ns, 'radialGradient');
  grad.setAttribute('id', gid);
  grad.setAttribute('cx', '38%');
  grad.setAttribute('cy', '32%');
  grad.setAttribute('r',  '62%');
  [
    ['0%',   '#707070'],
    ['30%',  '#3a3a3a'],
    ['70%',  '#1e1e1e'],
    ['100%', '#111'],
  ].forEach(([offset, color]) => {
    const s = document.createElementNS(ns, 'stop');
    s.setAttribute('offset', offset);
    s.setAttribute('stop-color', color);
    grad.appendChild(s);
  });

  const filter = document.createElementNS(ns, 'filter');
  filter.setAttribute('id', `${gid}s`);
  filter.setAttribute('x', '-20%');
  filter.setAttribute('y', '-20%');
  filter.setAttribute('width', '140%');
  filter.setAttribute('height', '140%');
  const feDropShadow = document.createElementNS(ns, 'feDropShadow');
  feDropShadow.setAttribute('dx', '0');
  feDropShadow.setAttribute('dy', '2');
  feDropShadow.setAttribute('stdDeviation', '3');
  feDropShadow.setAttribute('flood-color', 'rgba(0,0,0,0.5)');
  filter.appendChild(feDropShadow);

  defs.appendChild(grad);
  defs.appendChild(filter);
  svg.appendChild(defs);

  // ── Track arc (270°, dim) ──
  const track = document.createElementNS(ns, 'path');
  track.setAttribute('d', arcPath(135, 270));
  track.setAttribute('fill', 'none');
  track.setAttribute('stroke', '#bbb');
  track.setAttribute('stroke-width', '3');
  track.setAttribute('stroke-linecap', 'round');
  svg.appendChild(track);

  // ── Value arc (white) ──
  const arc = document.createElementNS(ns, 'path');
  arc.setAttribute('fill', 'none');
  arc.setAttribute('stroke', '#111');
  arc.setAttribute('stroke-width', '3');
  arc.setAttribute('stroke-linecap', 'round');
  arc.classList.add('k-arc');
  svg.appendChild(arc);

  // ── Knob body ──
  const rim = document.createElementNS(ns, 'circle');
  rim.setAttribute('r', R + 1.5);
  rim.setAttribute('fill', '#555');
  rim.setAttribute('filter', `url(#${gid}s)`);
  svg.appendChild(rim);

  const body = document.createElementNS(ns, 'circle');
  body.setAttribute('r', R);
  body.setAttribute('fill', `url(#${gid})`);
  svg.appendChild(body);

  // ── Indicator line (white, from center toward edge) ──
  const line = document.createElementNS(ns, 'line');
  line.setAttribute('x1', '0');
  line.setAttribute('y1', '0');
  line.setAttribute('stroke', '#f0f0f0');
  line.setAttribute('stroke-width', '2.5');
  line.setAttribute('stroke-linecap', 'round');
  line.classList.add('k-line');
  svg.appendChild(line);

  // ── Specular highlight ──
  const shine = document.createElementNS(ns, 'ellipse');
  shine.setAttribute('cx', '-7');
  shine.setAttribute('cy', '-9');
  shine.setAttribute('rx', '8');
  shine.setAttribute('ry', '5');
  shine.setAttribute('fill', 'rgba(255,255,255,0.12)');
  svg.appendChild(shine);

  updateKnobSVG(svg, dims[idx]);
  return svg;
}

function updateKnobSVG(svg, value) {
  const spanDeg = value * 270;
  const endDeg  = 135 + spanDeg;
  const [ex, ey] = ptOnArc(endDeg);
  const tx = (ex * 0.55).toFixed(2);
  const ty = (ey * 0.55).toFixed(2);

  svg.querySelector('.k-arc').setAttribute('d', arcPath(135, spanDeg));
  const line = svg.querySelector('.k-line');
  line.setAttribute('x2', tx);
  line.setAttribute('y2', ty);
}

function buildKnobGrid() {
  const grid = document.getElementById('knob-grid');

  dimensions.forEach((dim, idx) => {
    const cell = document.createElement('div');
    cell.className = 'knob-cell';
    cell.dataset.idx = idx;

    const svg = makeKnobSVG(idx);
    const label = document.createElement('div');
    label.className = 'knob-label';
    label.textContent = dim.name;

    const valDisplay = document.createElement('div');
    valDisplay.className = 'knob-value';
    valDisplay.id = `kv-${idx}`;
    valDisplay.textContent = '0.50';

    cell.appendChild(svg);
    cell.appendChild(label);
    cell.appendChild(valDisplay);
    grid.appendChild(cell);

    // Drag interaction (vertical drag: up = +, down = -)
    cell.addEventListener('mousedown', e => {
      const startY   = e.clientY;
      const startVal = dims[idx];
      cell.classList.add('dragging');

      const onMove = async e => {
        const delta = (startY - e.clientY) / 150;
        dims[idx] = Math.max(0, Math.min(1, startVal + delta));
        updateKnobDisplay(idx);
        await runInference();
      };

      const onUp = () => {
        cell.classList.remove('dragging');
        document.removeEventListener('mousemove', onMove);
        document.removeEventListener('mouseup', onUp);
      };

      document.addEventListener('mousemove', onMove);
      document.addEventListener('mouseup', onUp);
      e.preventDefault();
    });

    // Scroll wheel support
    cell.addEventListener('wheel', async e => {
      dims[idx] = Math.max(0, Math.min(1, dims[idx] - e.deltaY * 0.002));
      updateKnobDisplay(idx);
      await runInference();
      e.preventDefault();
    }, { passive: false });
  });
}

function updateKnobDisplay(idx) {
  const cell = document.querySelector(`.knob-cell[data-idx="${idx}"]`);
  if (!cell) return;
  const svg = cell.querySelector('svg');
  updateKnobSVG(svg, dims[idx]);
  document.getElementById(`kv-${idx}`).textContent = dims[idx].toFixed(2);
}

// ── RHYTHM ──────────────────────────────────────────────────
function stopRhythm() {
  rhythmTimers.forEach(clearTimeout);
  rhythmTimers = [];
  if (activePattern) {
    // All notes off on current channel
    if (midiOutput) midiOutput.send([0xB0 | midiChannel(), 123, 0]);
  }
  document.querySelectorAll('.btn-rhythm').forEach(b => b.classList.remove('active'));
  activePattern = null;
}

function playRhythm(name) {
  if (activePattern === name) { stopRhythm(); return; }
  stopRhythm();
  activePattern = name;
  document.querySelector(`.btn-rhythm[data-pattern="${name}"]`).classList.add('active');

  const hits = PATTERNS[name];
  hits.forEach(({ t, note }) => {
    rhythmTimers.push(setTimeout(() => {
      noteOn(note, 110);
      rhythmTimers.push(setTimeout(() => noteOff(note), 80));
    }, t));
  });

  // Auto-stop after last hit
  const duration = Math.max(...hits.map(h => h.t)) + 200;
  rhythmTimers.push(setTimeout(stopRhythm, duration));
}

// ── CHORDS ──────────────────────────────────────────────────
function stopChord() {
  if (chordTimer) { clearTimeout(chordTimer); chordTimer = null; }
  if (activeChord) {
    activeChord.forEach(n => noteOff(n));
    activeChord = null;
  }
  document.querySelectorAll('.btn-chord').forEach(b => b.classList.remove('playing'));
}

function playChord(btn, notes) {
  const alreadyPlaying = activeChord &&
    activeChord.length === notes.length &&
    activeChord.every((n, i) => n === notes[i]);

  stopChord();
  if (alreadyPlaying) return;

  activeChord = notes;
  btn.classList.add('playing');
  notes.forEach(n => noteOn(n, 90));
  chordTimer = setTimeout(stopChord, CHORD_DURATION);
}

// ── CONTROLS ────────────────────────────────────────────────
function bindControls() {
  document.getElementById('btn-connect').addEventListener('click', connectMIDI);
  document.getElementById('btn-send').addEventListener('click', sendPatch);

  document.querySelectorAll('.btn-rhythm').forEach(btn => {
    btn.addEventListener('click', () => playRhythm(btn.dataset.pattern));
  });

  document.querySelectorAll('.btn-chord').forEach(btn => {
    btn.addEventListener('click', () => {
      const notes = btn.dataset.notes.split(',').map(Number);
      playChord(btn, notes);
    });
  });
}

// ── BOOT ────────────────────────────────────────────────────
window.addEventListener('DOMContentLoaded', init);
