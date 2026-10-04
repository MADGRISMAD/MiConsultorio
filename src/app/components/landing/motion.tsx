"use client"
import React, { useEffect, useRef } from 'react';
import { motion, useReducedMotion, useSpring } from 'framer-motion';
import Lenis from 'lenis';

export const ease = [0.22, 1, 0.36, 1] as const;

/** Inertial smooth scrolling for the landing. Off when the user prefers reduced motion. */
export function useSmoothScroll() {
  const reduce = useReducedMotion();
  useEffect(() => {
    if (reduce) return;
    const lenis = new Lenis({ duration: 1.15, anchors: { offset: -72 } });
    let id = 0;
    const raf = (t: number) => {
      lenis.raf(t);
      id = requestAnimationFrame(raf);
    };
    id = requestAnimationFrame(raf);
    return () => {
      cancelAnimationFrame(id);
      lenis.destroy();
    };
  }, [reduce]);
}

/**
 * Headline that rises in word by word from behind a mask.
 * Wrap words in *asterisks* to set them in the italic accent; "\n" breaks the line.
 */
export function Split({
  text,
  as: Tag = "h2",
  className = "",
  accent = "italic",
  delay = 0,
  stagger = 0.06,
}: {
  text: string;
  as?: "h1" | "h2" | "h3" | "p";
  className?: string;
  accent?: string;
  delay?: number;
  stagger?: number;
}) {
  const reduce = useReducedMotion();
  const lines = text.split("\n");
  let n = 0;
  return (
    <Tag className={className}>
      <span className="sr-only">{text.replace(/\*/g, "")}</span>
      <motion.span
        aria-hidden="true"
        className="block"
        initial="hidden"
        whileInView="show"
        viewport={{ once: true, margin: "0px 0px -10% 0px" }}
        transition={{ staggerChildren: stagger, delayChildren: delay }}
      >
        {lines.map((line, li) => (
          <span key={li} className="block">
            {line.split("*").map((seg, si) =>
              seg.split(" ").filter(Boolean).map((word) => {
                const i = n++;
                return (
                  <React.Fragment key={`${li}-${si}-${i}`}>
                    <span className="inline-block overflow-hidden pb-[0.12em] -mb-[0.12em] align-bottom">
                      <motion.span
                        className={`inline-block will-change-transform ${si % 2 ? accent : ""}`}
                        variants={{
                          hidden: reduce ? { opacity: 0 } : { y: "110%", rotate: 4 },
                          show: reduce ? { opacity: 1 } : { y: "0%", rotate: 0 },
                        }}
                        transition={{ duration: 1, ease }}
                      >
                        {word}
                      </motion.span>
                    </span>{" "}
                  </React.Fragment>
                );
              })
            )}
          </span>
        ))}
      </motion.span>
    </Tag>
  );
}

/** Fades and lifts its children once they scroll into view. */
export function Reveal({ children, delay = 0, y = 28, className = "" }: { children: React.ReactNode; delay?: number; y?: number; className?: string }) {
  const reduce = useReducedMotion();
  return (
    <motion.div
      className={className}
      initial={{ opacity: 0, y: reduce ? 0 : y }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: "0px 0px -8% 0px" }}
      transition={{ duration: 0.9, delay, ease }}
    >
      {children}
    </motion.div>
  );
}

/** Pulls its child toward a mouse pointer hovering over it. */
export function Magnetic({ children, strength = 0.3, className = "" }: { children: React.ReactNode; strength?: number; className?: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const reduce = useReducedMotion();
  const cfg = { stiffness: 220, damping: 16, mass: 0.5 };
  const x = useSpring(0, cfg);
  const y = useSpring(0, cfg);
  return (
    <motion.div
      ref={ref}
      className={`inline-flex ${className}`}
      style={{ x, y }}
      onPointerMove={(e) => {
        if (reduce || e.pointerType !== "mouse" || !ref.current) return;
        const r = ref.current.getBoundingClientRect();
        x.set((e.clientX - (r.left + r.width / 2)) * strength);
        y.set((e.clientY - (r.top + r.height / 2)) * strength);
      }}
      onPointerLeave={() => {
        x.set(0);
        y.set(0);
      }}
    >
      {children}
    </motion.div>
  );
}

function ecgPath(width: number, beat: number) {
  let d = "M0 60";
  for (let x = 0; x < width; x += beat) {
    d += ` L${x + 90} 60 Q${x + 105} 47 ${x + 120} 60 L${x + 140} 60 L${x + 148} 70 L${x + 158} 8 L${x + 168} 100 L${x + 177} 60 L${x + 200} 60 Q${x + 222} 38 ${x + 246} 60 L${x + beat} 60`;
  }
  return d;
}

/** Heart-monitor trace with a sweeping highlight. Decorative. */
export function Ecg({ className = "", base = "stroke-ink/10", sweep = "stroke-signal", beats = 4 }: { className?: string; base?: string; sweep?: string; beats?: number }) {
  const d = ecgPath(beats * 300, 300);
  return (
    <svg aria-hidden="true" viewBox={`0 0 ${beats * 300} 120`} preserveAspectRatio="none" className={className} fill="none">
      <path d={d} className={base} strokeWidth={1.5} vectorEffect="non-scaling-stroke" strokeLinejoin="round" />
      <path d={d} pathLength={1000} className={`${sweep} ecg-sweep`} strokeWidth={2.5} vectorEffect="non-scaling-stroke" strokeLinejoin="round" strokeLinecap="round" />
    </svg>
  );
}

export const Icon = ({ children, className = "h-5 w-5", strokeWidth = 1.75 }: { children: React.ReactNode; className?: string; strokeWidth?: number }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    {children}
  </svg>
);

export const check = <path d="M20 6 9 17l-5-5" />;
export const arrow = <path d="M5 12h14M13 6l6 6-6 6" />;

export const Mark = ({ className = "h-7 w-7", light = false }: { className?: string; light?: boolean }) => (
  <svg viewBox="0 0 32 32" aria-hidden="true" className={className}>
    <rect width="32" height="32" rx="9" className={light ? "fill-paper" : "fill-ink"} />
    <path d="M16 8v16M8 16h16" stroke={light ? "#14211D" : "#F4F1EA"} strokeWidth="3.2" strokeLinecap="round" />
    <circle cx="23.5" cy="8.5" r="2.5" className="fill-signal" />
  </svg>
);

export const Wordmark = ({ light = false }: { light?: boolean }) => (
  <span className={`inline-flex items-center gap-2.5 ${light ? "text-paper" : "text-ink"}`}>
    <Mark light={light} />
    <span className="font-display text-[26px] leading-none tracking-[-0.01em]">Caresia</span>
  </span>
);

/** Primary pill button. */
export const btn = "group relative inline-flex select-none touch-manipulation items-center justify-center gap-2 overflow-hidden rounded-full font-medium transition-[transform,background-color,color] duration-200 ease-out-strong active:scale-[0.97] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal focus-visible:ring-offset-2";

export function ButtonLabel({ children }: { children: React.ReactNode }) {
  // Label slides up and a copy slides in from below on hover
  return (
    <span className="relative block overflow-hidden">
      <span className="block transition-transform duration-500 ease-out-strong group-hover:-translate-y-full">{children}</span>
      <span aria-hidden="true" className="absolute inset-0 block translate-y-full transition-transform duration-500 ease-out-strong group-hover:translate-y-0">{children}</span>
    </span>
  );
}
