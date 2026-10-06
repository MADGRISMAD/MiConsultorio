// Small animation toolkit for the landing: scroll-linked progress with spring
// smoothing, one-shot reveal on scroll, and a magnetic pointer effect.

export const EASE = 'cubic-bezier(0.22, 1, 0.36, 1)';

/** Primary pill button classes. */
export const btn =
  'group relative inline-flex select-none touch-manipulation items-center justify-center gap-2 overflow-hidden rounded-full font-medium transition-[transform,background-color,color] duration-200 ease-out-strong active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal focus-visible:ring-offset-2';

export const reducedMotion = () => typeof matchMedia !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches;

export const clamp = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v));

/** Piecewise-linear map of `v` from `input` stops to `output` stops, clamped at both ends. */
export function interp(v: number, input: number[], output: number[]): number {
  if (v <= input[0]) return output[0];
  for (let i = 1; i < input.length; i++) {
    if (v <= input[i]) {
      const t = (v - input[i - 1]) / (input[i] - input[i - 1]);
      return output[i - 1] + (output[i] - output[i - 1]) * t;
    }
  }
  return output[output.length - 1];
}

// ---------------------------------------------------------------------------
// One shared requestAnimationFrame loop. Callbacks receive the frame delta (s).
// ---------------------------------------------------------------------------
type Tick = (dt: number) => void;
const ticks = new Set<Tick>();
let raf = 0;
let last = 0;

function frame(now: number) {
  const dt = Math.min((now - last) / 1000, 0.05);
  last = now;
  for (const t of [...ticks]) t(dt);
  raf = ticks.size ? requestAnimationFrame(frame) : 0;
}

/** Runs `fn` every animation frame until the returned function is called. */
export function onFrame(fn: Tick): () => void {
  ticks.add(fn);
  if (!raf) {
    last = performance.now();
    raf = requestAnimationFrame(frame);
  }
  return () => ticks.delete(fn);
}

// ---------------------------------------------------------------------------
// Springs
// ---------------------------------------------------------------------------
export interface SpringConfig {
  stiffness: number;
  damping: number;
  mass?: number;
}

export class Spring {
  value: number;
  velocity = 0;
  constructor(
    initial: number,
    private cfg: SpringConfig
  ) {
    this.value = initial;
  }
  step(target: number, dt: number) {
    const { stiffness, damping, mass = 1 } = this.cfg;
    const sub = 1 / 240; // fixed sub-steps keep stiff springs stable
    for (let t = dt; t > 0; t -= sub) {
      const h = Math.min(sub, t);
      const a = (stiffness * (target - this.value) - damping * this.velocity) / mass;
      this.velocity += a * h;
      this.value += this.velocity * h;
    }
    return this.value;
  }
  get settled() {
    return Math.abs(this.velocity) < 0.0005;
  }
}

// ---------------------------------------------------------------------------
// Scroll progress
// ---------------------------------------------------------------------------
function edge(e: string): number {
  if (e === 'start') return 0;
  if (e === 'center') return 0.5;
  if (e === 'end') return 1;
  return parseFloat(e) / 100;
}

/**
 * Progress (0..1) of an element through the viewport. Offsets read like
 * framer-motion's: "<element edge> <viewport edge>", e.g. ["start end", "end start"]
 * runs from the element's top touching the viewport's bottom until its bottom leaves the top.
 */
export function scrollProgress(el: Element, offset: [string, string]): number {
  const rect = el.getBoundingClientRect();
  const vh = window.innerHeight;
  const [[es0, vs0], [es1, vs1]] = offset.map((o) => o.split(' ')) as [[string, string], [string, string]];
  // scroll distance for which the element point sits at the viewport point
  const at = (es: string, vs: string) => rect.top + scrollY + edge(es) * rect.height - edge(vs) * vh;
  const from = at(es0, vs0);
  const to = at(es1, vs1);
  return to === from ? 0 : clamp((scrollY - from) / (to - from), 0, 1);
}

/** Whole-page scroll progress, 0..1. */
export function pageProgress(): number {
  const max = document.documentElement.scrollHeight - window.innerHeight;
  return max <= 0 ? 0 : clamp(scrollY / max, 0, 1);
}

/**
 * Calls `cb` every frame with the (optionally spring-smoothed) progress of `el`.
 * Stops painting once the value settles. With reduced motion the value jumps instead of easing.
 */
export function track(el: Element, offset: [string, string], cb: (p: number) => void, spring?: SpringConfig): () => void {
  const sp = spring && !reducedMotion() ? new Spring(scrollProgress(el, offset), spring) : null;
  let prev = NaN;
  cb(scrollProgress(el, offset));
  return onFrame((dt) => {
    const target = scrollProgress(el, offset);
    const v = sp ? sp.step(target, dt) : target;
    if (v === prev && (!sp || sp.settled)) return;
    prev = v;
    cb(v);
  });
}

// ---------------------------------------------------------------------------
// Actions
// ---------------------------------------------------------------------------

/** Fires `cb(true)` once when the node scrolls into view. */
export function observeOnce(node: Element, cb: () => void, rootMargin = '0px 0px -10% 0px'): () => void {
  if (typeof IntersectionObserver === 'undefined') {
    cb();
    return () => {};
  }
  const io = new IntersectionObserver(
    (entries) => {
      if (entries.some((e) => e.isIntersecting)) {
        cb();
        io.disconnect();
      }
    },
    { rootMargin }
  );
  io.observe(node);
  return () => io.disconnect();
}

/** Svelte action: fade and lift the node once it scrolls into view. */
export function reveal(node: HTMLElement, opts: { delay?: number; y?: number; duration?: number; margin?: string } = {}) {
  const { delay = 0, y = 28, duration = 0.9, margin = '0px 0px -8% 0px' } = opts;
  const reduce = reducedMotion();
  node.style.opacity = '0';
  node.style.transform = reduce ? '' : `translateY(${y}px)`;
  node.style.willChange = 'opacity, transform';
  const stop = observeOnce(
    node,
    () => {
      node.style.transition = `opacity ${duration}s ${EASE} ${delay}s, transform ${duration}s ${EASE} ${delay}s`;
      node.style.opacity = '1';
      node.style.transform = 'none';
    },
    margin
  );
  return { destroy: stop };
}

/** Svelte action: pull the node toward a hovering mouse pointer. */
export function magnetic(node: HTMLElement, strength = 0.3) {
  const x = new Spring(0, { stiffness: 220, damping: 16, mass: 0.5 });
  const y = new Spring(0, { stiffness: 220, damping: 16, mass: 0.5 });
  let tx = 0;
  let ty = 0;
  const reduce = reducedMotion();
  const stop = onFrame((dt) => {
    const px = x.step(tx, dt);
    const py = y.step(ty, dt);
    node.style.transform = Math.abs(px) + Math.abs(py) < 0.01 && tx === 0 && ty === 0 ? '' : `translate3d(${px}px, ${py}px, 0)`;
  });
  const move = (e: PointerEvent) => {
    if (reduce || e.pointerType !== 'mouse') return;
    const r = node.getBoundingClientRect();
    tx = (e.clientX - (r.left + r.width / 2)) * strength;
    ty = (e.clientY - (r.top + r.height / 2)) * strength;
  };
  const leave = () => {
    tx = 0;
    ty = 0;
  };
  node.addEventListener('pointermove', move);
  node.addEventListener('pointerleave', leave);
  return {
    destroy() {
      stop();
      node.removeEventListener('pointermove', move);
      node.removeEventListener('pointerleave', leave);
    }
  };
}

/** Svelte action: run `cb(inView)` while the node is within `margin` of the viewport centre band. */
export function whileInView(node: Element, opts: { margin: string; cb: (inView: boolean) => void }) {
  const io = new IntersectionObserver((entries) => entries.forEach((e) => opts.cb(e.isIntersecting)), { rootMargin: opts.margin });
  io.observe(node);
  return { destroy: () => io.disconnect() };
}
