"use client"
import React, { useEffect, useState } from 'react';
import { AnimatePresence, motion, useMotionValueEvent, useScroll, useSpring } from 'framer-motion';
import { ButtonLabel, Wordmark, btn, ease } from './motion';
import { demoHref, loginPath } from './data';

const links = [["#funciones", "Funciones"], ["#especialidades", "Especialidades"], ["#precios", "Precios"], ["#preguntas", "Preguntas"]];

export default function Nav() {
  const { scrollY, scrollYProgress } = useScroll();
  const progress = useSpring(scrollYProgress, { stiffness: 140, damping: 30, mass: 0.3 });
  const [hidden, setHidden] = useState(false);
  const [solid, setSolid] = useState(false);
  const [open, setOpen] = useState(false);

  useMotionValueEvent(scrollY, "change", (y) => {
    const prev = scrollY.getPrevious() ?? 0;
    setSolid(y > 40);
    setHidden(y > 400 && y > prev && !open);
  });

  useEffect(() => {
    document.documentElement.style.overflow = open ? "hidden" : "";
    return () => { document.documentElement.style.overflow = ""; };
  }, [open]);

  return (
    <>
      <motion.div aria-hidden="true" className="fixed inset-x-0 top-0 z-[70] h-[2px] origin-left bg-signal" style={{ scaleX: progress }} />
      <motion.header
        className="fixed inset-x-0 top-0 z-50 px-3 pt-3 sm:px-5"
        animate={{ y: hidden ? -100 : 0 }}
        transition={{ duration: 0.5, ease }}
      >
        <nav
          className={`mx-auto flex h-14 max-w-6xl items-center justify-between rounded-full pl-4 pr-2 transition-[background-color,box-shadow,backdrop-filter] duration-500 ${
            solid ? "bg-paper/75 shadow-[0_1px_0_rgba(11,37,64,0.06),0_12px_32px_-12px_rgba(11,37,64,0.18)] ring-1 ring-ink/5 backdrop-blur-xl" : ""
          }`}
        >
          <a href="#inicio" aria-label="Caresia, inicio" className="rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-signal">
            <Wordmark />
          </a>
          <div className="hidden items-center md:flex">
            {links.map(([href, label]) => (
              <a key={href} href={href} className="group relative px-3.5 py-2 text-[15px] text-ink-soft transition-colors hover:text-ink">
                {label}
                <span className="absolute inset-x-3.5 bottom-1 h-px origin-right scale-x-0 bg-ink transition-transform duration-500 ease-out-strong group-hover:origin-left group-hover:scale-x-100" />
              </a>
            ))}
          </div>
          <div className="flex items-center gap-1.5">
            <a href={loginPath} className="hidden px-3 text-[15px] font-medium text-ink-soft hover:text-ink sm:block">Entrar</a>
            <a href={demoHref} className={`${btn} h-10 bg-ink px-5 text-sm text-paper hover:bg-signal`}>
              <ButtonLabel>Solicitar demo</ButtonLabel>
            </a>
            <button
              type="button"
              aria-label={open ? "Cerrar menú" : "Abrir menú"}
              aria-expanded={open}
              onClick={() => setOpen((o) => !o)}
              className="relative grid h-10 w-10 place-items-center rounded-full md:hidden"
            >
              <span className={`absolute h-[1.5px] w-5 bg-ink transition-transform duration-300 ${open ? "rotate-45" : "-translate-y-1"}`} />
              <span className={`absolute h-[1.5px] w-5 bg-ink transition-transform duration-300 ${open ? "-rotate-45" : "translate-y-1"}`} />
            </button>
          </div>
        </nav>
      </motion.header>

      <AnimatePresence>
        {open && (
          <motion.div
            className="fixed inset-0 z-40 flex flex-col justify-end bg-paper px-6 pb-12 pt-24 md:hidden"
            initial={{ clipPath: "inset(0 0 100% 0)" }}
            animate={{ clipPath: "inset(0 0 0% 0)" }}
            exit={{ clipPath: "inset(0 0 100% 0)" }}
            transition={{ duration: 0.7, ease }}
          >
            <ul className="space-y-1">
              {[...links, [loginPath, "Iniciar sesión"]].map(([href, label], i) => (
                <li key={href} className="overflow-hidden">
                  <motion.a
                    href={href}
                    onClick={() => setOpen(false)}
                    className="block font-display text-6xl leading-[1.1] text-ink"
                    initial={{ y: "100%" }}
                    animate={{ y: 0 }}
                    transition={{ duration: 0.8, delay: 0.15 + i * 0.06, ease }}
                  >
                    {label}
                  </motion.a>
                </li>
              ))}
            </ul>
            <p className="mt-10 font-mono text-xs uppercase tracking-[0.14em] text-ink-faint">Dental · Medicina general</p>
          </motion.div>
        )}
      </AnimatePresence>
    </>
  );
}
