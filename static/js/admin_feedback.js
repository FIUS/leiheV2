(() => {
    if (window.__adminFeedbackInitialized) return;
    window.__adminFeedbackInitialized = true;

    const scrollKey = 'admin-action-scroll-position';

    const getScrollState = () => {
        const main = document.querySelector('main.overflow-y-auto') || document.querySelector('main');
        return JSON.stringify({
            windowY: window.scrollY || document.documentElement.scrollTop || 0,
            mainY: main ? main.scrollTop : 0
        });
    };

    const restoreScrollState = (saved) => {
        if (!saved) return;
        let windowY = 0;
        let mainY = 0;
        try {
            const parsed = JSON.parse(saved);
            if (parsed && typeof parsed === 'object') {
                windowY = Number(parsed.windowY) || 0;
                mainY = Number(parsed.mainY) || 0;
            } else {
                windowY = Number(saved) || 0;
                mainY = windowY;
            }
        } catch {
            windowY = Number(saved) || 0;
            mainY = windowY;
        }

        window.scrollTo(0, windowY);
        const main = document.querySelector('main.overflow-y-auto') || document.querySelector('main');
        if (main && mainY > 0) {
            main.scrollTop = mainY;
        }
    };

    const savedScroll = sessionStorage.getItem(scrollKey);
    if (savedScroll !== null) {
        sessionStorage.removeItem(scrollKey);
        const onReady = () => {
            restoreScrollState(savedScroll);
            requestAnimationFrame(() => restoreScrollState(savedScroll));
            setTimeout(() => restoreScrollState(savedScroll), 60);
        };
        if (document.readyState === 'loading') {
            window.addEventListener('DOMContentLoaded', onReady, { once: true });
        } else {
            onReady();
        }
    }

    document.addEventListener('submit', event => {
        const form = event.target;
        if (form instanceof HTMLFormElement && (form.method || 'get').toLowerCase() === 'post') {
            sessionStorage.setItem(scrollKey, getScrollState());
        }
    }, true);

    const showToast = (message, type = 'success') => {
        if (!message) return;
        const isError = type === 'error';
        const toast = document.createElement('div');
        toast.setAttribute('role', isError ? 'alert' : 'status');
        toast.className = `fixed right-4 top-4 z-[200] flex max-w-md items-center gap-3 rounded-xl border px-4 py-3 text-sm font-medium shadow-2xl backdrop-blur transition-all duration-300 transform translate-y-0 opacity-100 cursor-pointer ${
            isError
                ? 'border-red-500/50 bg-[#1F0D10]/95 text-red-100 shadow-red-950/40'
                : 'border-emerald-500/50 bg-[#0B1A14]/95 text-emerald-100 shadow-emerald-950/40'
        }`;

        const iconSvg = isError
            ? `<svg class="w-5 h-5 shrink-0 text-red-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>`
            : `<svg class="w-5 h-5 shrink-0 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>`;

        toast.innerHTML = `
            ${iconSvg}
            <span class="flex-1">${message.replace(/</g, '&lt;').replace(/>/g, '&gt;')}</span>
            <button type="button" aria-label="Schließen" class="ml-2 text-gray-400 hover:text-white transition">
                <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
            </button>
        `;

        const dismiss = () => {
            toast.style.opacity = '0';
            toast.style.transform = 'translateY(-8px)';
            setTimeout(() => toast.remove(), 300);
        };

        toast.addEventListener('click', dismiss);
        document.body.appendChild(toast);
        setTimeout(dismiss, 4500);
    };

    window.showAdminNotice = showToast;

    const params = new URLSearchParams(window.location.search);
    const message = params.get('notice');
    if (message) {
        const type = params.get('notice_type') || 'success';
        if (document.readyState === 'loading') {
            window.addEventListener('DOMContentLoaded', () => showToast(message, type), { once: true });
        } else {
            showToast(message, type);
        }

        params.delete('notice');
        params.delete('notice_type');
        const query = params.toString();
        history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`);
    }
})();
