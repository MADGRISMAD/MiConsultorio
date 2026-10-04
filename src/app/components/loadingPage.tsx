import React from 'react';

const loadingPage: React.FC= ({}) => {

  return (
    <div role="status" className="grid min-h-[100svh] place-items-center">
        {/* Fades in only after 400ms, so fast session checks don't flash a spinner */}
        <span className="[animation:fade_200ms_400ms_both]">
            <span className="block h-5 w-5 animate-[spin_0.6s_linear_infinite] rounded-full border-2 border-slate-200 border-t-slate-700" />
        </span>
        <span className="sr-only">Cargando…</span>
    </div>
    );
};

export default loadingPage;
