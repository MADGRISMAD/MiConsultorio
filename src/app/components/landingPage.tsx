"use client"
import React from 'react';
import Nav from './landing/Nav';
import Hero from './landing/Hero';
import Chaos from './landing/Chaos';
import Features from './landing/Features';
import { Marquee, Specialties } from './landing/Specialties';
import { Cta, Faq, Footer, Pricing, Steps } from './landing/Closing';
import { useSmoothScroll } from './landing/motion';

const LandingPage: React.FC = () => {
  useSmoothScroll();
  return (
    <div className="grain overflow-x-clip bg-paper font-body text-ink antialiased selection:bg-signal selection:text-white [-webkit-tap-highlight-color:transparent]">
      <Nav />
      <Hero />
      <Chaos />
      <Features />
      <Marquee />
      <Specialties />
      <Steps />
      <div className="relative z-10 -mt-9 rounded-t-[36px] bg-paper">
        <Pricing />
        <Faq />
      </div>
      <Cta />
      <Footer />
    </div>
  );
};

export default LandingPage;
