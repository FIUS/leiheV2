(() => {
    // Shared Regular Expression to extract Game ID from raw text, URLs (e.g. verleih.fius.de/1 -> 1), hashes (#1 -> 1), etc.
    const GAME_ID_REGEX = /(?:[?&]id=(\d+)|(?:^|\/|#)(\d+)\/?(?:[?#]|$)|(?:id[:\s#]+)(\d+)\b|\b(\d+)\b)/i;

    function extractGameID(raw) {
        if (raw === null || raw === undefined) return '';
        const text = String(raw).trim();
        if (!text) return '';

        const match = text.match(GAME_ID_REGEX);
        if (match) {
            for (let i = 1; i < match.length; i++) {
                if (match[i]) return match[i];
            }
        }

        // Fallback: extract continuous digits
        const digits = text.replace(/\D/g, '');
        return digits || text;
    }

    // Expose globally
    window.extractGameID = extractGameID;

    // Web Audio API beep sound for scan confirmation
    function playScanBeep() {
        try {
            const AudioContext = window.AudioContext || window.webkitAudioContext;
            if (!AudioContext) return;
            const ctx = new AudioContext();
            const osc = ctx.createOscillator();
            const gain = ctx.createGain();
            osc.type = 'sine';
            osc.frequency.setValueAtTime(880, ctx.currentTime); // 880Hz A5 note
            gain.gain.setValueAtTime(0.12, ctx.currentTime);
            gain.gain.exponentialRampToValueAtTime(0.001, ctx.currentTime + 0.1);
            osc.connect(gain);
            gain.connect(ctx.destination);
            osc.start();
            osc.stop(ctx.currentTime + 0.1);
        } catch (_) {}
    }

    let activeStream = null;
    let scanAnimationFrame = null;
    let activeCallback = null;
    let barcodeDetector = null;
    let videoDevices = [];
    let currentDeviceIndex = 0;
    let torchTrack = null;
    let torchOn = false;

    // Check BarcodeDetector support
    if ('BarcodeDetector' in window) {
        try {
            barcodeDetector = new window.BarcodeDetector({
                formats: ['qr_code', 'code_128', 'code_39', 'ean_13', 'ean_8', 'upc_a', 'upc_e', 'data_matrix', 'itf']
            });
        } catch (_) {
            barcodeDetector = null;
        }
    }

    function createScannerModal() {
        if (document.getElementById('camera-scanner-modal')) return;

        const modal = document.createElement('div');
        modal.id = 'camera-scanner-modal';
        modal.className = 'fixed inset-0 z-[200] hidden flex-col items-center justify-center bg-black/85 p-3 sm:p-6 backdrop-blur-md transition-opacity duration-200';
        modal.innerHTML = `
            <div class="relative flex w-full max-w-lg flex-col overflow-hidden rounded-2xl border border-theme-border bg-theme-card shadow-2xl">
                <!-- Header -->
                <div class="flex items-center justify-between border-b border-theme-border bg-theme-bg px-4 py-3">
                    <div class="flex items-center gap-2">
                        <svg class="h-5 w-5 text-blue-400 animate-pulse" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 9a2 2 0 012-2h.93a2 2 0 001.664-.89l.812-1.22A2 2 0 0110.07 4h3.86a2 2 0 011.664.89l.812 1.22A2 2 0 0018.07 7H19a2 2 0 012 2v9a2 2 0 01-2 2H5a2 2 0 01-2-2V9z" />
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 13a3 3 0 11-6 0 3 3 0 016 0z" />
                        </svg>
                        <h3 class="text-sm font-bold text-white">Kamera-Scanner</h3>
                    </div>
                    <div class="flex items-center gap-2">
                        <button type="button" id="scanner-torch-btn" title="Licht an/aus" class="hidden rounded-lg border border-theme-border bg-theme-card p-1.5 text-yellow-300 hover:bg-theme-bg">
                            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                            </svg>
                        </button>
                        <button type="button" id="scanner-switch-btn" title="Kamera wechseln" class="hidden rounded-lg border border-theme-border bg-theme-card p-1.5 text-gray-300 hover:bg-theme-bg">
                            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                            </svg>
                        </button>
                        <button type="button" id="scanner-close-btn" class="rounded-lg p-1.5 text-gray-400 hover:bg-theme-bg hover:text-white" aria-label="Schließen">
                            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                            </svg>
                        </button>
                    </div>
                </div>

                <!-- Video Viewport -->
                <div class="relative flex min-h-[260px] max-h-[60vh] w-full items-center justify-center overflow-hidden bg-black">
                    <video id="scanner-video" playsinline autoplay muted class="h-full w-full object-cover"></video>
                    <canvas id="scanner-canvas" class="hidden"></canvas>

                    <!-- Scan Targeting Frame -->
                    <div id="scanner-target-box" class="pointer-events-none absolute h-44 w-44 sm:h-56 sm:w-56 rounded-2xl border-2 border-blue-400/70 shadow-[0_0_0_9999px_rgba(0,0,0,0.45)] transition-colors duration-200">
                        <div class="absolute -top-1 -left-1 h-4 w-4 border-t-4 border-l-4 border-blue-400 rounded-tl"></div>
                        <div class="absolute -top-1 -right-1 h-4 w-4 border-t-4 border-r-4 border-blue-400 rounded-tr"></div>
                        <div class="absolute -bottom-1 -left-1 h-4 w-4 border-b-4 border-l-4 border-blue-400 rounded-bl"></div>
                        <div class="absolute -bottom-1 -right-1 h-4 w-4 border-b-4 border-r-4 border-blue-400 rounded-br"></div>
                        <!-- Animated scan laser -->
                        <div class="absolute left-2 right-2 top-0 h-0.5 bg-gradient-to-r from-transparent via-red-500 to-transparent animate-pulse shadow-[0_0_8px_#ef4444]"></div>
                    </div>

                    <!-- Error / Status Notice -->
                    <div id="scanner-error" class="absolute inset-x-4 top-4 hidden rounded-xl border border-red-800 bg-red-950/90 p-3 text-center text-xs text-red-200"></div>
                </div>

                <!-- Bottom Helper / Manual Entry -->
                <div class="flex flex-col gap-2 border-t border-theme-border bg-theme-bg p-3.5">
                    <p class="text-center text-xs text-theme-muted">Barcode oder QR-Code (z. B. URL oder ID) vor die Kamera halten</p>
                    <form id="scanner-manual-form" class="mt-1 flex gap-2">
                        <input type="text" id="scanner-manual-input" placeholder="Oder Code manuell eingeben …" class="min-w-0 flex-1 rounded-lg border border-theme-border bg-theme-card px-3 py-2 text-xs text-white placeholder:text-theme-muted outline-none focus:border-blue-500">
                        <button type="submit" class="rounded-lg bg-blue-600 px-3 py-2 text-xs font-semibold text-white hover:bg-blue-500">Übernehmen</button>
                    </form>
                </div>
            </div>
        `;

        document.body.appendChild(modal);

        document.getElementById('scanner-close-btn').addEventListener('click', closeCameraScanner);
        modal.addEventListener('click', (e) => {
            if (e.target === modal) closeCameraScanner();
        });
        document.addEventListener('keydown', (e) => {
            if (e.key === 'Escape' && !modal.classList.contains('hidden')) {
                closeCameraScanner();
            }
        });

        document.getElementById('scanner-switch-btn').addEventListener('click', () => {
            if (videoDevices.length > 1) {
                currentDeviceIndex = (currentDeviceIndex + 1) % videoDevices.length;
                startStream(videoDevices[currentDeviceIndex].deviceId);
            }
        });

        document.getElementById('scanner-torch-btn').addEventListener('click', async () => {
            if (!torchTrack) return;
            try {
                torchOn = !torchOn;
                await torchTrack.applyConstraints({ advanced: [{ torch: torchOn }] });
                const btn = document.getElementById('scanner-torch-btn');
                if (btn) btn.classList.toggle('bg-yellow-500/20', torchOn);
            } catch (_) {}
        });

        document.getElementById('scanner-manual-form').addEventListener('submit', (e) => {
            e.preventDefault();
            const input = document.getElementById('scanner-manual-input');
            const value = (input ? input.value : '').trim();
            if (value) {
                const gameId = extractGameID(value);
                triggerSuccess(gameId, value);
            }
        });
    }

    async function enumerateCameras() {
        try {
            const devices = await navigator.mediaDevices.enumerateDevices();
            videoDevices = devices.filter(d => d.kind === 'videoinput');
            const switchBtn = document.getElementById('scanner-switch-btn');
            if (switchBtn) {
                switchBtn.classList.toggle('hidden', videoDevices.length <= 1);
            }
        } catch (_) {}
    }

    async function startStream(deviceId = null) {
        stopStream();
        const video = document.getElementById('scanner-video');
        const errEl = document.getElementById('scanner-error');
        if (errEl) errEl.classList.add('hidden');

        try {
            const constraints = {
                video: deviceId
                    ? { deviceId: { exact: deviceId } }
                    : { facingMode: { ideal: 'environment' }, width: { ideal: 1280 }, height: { ideal: 720 } }
            };

            const stream = await navigator.mediaDevices.getUserMedia(constraints);
            activeStream = stream;
            video.srcObject = stream;
            await video.play();

            // Check torch capabilities
            const track = stream.getVideoTracks()[0];
            torchTrack = track;
            const torchBtn = document.getElementById('scanner-torch-btn');
            if (torchBtn && track && track.getCapabilities) {
                const caps = track.getCapabilities();
                torchBtn.classList.toggle('hidden', !caps.torch);
            } else if (torchBtn) {
                torchBtn.classList.add('hidden');
            }

            await enumerateCameras();
            startScanningLoop();
        } catch (err) {
            console.error('Kamerafehler:', err);
            if (errEl) {
                errEl.textContent = 'Kamera konnte nicht gestartet werden. Bitte Zugriff im Browser erlauben oder Code manuell eingeben.';
                errEl.classList.remove('hidden');
            }
        }
    }

    function stopStream() {
        if (scanAnimationFrame) {
            cancelAnimationFrame(scanAnimationFrame);
            scanAnimationFrame = null;
        }
        if (activeStream) {
            activeStream.getTracks().forEach(t => t.stop());
            activeStream = null;
        }
        torchTrack = null;
        torchOn = false;
        const video = document.getElementById('scanner-video');
        if (video) video.srcObject = null;
    }

    function triggerSuccess(gameId, rawValue) {
        playScanBeep();
        const targetBox = document.getElementById('scanner-target-box');
        if (targetBox) {
            targetBox.classList.remove('border-blue-400/70');
            targetBox.classList.add('border-green-400', 'bg-green-500/20');
        }

        setTimeout(() => {
            if (activeCallback) {
                activeCallback(gameId, rawValue);
            }
            closeCameraScanner();
        }, 150);
    }

    function startScanningLoop() {
        const video = document.getElementById('scanner-video');
        const canvas = document.getElementById('scanner-canvas');
        if (!video || !canvas) return;

        let isScanning = true;

        const scanFrame = async () => {
            if (!activeStream || video.readyState < 2) {
                scanAnimationFrame = requestAnimationFrame(scanFrame);
                return;
            }

            if (barcodeDetector) {
                try {
                    const barcodes = await barcodeDetector.detect(video);
                    if (barcodes && barcodes.length > 0) {
                        const raw = barcodes[0].rawValue || '';
                        const gameId = extractGameID(raw);
                        if (gameId) {
                            triggerSuccess(gameId, raw);
                            return;
                        }
                    }
                } catch (_) {}
            }

            scanAnimationFrame = requestAnimationFrame(scanFrame);
        };

        scanAnimationFrame = requestAnimationFrame(scanFrame);
    }

    function openCameraScanner(callback, options = {}) {
        createScannerModal();
        activeCallback = callback;
        const modal = document.getElementById('camera-scanner-modal');
        const input = document.getElementById('scanner-manual-input');
        const targetBox = document.getElementById('scanner-target-box');
        if (input) input.value = '';
        if (targetBox) {
            targetBox.className = 'pointer-events-none absolute h-44 w-44 sm:h-56 sm:w-56 rounded-2xl border-2 border-blue-400/70 shadow-[0_0_0_9999px_rgba(0,0,0,0.45)] transition-colors duration-200';
        }

        modal.classList.remove('hidden');
        modal.classList.add('flex');
        startStream();
    }

    function closeCameraScanner() {
        stopStream();
        const modal = document.getElementById('camera-scanner-modal');
        if (modal) {
            modal.classList.add('hidden');
            modal.classList.remove('flex');
        }
        activeCallback = null;
    }

    // Expose openCameraScanner globally
    window.openCameraScanner = openCameraScanner;
    window.closeCameraScanner = closeCameraScanner;
})();
