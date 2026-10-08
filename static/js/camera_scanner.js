(() => {
    const GAME_ID_REGEX = /(?:[?&]id=(\d+)|(?:^|\/|#)(\d+)\/?(?:[?#]|$)|(?:id[:\s#]+)(\d+)\b|\b(\d+)\b)/i;

    function extractGameID(raw) {
        if (raw === null || raw === undefined) return '';
        const text = String(raw).trim();
        if (!text) return '';

        const match = text.match(GAME_ID_REGEX);
        if (match) {
            for (let index = 1; index < match.length; index++) {
                if (match[index]) return match[index];
            }
        }

        return text.replace(/\D/g, '') || text;
    }

    window.extractGameID = extractGameID;

    let activeCallback = null;
    let scanner = null;
    let cameras = [];
    let currentCameraIndex = 0;
    let sessionID = 0;
    let scanHandled = false;

    function playScanBeep() {
        try {
            const AudioContext = window.AudioContext || window.webkitAudioContext;
            if (!AudioContext) return;
            const context = new AudioContext();
            const oscillator = context.createOscillator();
            const gain = context.createGain();
            oscillator.frequency.setValueAtTime(880, context.currentTime);
            gain.gain.setValueAtTime(0.12, context.currentTime);
            gain.gain.exponentialRampToValueAtTime(0.001, context.currentTime + 0.1);
            oscillator.connect(gain);
            gain.connect(context.destination);
            oscillator.start();
            oscillator.stop(context.currentTime + 0.1);
            oscillator.addEventListener('ended', () => context.close());
        } catch (_) {
            // Audio confirmation is optional.
        }
    }

    function showError(message) {
        const error = document.getElementById('scanner-error');
        if (!error) return;
        error.textContent = message;
        error.classList.remove('hidden');
    }

    function clearError() {
        document.getElementById('scanner-error')?.classList.add('hidden');
    }

    function createScannerModal() {
        if (document.getElementById('camera-scanner-modal')) return;

        const modal = document.createElement('div');
        modal.id = 'camera-scanner-modal';
        modal.className = 'fixed inset-0 z-[200] hidden flex-col items-center justify-center bg-black/85 p-3 sm:p-6 backdrop-blur-md';
        modal.innerHTML = `
            <div class="relative flex w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-theme-border bg-theme-card shadow-2xl">
                <div class="flex items-center justify-between border-b border-theme-border bg-theme-bg px-4 py-3">
                    <h3 class="text-sm font-bold text-white">Kamera-Scanner</h3>
                    <div class="flex items-center gap-2">
                        <button type="button" id="scanner-switch-btn" title="Kamera wechseln" class="hidden rounded-lg border border-theme-border bg-theme-card p-1.5 text-gray-300 hover:bg-theme-bg">
                            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                        </button>
                        <button type="button" id="scanner-close-btn" class="rounded-lg p-1.5 text-gray-400 hover:bg-theme-bg hover:text-white" aria-label="Schließen">
                            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
                        </button>
                    </div>
                </div>
                <div class="relative min-h-[260px] bg-black">
                    <div id="scanner-reader" class="min-h-[260px]"></div>
                    <div id="scanner-error" class="absolute inset-x-4 top-4 hidden rounded-xl border border-red-800 bg-red-950/90 p-3 text-center text-xs text-red-200"></div>
                </div>
                <div class="flex flex-col gap-2 border-t border-theme-border bg-theme-bg p-3.5">
                    <p class="text-center text-xs text-theme-muted">Code in den markierten Bereich halten</p>
                    <form id="scanner-manual-form" class="mt-1 flex gap-2">
                        <input type="text" id="scanner-manual-input" placeholder="Oder Code manuell eingeben …" class="min-w-0 flex-1 rounded-lg border border-theme-border bg-theme-card px-3 py-2 text-xs text-white placeholder:text-theme-muted outline-none focus:border-blue-500">
                        <button type="submit" class="rounded-lg bg-blue-600 px-3 py-2 text-xs font-semibold text-white hover:bg-blue-500">Übernehmen</button>
                    </form>
                </div>
            </div>
        `;

        document.body.appendChild(modal);
        document.getElementById('scanner-close-btn').addEventListener('click', closeCameraScanner);
        document.getElementById('scanner-switch-btn').addEventListener('click', async () => {
            if (cameras.length < 2) return;
            currentCameraIndex = (currentCameraIndex + 1) % cameras.length;
            await startScanner(sessionID, cameras[currentCameraIndex].id);
        });
        document.getElementById('scanner-manual-form').addEventListener('submit', (event) => {
            event.preventDefault();
            const value = document.getElementById('scanner-manual-input').value.trim();
            if (value) handleScan(value);
        });
        modal.addEventListener('click', (event) => {
            if (event.target === modal) closeCameraScanner();
        });
        document.addEventListener('keydown', (event) => {
            if (event.key === 'Escape' && !modal.classList.contains('hidden')) closeCameraScanner();
        });
    }

    async function stopAndClear(currentScanner) {
        try {
            await currentScanner.stop();
        } catch (error) {
            console.warn('Kamera-Stream konnte nicht sauber beendet werden:', error);
        }
        try {
            await currentScanner.clear();
        } catch (error) {
            console.warn('Kamera-Ansicht konnte nicht bereinigt werden:', error);
        }
    }

    async function stopScanner() {
        const currentScanner = scanner;
        scanner = null;
        if (currentScanner) await stopAndClear(currentScanner);
    }

    function supportedFormats() {
        const formats = window.Html5QrcodeSupportedFormats;
        if (!formats) return undefined;
        return [
            formats.QR_CODE,
            formats.CODE_128,
            formats.CODE_39,
            formats.EAN_13,
            formats.EAN_8,
            formats.UPC_A,
            formats.UPC_E,
            formats.ITF,
        ].filter(format => format !== undefined);
    }

    async function startScanner(expectedSessionID, cameraID = null) {
        await stopScanner();
        if (expectedSessionID !== sessionID) return;
        clearError();

        if (!window.Html5Qrcode) {
            showError('Der Scanner konnte nicht geladen werden. Bitte die Seite neu laden oder den Code manuell eingeben.');
            return;
        }
        if (!navigator.mediaDevices?.getUserMedia) {
            showError('Der Browser erlaubt keinen Kamerazugriff. Bitte die Seite über HTTPS öffnen oder den Code manuell eingeben.');
            return;
        }

        try {
            cameras = await window.Html5Qrcode.getCameras();
            if (expectedSessionID !== sessionID) return;
            if (cameras.length === 0) {
                showError('Es wurde keine Kamera gefunden. Bitte eine Kamera anschließen oder den Code manuell eingeben.');
                return;
            }
            const switchButton = document.getElementById('scanner-switch-btn');
            switchButton?.classList.toggle('hidden', cameras.length < 2);

            const selectedCamera = cameraID || cameras.find(camera => /back|rear|environment/i.test(camera.label))?.id || cameras[0]?.id;
            currentCameraIndex = Math.max(0, cameras.findIndex(camera => camera.id === selectedCamera));
            const newScanner = new window.Html5Qrcode('scanner-reader', {
                formatsToSupport: supportedFormats(),
                verbose: false,
            });
            scanner = newScanner;
            await newScanner.start(
                selectedCamera ? { deviceId: { exact: selectedCamera } } : { facingMode: { ideal: 'environment' } },
                { fps: 10 },
                handleScan,
                () => {}
            );
            if (expectedSessionID !== sessionID || scanner !== newScanner) {
                if (scanner === newScanner) scanner = null;
                await stopAndClear(newScanner);
                return;
            }
        } catch (error) {
            if (expectedSessionID !== sessionID) return;
            console.error('Kamerafehler:', error);
            await stopScanner();
            const detail = error instanceof Error ? ` (${error.message})` : '';
            showError(`Kamera konnte nicht gestartet werden. Bitte Kamerazugriff erlauben oder den Code manuell eingeben.${detail}`);
        }
    }

    function handleScan(decodedText) {
        if (scanHandled) return;
        const gameID = extractGameID(decodedText);
        if (!gameID) return;

        scanHandled = true;
        playScanBeep();
        const callback = activeCallback;
        closeCameraScanner();
        callback?.(gameID, decodedText);
    }

    function openCameraScanner(callback) {
        createScannerModal();
        activeCallback = callback;
        scanHandled = false;
        sessionID++;
        const modal = document.getElementById('camera-scanner-modal');
        document.getElementById('scanner-manual-input').value = '';
        modal.classList.remove('hidden');
        modal.classList.add('flex');
        startScanner(sessionID);
    }

    function closeCameraScanner() {
        sessionID++;
        stopScanner();
        const modal = document.getElementById('camera-scanner-modal');
        modal?.classList.add('hidden');
        modal?.classList.remove('flex');
        activeCallback = null;
    }

    window.openCameraScanner = openCameraScanner;
    window.closeCameraScanner = closeCameraScanner;
})();
