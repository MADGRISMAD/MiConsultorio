// Records a dictation and turns it into a 16 kHz mono WAV (what the model reads reliably). Nothing is stored: the
// audio lives in memory until it is sent for the draft of the note.

export const MAX_DICTATION_SECONDS = 180;

export function micAvailable(): boolean {
  return typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia && typeof MediaRecorder !== 'undefined';
}

export class Recorder {
  #stream: MediaStream | null = null;
  #rec: MediaRecorder | null = null;
  #chunks: Blob[] = [];
  #started = 0;

  async start(): Promise<void> {
    this.#stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    this.#chunks = [];
    this.#rec = new MediaRecorder(this.#stream);
    this.#rec.ondataavailable = (e) => e.data.size && this.#chunks.push(e.data);
    this.#rec.start();
    this.#started = Date.now();
  }

  get seconds(): number {
    return this.#rec ? Math.round((Date.now() - this.#started) / 1000) : 0;
  }

  #release() {
    this.#stream?.getTracks().forEach((t) => t.stop());
    this.#stream = null;
  }

  /** Stops, and gives the WAV as base64. */
  stop(): Promise<{ base64: string; seconds: number }> {
    const rec = this.#rec;
    if (!rec) return Promise.reject(new Error('No hay una grabación en curso.'));
    const seconds = this.seconds;
    return new Promise((resolve, reject) => {
      rec.onstop = async () => {
        this.#release();
        try {
          const blob = new Blob(this.#chunks, { type: rec.mimeType || 'audio/webm' });
          resolve({ base64: toBase64(await toWav(await blob.arrayBuffer())), seconds });
        } catch {
          reject(new Error('No se pudo procesar el audio. Intenta de nuevo.'));
        }
      };
      rec.stop();
    });
  }

  cancel() {
    if (this.#rec && this.#rec.state !== 'inactive') {
      this.#rec.onstop = null;
      this.#rec.stop();
    }
    this.#release();
    this.#rec = null;
  }
}

async function toWav(data: ArrayBuffer): Promise<Uint8Array> {
  const ctx = new AudioContext();
  const decoded = await ctx.decodeAudioData(data);
  await ctx.close();
  const rate = 16000;
  const off = new OfflineAudioContext(1, Math.max(1, Math.ceil(decoded.duration * rate)), rate);
  const src = off.createBufferSource();
  src.buffer = decoded;
  src.connect(off.destination);
  src.start();
  const pcm = (await off.startRendering()).getChannelData(0);
  const out = new Uint8Array(44 + pcm.length * 2);
  const v = new DataView(out.buffer);
  const str = (o: number, s: string) => [...s].forEach((c, i) => v.setUint8(o + i, c.charCodeAt(0)));
  str(0, 'RIFF');
  v.setUint32(4, 36 + pcm.length * 2, true);
  str(8, 'WAVE');
  str(12, 'fmt ');
  v.setUint32(16, 16, true);
  v.setUint16(20, 1, true); // PCM
  v.setUint16(22, 1, true); // mono
  v.setUint32(24, rate, true);
  v.setUint32(28, rate * 2, true);
  v.setUint16(32, 2, true);
  v.setUint16(34, 16, true);
  str(36, 'data');
  v.setUint32(40, pcm.length * 2, true);
  for (let i = 0; i < pcm.length; i++) {
    const s = Math.max(-1, Math.min(1, pcm[i]));
    v.setInt16(44 + i * 2, s < 0 ? s * 0x8000 : s * 0x7fff, true);
  }
  return out;
}

function toBase64(bytes: Uint8Array): string {
  let bin = '';
  for (let i = 0; i < bytes.length; i += 0x8000) bin += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(bin);
}
