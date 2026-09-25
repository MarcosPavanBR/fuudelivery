// Alerta de pedido novo no painel da loja: som (gerado com Web Audio, sem
// arquivo) e o título da aba piscando até a janela voltar ao foco. Numa
// cozinha barulhenta, pedido em análise sem alerta fica esquecido.

const SOUND_KEY = "fuu:new-order-sound";

export function isSoundOn() {
  try {
    return localStorage.getItem(SOUND_KEY) !== "off";
  } catch {
    return true;
  }
}

export function setSoundOn(on) {
  try {
    localStorage.setItem(SOUND_KEY, on ? "on" : "off");
  } catch {
    // Sem storage (aba anônima): vale só nesta sessão.
  }
}

let audioCtx = null;

// O navegador só libera áudio depois de um clique na página; chamar isto em
// qualquer interação deixa o som pronto para o próximo pedido.
export function unlockAudio() {
  try {
    const Ctx = window.AudioContext || window.webkitAudioContext;
    if (!Ctx) return;
    if (!audioCtx) audioCtx = new Ctx();
    if (audioCtx.state === "suspended") audioCtx.resume();
  } catch {
    // Sem Web Audio: fica só o título piscando.
  }
}

function beep() {
  if (!audioCtx || audioCtx.state !== "running") return;
  const now = audioCtx.currentTime;
  // Dois toques curtos, agudos o bastante para atravessar barulho de cozinha.
  [0, 0.28].forEach((offset) => {
    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();
    osc.type = "sine";
    osc.frequency.value = 880;
    gain.gain.setValueAtTime(0.0001, now + offset);
    gain.gain.exponentialRampToValueAtTime(0.4, now + offset + 0.02);
    gain.gain.exponentialRampToValueAtTime(0.0001, now + offset + 0.22);
    osc.connect(gain).connect(audioCtx.destination);
    osc.start(now + offset);
    osc.stop(now + offset + 0.25);
  });
}

let flashTimer = null;
let originalTitle = null;

function flashTitle(count) {
  if (typeof document === "undefined") return;
  if (originalTitle === null) originalTitle = document.title;
  clearInterval(flashTimer);
  const alertTitle = count > 1 ? `(${count}) Novos pedidos!` : "(1) Novo pedido!";
  let on = false;
  flashTimer = setInterval(() => {
    on = !on;
    document.title = on ? alertTitle : originalTitle;
  }, 1000);
  const stop = () => {
    clearInterval(flashTimer);
    flashTimer = null;
    document.title = originalTitle;
    window.removeEventListener("focus", stop);
  };
  if (document.hasFocus()) setTimeout(stop, 6000);
  else window.addEventListener("focus", stop);
}

export function alertNewOrders(count) {
  if (count <= 0) return;
  if (isSoundOn()) beep();
  flashTitle(count);
}
