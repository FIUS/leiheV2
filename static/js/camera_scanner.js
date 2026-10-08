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

    // --- PURE JAVASCRIPT BARCODE & QR DECODERS ---

    // 1. Code 128 Patterns & Decoder
    const CODE128_PATTERNS = [
        "212222","222122","222221","121223","121322","131222","122213","122312","132212","221213",
        "221312","231212","112232","122132","122231","113222","123122","123221","223211","221132",
        "221231","213212","223112","312131","311222","321122","321221","312212","322112","322211",
        "212123","212321","232121","111323","131123","131321","112313","132113","132311","211313",
        "231113","231311","112133","112331","132131","113123","113321","133121","313121","211331",
        "231131","213113","213311","213131","311123","311321","331121","312113","312311","332111",
        "314111","221411","431111","111224","111422","121124","121421","141122","141221","112214",
        "112412","122114","122411","142112","142211","241211","221114","413111","241112","134111",
        "111242","121142","121241","114212","124112","124211","411212","421112","421211","212141",
        "214121","412121","111143","111341","131141","114113","114311","411113","411311","113141",
        "114131","311141","411131","211412","211214","211232","2331112"
    ];

    const CODE128_PATTERN_MAP = {};
    CODE128_PATTERNS.forEach((p, idx) => { CODE128_PATTERN_MAP[p] = idx; });

    function decodeCode128Runs(runs) {
        if (runs.length < 13) return null; // minimum runs: Start (6), 1 char (6), Check (6), Stop (7) -> ~25 runs

        for (let i = 0; i <= runs.length - 19; i++) {
            // Check for Start A (103), Start B (104), Start C (105)
            const r6 = runs.slice(i, i + 6);
            const total = r6.reduce((a, b) => a + b, 0);
            if (total === 0) continue;
            const pat = r6.map(r => Math.max(1, Math.min(4, Math.round(r * 11 / total)))).join('');
            const startVal = CODE128_PATTERN_MAP[pat];
            if (startVal !== 103 && startVal !== 104 && startVal !== 105) continue;

            let codeSet = startVal === 103 ? 'A' : (startVal === 104 ? 'B' : 'C');
            const values = [startVal];
            let curr = i + 6;
            let foundStop = false;

            while (curr <= runs.length - 6) {
                // Check if remaining runs form Stop (7 elements)
                if (curr <= runs.length - 7) {
                    const r7 = runs.slice(curr, curr + 7);
                    const t7 = r7.reduce((a, b) => a + b, 0);
                    const p7 = r7.map(r => Math.max(1, Math.min(4, Math.round(r * 13 / t7)))).join('');
                    if (p7 === "2331112" || CODE128_PATTERN_MAP[p7] === 106) {
                        foundStop = true;
                        break;
                    }
                }

                const rc = runs.slice(curr, curr + 6);
                const tc = rc.reduce((a, b) => a + b, 0);
                const pc = rc.map(r => Math.max(1, Math.min(4, Math.round(r * 11 / tc)))).join('');
                const val = CODE128_PATTERN_MAP[pc];
                if (val === undefined) break;

                values.push(val);
                curr += 6;
            }

            if (foundStop && values.length >= 3) {
                const checkVal = values[values.length - 1];
                const dataVals = values.slice(1, -1);
                let sum = values[0];
                for (let k = 0; k < dataVals.length; k++) {
                    sum += dataVals[k] * (k + 1);
                }
                if (sum % 103 === checkVal) {
                    let result = '';
                    let activeSet = codeSet;
                    for (let k = 0; k < dataVals.length; k++) {
                        const v = dataVals[k];
                        if (v === 101) activeSet = 'A';
                        else if (v === 100) activeSet = 'B';
                        else if (v === 99) activeSet = 'C';
                        else if (v === 102) { /* FNC1 */ }
                        else if (activeSet === 'C') {
                            result += (v < 10 ? '0' : '') + v;
                        } else if (activeSet === 'B') {
                            result += String.fromCharCode(v + 32);
                        } else if (activeSet === 'A') {
                            result += v < 64 ? String.fromCharCode(v + 32) : String.fromCharCode(v - 64);
                        }
                    }
                    if (result.length > 0) return result;
                }
            }
        }
        return null;
    }

    // 2. Code 39 Patterns & Decoder
    const CODE39_CHARS = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ-. $/+%*";
    const CODE39_PATTERNS = {
        "000110100": "0", "100100001": "1", "001100001": "2", "101100000": "3",
        "000110001": "4", "100110000": "5", "001110000": "6", "000100101": "7",
        "100100100": "8", "001100100": "9", "100001001": "A", "001001001": "B",
        "101001000": "C", "000011001": "D", "100011000": "E", "001011000": "F",
        "000001101": "G", "100001100": "H", "001001100": "I", "000011100": "J",
        "100000011": "K", "001000011": "L", "101000010": "M", "000010011": "N",
        "100010010": "O", "001010010": "P", "000000111": "Q", "100000110": "R",
        "001000110": "S", "000010110": "T", "110000001": "U", "011000001": "V",
        "111000000": "W", "010010001": "X", "110010000": "Y", "011010000": "Z",
        "010000101": "-", "110000100": ".", "011000100": " ", "010101000": "$",
        "010100010": "/", "010001010": "+", "000101010": "%", "010010100": "*"
    };

    function decodeCode39Runs(runs) {
        if (runs.length < 19) return null; // * + 1 char + * (9 + 1 + 9 + 1 + 9)
        for (let i = 0; i <= runs.length - 19; i++) {
            // Check Start asterisk '*'
            const r9 = runs.slice(i, i + 9);
            const sorted = [...r9].sort((a, b) => a - b);
            const threshold = (sorted[5] + sorted[6]) / 2;
            const pat = r9.map(r => r > threshold ? '1' : '0').join('');
            if (CODE39_PATTERNS[pat] !== '*') continue;

            let result = '';
            let curr = i + 10; // 9 elements + 1 gap
            let foundEnd = false;

            while (curr <= runs.length - 9) {
                const chunk = runs.slice(curr, curr + 9);
                const sChunk = [...chunk].sort((a, b) => a - b);
                const th = (sChunk[5] + sChunk[6]) / 2;
                const p = chunk.map(r => r > th ? '1' : '0').join('');
                const char = CODE39_PATTERNS[p];
                if (!char) break;
                if (char === '*') {
                    foundEnd = true;
                    break;
                }
                result += char;
                curr += 10;
            }

            if (foundEnd && result.length > 0) return result;
        }
        return null;
    }

    // 3. EAN-13 / EAN-8 Decoder
    const EAN_L_PATTERNS = ["0001101","0011001","0010011","0111101","0100011","0110001","0101111","0111011","0110111","0001011"];
    const EAN_G_PATTERNS = ["0100111","0110011","0011011","0100001","0011101","0111001","0000101","0010001","0001001","0010111"];
    const EAN_R_PATTERNS = ["1110010","1100110","1101100","1000010","1011100","1001110","1010000","1000100","1001000","1110100"];
    const EAN_FIRST_DIGIT = ["LLLLLL","LLGLGG","LLGGLG","LLGGGL","LGLLGG","LGGLLG","LGGGLL","LGLGLG","LGLGGL","LGGLGL"];

    function decodeEANRuns(runs) {
        if (runs.length < 59) return null; // EAN-13: 3 start + 24 (6*4) + 5 middle + 24 (6*4) + 3 end = 59 runs
        for (let i = 0; i <= runs.length - 59; i++) {
            // Guard 1-1-1
            let curr = i + 3;
            let leftDigits = [];
            let parities = [];

            for (let d = 0; d < 6; d++) {
                const r4 = runs.slice(curr, curr + 4);
                const total = r4.reduce((a, b) => a + b, 0);
                if (total === 0) break;
                const p = r4.map(r => Math.max(1, Math.min(4, Math.round(r * 7 / total)))).join('');
                let matched = false;
                for (let num = 0; num < 10; num++) {
                    const lSeq = [3,2,1,1]; // generic L length map
                    // Compare binary representation
                    let bin = '';
                    for (let bit = 0; bit < 4; bit++) bin += (bit % 2 === 0 ? '0' : '1').repeat(Math.max(1, Math.round(r4[bit] * 7 / total)));
                    if (bin.length === 7) {
                        if (EAN_L_PATTERNS[num] === bin) { leftDigits.push(num); parities.push('L'); matched = true; break; }
                        if (EAN_G_PATTERNS[num] === bin) { leftDigits.push(num); parities.push('G'); matched = true; break; }
                    }
                }
                if (!matched) break;
                curr += 4;
            }

            if (leftDigits.length !== 6) continue;
            curr += 5; // Middle guard

            let rightDigits = [];
            for (let d = 0; d < 6; d++) {
                const r4 = runs.slice(curr, curr + 4);
                const total = r4.reduce((a, b) => a + b, 0);
                if (total === 0) break;
                let bin = '';
                for (let bit = 0; bit < 4; bit++) bin += (bit % 2 === 0 ? '1' : '0').repeat(Math.max(1, Math.round(r4[bit] * 7 / total)));
                let matched = false;
                for (let num = 0; num < 10; num++) {
                    if (bin.length === 7 && EAN_R_PATTERNS[num] === bin) {
                        rightDigits.push(num);
                        matched = true;
                        break;
                    }
                }
                if (!matched) break;
                curr += 4;
            }

            if (rightDigits.length === 6) {
                const parityStr = parities.join('');
                const firstDigit = EAN_FIRST_DIGIT.indexOf(parityStr);
                if (firstDigit !== -1) {
                    const fullDigits = [firstDigit, ...leftDigits, ...rightDigits];
                    let sum = 0;
                    for (let k = 0; k < 12; k++) sum += fullDigits[k] * (k % 2 === 0 ? 1 : 3);
                    const check = (10 - (sum % 10)) % 10;
                    if (check === fullDigits[12]) return fullDigits.join('');
                }
            }
        }
        return null;
    }

    // 4. Lightweight QR-Code Finder and Decoder
    function decodeQRFromLuminance(gray, width, height) {
        // Fast Finder Pattern Finder (1:1:3:1:1 ratio)
        const finders = [];
        const step = Math.max(1, Math.floor(height / 100));

        for (let y = 10; y < height - 10; y += step) {
            const rowOffset = y * width;
            let currentBit = (gray[rowOffset] < 128) ? 1 : 0;
            let currentLen = 0;
            const runs = [];
            const runStarts = [];

            for (let x = 0; x < width; x++) {
                const bit = (gray[rowOffset + x] < 128) ? 1 : 0;
                if (bit === currentBit) {
                    currentLen++;
                } else {
                    runs.push(currentLen);
                    runStarts.push(x - currentLen);
                    currentBit = bit;
                    currentLen = 1;

                    if (runs.length >= 5 && currentBit === 0) { // last finished run was black
                        const r = runs.slice(-5);
                        const total = r[0] + r[1] + r[2] + r[3] + r[4];
                        const u = total / 7;
                        const tol = Math.max(1.5, u * 0.7);

                        if (Math.abs(r[0] - u) < tol && Math.abs(r[1] - u) < tol &&
                            Math.abs(r[2] - 3*u) < tol * 3 && Math.abs(r[3] - u) < tol &&
                            Math.abs(r[4] - u) < tol) {
                            const cx = runStarts[runStarts.length - 5] + total / 2;
                            // Check vertical consistency
                            let vCount = 0;
                            let vTotal = 0;
                            const icx = Math.round(cx);
                            if (icx >= 0 && icx < width) {
                                let cy = y;
                                // Simple clustering of finder points
                                let exists = false;
                                for (const f of finders) {
                                    if (Math.hypot(f.x - cx, f.y - cy) < u * 3) {
                                        f.x = (f.x + cx) / 2;
                                        f.y = (f.y + cy) / 2;
                                        f.count++;
                                        exists = true;
                                        break;
                                    }
                                }
                                if (!exists) {
                                    finders.push({ x: cx, y: cy, size: u, count: 1 });
                                }
                            }
                        }
                    }
                }
            }
        }

        // Need at least 3 finder candidates
        const qualified = finders.filter(f => f.count >= 2);
        if (qualified.length < 3) return null;

        // Try combinations of 3 finders
        for (let i = 0; i < qualified.length; i++) {
            for (let j = i + 1; j < qualified.length; j++) {
                for (let k = j + 1; k < qualified.length; k++) {
                    const p1 = qualified[i], p2 = qualified[j], p3 = qualified[k];
                    const d12 = Math.hypot(p1.x - p2.x, p1.y - p2.y);
                    const d23 = Math.hypot(p2.x - p3.x, p2.y - p3.y);
                    const d31 = Math.hypot(p3.x - p1.x, p3.y - p1.y);

                    const sides = [{d: d12, a: p1, b: p2, c: p3}, {d: d23, a: p2, b: p3, c: p1}, {d: d31, a: p3, b: p1, c: p2}];
                    sides.sort((x, y) => y.d - x.d); // Hypotenuse is largest

                    const hyp = sides[0];
                    const tl = hyp.c; // Top-Left is opposite hypotenuse
                    let tr = hyp.a;
                    let bl = hyp.b;

                    // Cross product to orient TR vs BL
                    const cross = (tr.x - tl.x) * (bl.y - tl.y) - (tr.y - tl.y) * (bl.x - tl.x);
                    if (cross < 0) { const temp = tr; tr = bl; bl = temp; }

                    const side1 = Math.hypot(tr.x - tl.x, tr.y - tl.y);
                    const side2 = Math.hypot(bl.x - tl.x, bl.y - tl.y);
                    if (Math.abs(side1 - side2) / Math.max(side1, side2) > 0.35) continue;

                    const avgModule = (tl.size + tr.size + bl.size) / 3;
                    if (avgModule <= 0) continue;

                    const estDimension = Math.round(((side1 + side2) / 2) / avgModule) + 7;
                    const version = Math.max(1, Math.min(10, Math.round((estDimension - 17) / 4)));
                    const N = 4 * version + 17;

                    // Sample grid
                    const matrix = [];
                    for (let r = 0; r < N; r++) {
                        matrix[r] = new Uint8Array(N);
                        const vY = r / (N - 1);
                        for (let c = 0; c < N; c++) {
                            const vX = c / (N - 1);
                            // Bilinear coordinate mapping
                            const px = Math.round(tl.x + vX * (tr.x - tl.x) + vY * (bl.x - tl.x));
                            const py = Math.round(tl.y + vX * (tr.y - tl.y) + vY * (bl.y - tl.y));
                            if (px >= 0 && px < width && py >= 0 && py < height) {
                                matrix[r][c] = gray[py * width + px] < 128 ? 1 : 0;
                            }
                        }
                    }

                    // Format Info (Read top-left format info bits)
                    let formatBits = 0;
                    for (let b = 0; b <= 5; b++) formatBits = (formatBits << 1) | matrix[8][b];
                    formatBits = (formatBits << 1) | matrix[8][7];
                    formatBits = (formatBits << 1) | matrix[8][8];
                    formatBits = (formatBits << 1) | matrix[7][8];
                    for (let b = 5; b >= 0; b--) formatBits = (formatBits << 1) | matrix[b][8];
                    formatBits ^= 0x5412; // Unmask format bits

                    const maskPattern = (formatBits >> 10) & 0x07;
                    const maskFn = [
                        (r, c) => (r + c) % 2 === 0,
                        (r, c) => r % 2 === 0,
                        (r, c) => c % 3 === 0,
                        (r, c) => (r + c) % 3 === 0,
                        (r, c) => (Math.floor(r / 2) + Math.floor(c / 3)) % 2 === 0,
                        (r, c) => ((r * c) % 2) + ((r * c) % 3) === 0,
                        (r, c) => (((r * c) % 2) + ((r * c) % 3)) % 2 === 0,
                        (r, c) => (((r + c) % 2) + ((r * c) % 3)) % 2 === 0
                    ][maskPattern] || ((r, c) => (r + c) % 2 === 0);

                    // Read bits unmasked
                    const bits = [];
                    let upward = true;
                    for (let col = N - 1; col > 0; col -= 2) {
                        if (col === 6) col--; // Skip vertical timing pattern
                        for (let count = 0; count < N; count++) {
                            const row = upward ? (N - 1 - count) : count;
                            for (let c = col; c >= col - 1; c--) {
                                // Skip function patterns
                                const inTL = row <= 8 && c <= 8;
                                const inTR = row <= 8 && c >= N - 8;
                                const inBL = row >= N - 8 && c <= 8;
                                const inTiming = row === 6 || c === 6;
                                if (!inTL && !inTR && !inBL && !inTiming) {
                                    let bit = matrix[row][c];
                                    if (maskFn(row, c)) bit ^= 1;
                                    bits.push(bit);
                                }
                            }
                        }
                        upward = !upward;
                    }

                    // Convert bit stream to bytes
                    const bytes = [];
                    for (let b = 0; b <= bits.length - 8; b += 8) {
                        let val = 0;
                        for (let k = 0; k < 8; k++) val = (val << 1) | bits[b + k];
                        bytes.push(val);
                    }

                    // Parse Data Mode (Byte mode = 0100, Alphanumeric = 0010, Numeric = 0001)
                    if (bytes.length >= 2) {
                        let bitPtr = 0;
                        const readBits = (n) => {
                            let v = 0;
                            for (let k = 0; k < n; k++) {
                                if (bitPtr < bits.length) v = (v << 1) | bits[bitPtr++];
                            }
                            return v;
                        };

                        const mode = readBits(4);
                        let text = '';
                        if (mode === 4) { // Byte Mode
                            const count = readBits(version <= 9 ? 8 : 16);
                            const charBytes = [];
                            for (let c = 0; c < count; c++) charBytes.push(readBits(8));
                            try { text = new TextDecoder('utf-8').decode(new Uint8Array(charBytes)); }
                            catch (_) { text = String.fromCharCode(...charBytes); }
                        } else if (mode === 1) { // Numeric Mode
                            const count = readBits(version <= 9 ? 10 : (version <= 26 ? 12 : 14));
                            let rem = count;
                            while (rem >= 3) { text += String(readBits(10)).padStart(3, '0'); rem -= 3; }
                            if (rem === 2) text += String(readBits(7)).padStart(2, '0');
                            else if (rem === 1) text += String(readBits(4));
                        } else if (mode === 2) { // Alphanumeric Mode
                            const ALPHANUM = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:";
                            const count = readBits(version <= 9 ? 9 : (version <= 26 ? 11 : 13));
                            let rem = count;
                            while (rem >= 2) {
                                const pair = readBits(11);
                                text += ALPHANUM[Math.floor(pair / 45)] + ALPHANUM[pair % 45];
                                rem -= 2;
                            }
                            if (rem === 1) text += ALPHANUM[readBits(6)];
                        }

                        if (text && text.trim().length > 0) return text.trim();
                    }
                }
            }
        }
        return null;
    }

    // High-Level Frame Scanner
    function decodeFrame(imageData) {
        const { width, height, data } = imageData;
        const gray = new Uint8Array(width * height);

        // Convert to grayscale
        for (let i = 0, p = 0; i < data.length; i += 4, p++) {
            gray[p] = (data[i] * 77 + data[i + 1] * 150 + data[i + 2] * 29) >> 8;
        }

        // 1. Try QR Code
        const qrResult = decodeQRFromLuminance(gray, width, height);
        if (qrResult) return qrResult;

        // 2. Sample 1D Scanlines (Horizontal across middle 70% of frame)
        const lineCount = 15;
        const startY = Math.floor(height * 0.15);
        const endY = Math.floor(height * 0.85);
        const stepY = Math.max(1, Math.floor((endY - startY) / lineCount));

        for (let y = startY; y <= endY; y += stepY) {
            const rowOffset = y * width;
            // Moving average binarization
            let avg = 128;
            const binarized = new Uint8Array(width);
            for (let x = 0; x < width; x++) {
                const g = gray[rowOffset + x];
                avg = ((avg * 7) + g) >> 3;
                binarized[x] = g < (avg * 0.9) ? 1 : 0;
            }

            // Convert to Run-Length
            const runs = [];
            let currVal = binarized[0];
            let len = 0;
            for (let x = 0; x < width; x++) {
                if (binarized[x] === currVal) {
                    len++;
                } else {
                    runs.push(len);
                    currVal = binarized[x];
                    len = 1;
                }
            }
            if (len > 0) runs.push(len);

            // Attempt Code 128
            const c128 = decodeCode128Runs(runs);
            if (c128) return c128;

            // Attempt Code 39
            const c39 = decodeCode39Runs(runs);
            if (c39) return c39;

            // Attempt EAN
            const ean = decodeEANRuns(runs);
            if (ean) return ean;
        }

        return null;
    }

    // --- SCANNER MODAL & CAMERA STREAM MANAGEMENT ---

    let activeStream = null;
    let scanAnimationFrame = null;
    let activeCallback = null;
    let barcodeDetector = null;
    let videoDevices = [];
    let currentDeviceIndex = 0;
    let torchTrack = null;
    let torchOn = false;

    async function initBarcodeDetector() {
        if (!('BarcodeDetector' in window)) return null;
        try {
            if (typeof window.BarcodeDetector.getSupportedFormats === 'function') {
                const supported = await window.BarcodeDetector.getSupportedFormats();
                const preferred = ['qr_code', 'code_128', 'code_39', 'ean_13', 'ean_8', 'upc_a', 'upc_e'];
                const formats = preferred.filter(f => supported.includes(f));
                if (formats.length > 0) return new window.BarcodeDetector({ formats });
            }
            return new window.BarcodeDetector();
        } catch (_) {
            try { return new window.BarcodeDetector(); } catch (_) { return null; }
        }
    }

    initBarcodeDetector().then(detector => { barcodeDetector = detector; });

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

        let frameCounter = 0;

        const scanFrame = async () => {
            if (!activeStream || video.readyState < 2 || video.videoWidth === 0) {
                scanAnimationFrame = requestAnimationFrame(scanFrame);
                return;
            }

            frameCounter++;

            // 1. Try Native BarcodeDetector API if available
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

            // 2. Pure JavaScript Fallback Decoder (runs on canvas frame)
            try {
                canvas.width = video.videoWidth;
                canvas.height = video.videoHeight;
                const ctx = canvas.getContext('2d', { willReadFrequently: true });
                ctx.drawImage(video, 0, 0, canvas.width, canvas.height);

                // Sample viewfinder center region first for fast response
                const cropW = Math.min(canvas.width, Math.max(200, Math.floor(canvas.width * 0.65)));
                const cropH = Math.min(canvas.height, Math.max(200, Math.floor(canvas.height * 0.65)));
                const cropX = Math.floor((canvas.width - cropW) / 2);
                const cropY = Math.floor((canvas.height - cropH) / 2);

                const centerImg = ctx.getImageData(cropX, cropY, cropW, cropH);
                let decoded = decodeFrame(centerImg);

                // If not found in center, scan full frame on alternating frames
                if (!decoded && frameCounter % 2 === 0) {
                    const fullImg = ctx.getImageData(0, 0, canvas.width, canvas.height);
                    decoded = decodeFrame(fullImg);
                }

                if (decoded) {
                    const gameId = extractGameID(decoded);
                    if (gameId) {
                        triggerSuccess(gameId, decoded);
                        return;
                    }
                }
            } catch (_) {}

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
