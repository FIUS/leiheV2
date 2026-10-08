const SVG_NS = 'http://www.w3.org/2000/svg';
const LABEL_GAP_MM = 2.5;

function parseLabelMillimeters(value, fieldName) {
    const parsed = Number(String(value).trim().replace(',', '.'));
    if (!Number.isFinite(parsed) || parsed <= 0 || parsed > 1000) {
        throw new Error(`${fieldName} muss größer als 0 mm und höchstens 1000 mm sein.`);
    }
    return parsed;
}

function parseLabelSize(value) {
    const match = String(value).trim().match(/^(\d+(?:[.,]\d+)?)\s*(?:mm)?\s*[x×]\s*(\d+(?:[.,]\d+)?)\s*(?:mm)?$/i);
    if (!match) {
        throw new Error('Etikettengröße bitte als „Breite x Höhe“ in mm eingeben, zum Beispiel 50 x 30.');
    }
    return {
        width: parseLabelMillimeters(match[1], 'Etikettenbreite'),
        height: parseLabelMillimeters(match[2], 'Etikettenhöhe')
    };
}

function labelSvgElement(name, attributes = {}) {
    const element = document.createElementNS(SVG_NS, name);
    Object.entries(attributes).forEach(([key, value]) => element.setAttribute(key, String(value)));
    return element;
}

function cleanLabelFilename(value) {
    const slug = value.normalize('NFKD')
        .replace(/[\u0300-\u036f]/g, '')
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '_')
        .replace(/^_+|_+$/g, '')
        .slice(0, 60);
    return slug || 'spiel';
}

function wrapLabelText(value, maxCharacters) {
    const words = value.trim().split(/\s+/).filter(Boolean);
    const lines = [];
    let line = '';
    words.forEach((word) => {
        if (word.length > maxCharacters) {
            if (line) lines.push(line);
            line = '';
            for (let index = 0; index < word.length; index += maxCharacters) {
                lines.push(word.slice(index, index + maxCharacters));
            }
            return;
        }
        const candidate = line ? `${line} ${word}` : word;
        if (candidate.length > maxCharacters && line) {
            lines.push(line);
            line = word;
        } else {
            line = candidate;
        }
    });
    if (line) lines.push(line);
    return lines;
}

function appendLabelText(parent, value, x, y, maxWidth, options = {}) {
    const fontSize = options.fontSize || 3.2;
    const maxCharacters = Math.max(4, Math.floor(maxWidth / (fontSize * 0.56)));
    const lines = options.lines || wrapLabelText(value, maxCharacters);
    const text = labelSvgElement('text', {
        x, y, fill: options.fill || '#111827',
        'font-family': options.fontFamily || 'Arial, Helvetica, sans-serif',
        'font-size': fontSize,
        'font-weight': options.fontWeight || '400',
        'text-anchor': options.anchor || 'start'
    });
    lines.forEach((line, index) => {
        const tspan = labelSvgElement('tspan', { x, dy: index === 0 ? 0 : fontSize * 1.2 });
        tspan.textContent = line;
        text.appendChild(tspan);
    });
    parent.appendChild(text);
    return lines.length * fontSize * 1.2;
}

function wrapLabelTitle(value, maxCharacters, maxLines = 2) {
    const lines = wrapLabelText(value, maxCharacters);
    if (lines.length <= maxLines) return lines;
    const visibleLines = lines.slice(0, maxLines);
    let lastLine = visibleLines[maxLines - 1];
    if (lastLine.length >= maxCharacters) lastLine = lastLine.slice(0, maxCharacters - 1);
    visibleLines[maxLines - 1] = `${lastLine.trimEnd()}…`;
    return visibleLines;
}

function fitLabelTitle(value, maxWidth, maxLines, overflow, initialFontSize) {
    let fontSize = initialFontSize;
    let maxCharacters = Math.max(1, Math.floor(maxWidth / (fontSize * 0.56)));
    let lines = wrapLabelText(value, maxCharacters);

    if (lines.length > maxLines && overflow === 'shrink') {
        while (lines.length > maxLines && fontSize > 0.8) {
            fontSize = Math.max(0.8, fontSize - 0.1);
            maxCharacters = Math.max(1, Math.floor(maxWidth / (fontSize * 0.56)));
            lines = wrapLabelText(value, maxCharacters);
        }
    }

    if (lines.length > maxLines) {
        lines = wrapLabelTitle(value, maxCharacters, maxLines);
    }
    return { lines, fontSize };
}

document.addEventListener('alpine:init', () => {
    Alpine.data('barcodeDesigner', () => ({
        rollType: 'continuous',
        bandWidth: '29',
        lengthMode: 'dynamic',
        fixedLength: 60,
        dieCutSize: '29x90',
        codeType: 'qr',
        payloadMode: 'id',
        urlTemplate: 'https://verleih.fius.de/{id}',
        layout: 'vertical',
        showBrand: true,
        brandText: 'Verleih',
        showID: true,
        idTemplate: 'ID: #{id}',
        showTitle: true,
        titleMaxLines: 2,
        titleOverflow: 'ellipsis',
        showRoom: false,
        roomFontSize: 2.4,
        showShelf: false,
        shelfFontSize: 2.4,
        onlyCompleteLocations: false,
        items: [],
        selectedIds: [],
        search: '',
        exporting: false,
        exportProgress: 0,
        error: '',

        async init() {
            const updatePreview = () => this.renderPreview();
            [
                'rollType', 'bandWidth', 'lengthMode', 'fixedLength', 'dieCutSize',
                'codeType', 'payloadMode', 'urlTemplate', 'layout', 'showBrand',
                'brandText', 'showID', 'idTemplate', 'showTitle', 'titleMaxLines', 'titleOverflow',
                'showRoom', 'roomFontSize', 'showShelf', 'shelfFontSize', 'selectedIds'
            ].forEach((key) => this.$watch(key, updatePreview));

            try {
                const response = await fetch('/admin/api/etiketten/spiele', {
                    headers: { Accept: 'application/json' }
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Spiele konnten nicht geladen werden.');
                this.items = data.items || [];
                this.renderPreview();
            } catch (error) {
                this.error = error.message || 'Spiele konnten nicht geladen werden.';
            }
        },

        get filteredItems() {
            const search = this.search.trim().toLocaleLowerCase('de');
            const cleanID = window.extractGameID ? window.extractGameID(search) : '';
            return this.items.filter(item =>
                !search ||
                (cleanID && String(item.id) === cleanID) ||
                `${item.name} ${item.id}`.toLocaleLowerCase('de').includes(search) ||
                (cleanID && `${item.name} ${item.id}`.toLocaleLowerCase('de').includes(cleanID))
            );
        },

        get selectedItems() {
            const selected = new Set(this.selectedIds);
            return this.items.filter(item => selected.has(item.id));
        },

        get exportItems() {
            return this.onlyCompleteLocations
                ? this.selectedItems.filter(item => this.hasCompleteLocation(item))
                : this.selectedItems;
        },

        hasCompleteLocation(item) {
            return Boolean(String(item.room || '').trim() && String(item.shelf || '').trim());
        },

        get previewItem() {
            return this.selectedItems[0] || null;
        },

        get allVisibleSelected() {
            return this.filteredItems.length > 0 && this.filteredItems.every(item => this.selectedIds.includes(item.id));
        },

        get labelDimensions() {
            const item = this.previewItem || { id: 0, name: '' };
            try {
                return this.outputDimensionsForItem(item);
            } catch {
                return { width: '—', height: '—' };
            }
        },

        outputDimensionsForItem(item) {
            return this.dimensionsForItem(item);
        },

        dimensionsForItem(item) {
            if (this.rollType === 'diecut') {
                const dimensions = parseLabelSize(this.dieCutSize);
                const isLandscape = this.layout === 'horizontal';
                const oriented = isLandscape === (dimensions.width > dimensions.height)
                    ? dimensions
                    : { width: dimensions.height, height: dimensions.width };
                return this.fitDimensionsToContent(oriented, item);
            }
            const width = parseLabelMillimeters(this.bandWidth, 'Bandbreite');
            if (this.lengthMode === 'fixed') {
                const dimensions = { width, height: parseLabelMillimeters(this.fixedLength, 'Etikettenlänge') };
                const oriented = this.layout === 'horizontal'
                    ? { width: dimensions.height, height: dimensions.width }
                    : dimensions;
                return this.fitDimensionsToContent(oriented, item);
            }
            if (this.layout === 'vertical') {
                const flow = this.verticalFlow(item, width);
                return { width, height: Math.max(1, Math.ceil(flow.height)) };
            }
            const horizontalWidth = Math.max(30, Math.ceil(width * 1.8));
            const flow = this.horizontalTextFlow(item, horizontalWidth);
            return { width: horizontalWidth, height: Math.max(width, Math.ceil(flow.height)) };
        },

        fitDimensionsToContent(dimensions, item) {
            const contentHeight = this.layout === 'vertical'
                ? this.verticalFlow(item, dimensions.width).height
                : this.horizontalTextFlow(item, dimensions.width).height;
            return { ...dimensions, height: Math.max(dimensions.height, Math.ceil(contentHeight)) };
        },

        titleBlock(item, maxWidth) {
            const fontSize = 2.8;
            const layout = this.showTitle
                ? fitLabelTitle(item.name, maxWidth, Number(this.titleMaxLines), this.titleOverflow, fontSize)
                : { lines: [], fontSize };
            return { value: item.name, lines: layout.lines, fontSize: layout.fontSize, height: layout.lines.length * layout.fontSize * 1.2 };
        },

        locationBlocks(item, maxWidth) {
            const blocks = [];
            [
                { show: this.showRoom, value: item.room, fontSize: Number(this.roomFontSize), weight: '400' },
                { show: this.showShelf, value: item.shelf, fontSize: Number(this.shelfFontSize), weight: '700' }
            ].forEach(({ show, value, fontSize, weight }) => {
                const location = String(value || '').trim();
                if (!show || !location) return;
                const lines = wrapLabelText(location, Math.max(1, Math.floor(maxWidth / (fontSize * 0.56))));
                blocks.push({ value: location, lines, fontSize, height: lines.length * fontSize * 1.2, weight });
            });
            return blocks;
        },

        verticalFlow(item, width) {
            const textWidth = width;
            let header = null;
            if (this.showBrand && this.brandText.trim()) {
                const fontSize = Math.min(3.4, textWidth / 8);
                const lines = wrapLabelText(this.brandText.trim(), Math.max(1, Math.floor(textWidth / (fontSize * 0.56))));
                header = { value: this.brandText.trim(), lines, fontSize, height: lines.length * fontSize * 1.2, weight: '700' };
            }
            const codeHeight = this.codeType === 'qr' ? width : Math.min(13, width);
            const codeY = header ? header.height + LABEL_GAP_MM : 0;
            const blocks = [];
            const idLine = this.idTemplate.replaceAll('{id}', String(item.id));
            if (this.showID) {
                const fontSize = 3.2;
                const lines = wrapLabelText(idLine, Math.max(1, Math.floor(textWidth / (fontSize * 0.56))));
                blocks.push({ value: idLine, lines, fontSize, height: lines.length * fontSize * 1.2, weight: '700' });
            }
            if (this.showTitle) blocks.push(this.titleBlock(item, textWidth));
            blocks.push(...this.locationBlocks(item, textWidth));

            const positionedBlocks = [];
            let textY = codeY + codeHeight + (blocks.length ? LABEL_GAP_MM : 0);
            blocks.forEach((block, index) => {
                positionedBlocks.push({ ...block, y: textY + block.fontSize });
                textY += block.height;
                if (index < blocks.length - 1) textY += LABEL_GAP_MM;
            });
            return {
                height: Math.max(header ? header.height : 0, codeY + codeHeight, textY),
                header: header ? { ...header, y: header.fontSize } : null,
                codeY,
                codeHeight,
                textBlocks: positionedBlocks,
                textWidth
            };
        },

        horizontalTextFlow(item, width) {
            let nominalCodeWidth;
            if (this.rollType === 'diecut') {
                const size = parseLabelSize(this.dieCutSize);
                nominalCodeWidth = Math.min(size.width, size.height);
            } else {
                nominalCodeWidth = parseLabelMillimeters(this.bandWidth, 'Bandbreite');
            }
            const codeWidth = Math.min(nominalCodeWidth, width);
            const textX = codeWidth + LABEL_GAP_MM;
            const textWidth = Math.max(1, width - textX);
            const blocks = [];
            if (this.showBrand && this.brandText.trim()) {
                const fontSize = Math.min(3.2, textWidth / 4);
                const lines = wrapLabelText(this.brandText.trim(), Math.max(1, Math.floor(textWidth / (fontSize * 0.56))));
                blocks.push({ value: this.brandText.trim(), lines, fontSize, height: lines.length * fontSize * 1.2, weight: '700' });
            }
            const idLine = this.idTemplate.replaceAll('{id}', String(item.id));
            if (this.showID) {
                const fontSize = Math.min(3.2, textWidth / 4);
                const lines = wrapLabelText(idLine, Math.max(1, Math.floor(textWidth / (fontSize * 0.56))));
                blocks.push({ value: idLine, lines, fontSize, height: lines.length * fontSize * 1.2, weight: '700' });
            }
            if (this.showTitle) blocks.push(this.titleBlock(item, textWidth));
            blocks.push(...this.locationBlocks(item, textWidth));
            let y = 0;
            const positionedBlocks = blocks.map((block, index) => {
                const positioned = { ...block, y: y + block.fontSize };
                y += block.height;
                if (index < blocks.length - 1) y += LABEL_GAP_MM;
                return positioned;
            });
            return { height: y, codeWidth, codeHeight: this.codeType === 'qr' ? codeWidth : Math.min(13, codeWidth), textX, textWidth, textBlocks: positionedBlocks };
        },

        payloadFor(item) {
            if (this.payloadMode === 'id') return String(item.id);
            if (!this.urlTemplate.includes('{id}')) throw new Error('Das URL-Template muss den Platzhalter {id} enthalten.');
            return this.urlTemplate.replaceAll('{id}', encodeURIComponent(item.id));
        },

        toggleItem(id, checked) {
            if (checked && !this.selectedIds.includes(id)) this.selectedIds = [...this.selectedIds, id];
            if (!checked) this.selectedIds = this.selectedIds.filter(value => value !== id);
        },

        toggleVisible(checked) {
            const visibleIds = new Set(this.filteredItems.map(item => item.id));
            this.selectedIds = checked
                ? [...new Set([...this.selectedIds, ...visibleIds])]
                : this.selectedIds.filter(id => !visibleIds.has(id));
        },

        renderCode(svg, payload, x, y, width, height) {
            if (this.codeType === 'qr') {
                const qr = qrcode(0, 'M');
                qr.addData(payload, 'Byte');
                qr.make();
                const qrMarkup = qr.createSvgTag(1, 0);
                const parsed = new DOMParser().parseFromString(qrMarkup, 'image/svg+xml').documentElement;
                if (parsed.localName !== 'svg' || parsed.querySelector('parsererror')) throw new Error('QR-Code konnte nicht erzeugt werden.');
                const nested = labelSvgElement('svg', {
                    x, y, width, height,
                    viewBox: `0 0 ${qr.getModuleCount()} ${qr.getModuleCount()}`,
                    preserveAspectRatio: 'xMidYMid meet',
                    'aria-label': `QR-Code ${payload}`
                });
                Array.from(parsed.childNodes).forEach(child => nested.appendChild(document.importNode(child, true)));
                svg.appendChild(nested);
                return;
            }

            const temporary = labelSvgElement('svg');
            try {
                JsBarcode(temporary, payload, {
                    format: 'CODE128',
                    displayValue: false,
                    margin: 0,
                    width: 2,
                    height: 80,
                    lineColor: '#111827',
                    background: '#ffffff'
                });
            } catch {
                throw new Error('Code-128 kann den Inhalt nicht codieren. Verwende eine ID oder ein ASCII-URL-Template.');
            }
            temporary.setAttribute('x', String(x));
            temporary.setAttribute('y', String(y));
            temporary.setAttribute('width', String(width));
            temporary.setAttribute('height', String(height));
            temporary.setAttribute('preserveAspectRatio', 'none');
            svg.appendChild(temporary);
        },

        createLabelSVG(item) {
            const { width, height } = this.outputDimensionsForItem(item);
            const svg = labelSvgElement('svg', {
                xmlns: SVG_NS,
                width: `${width}mm`,
                height: `${height}mm`,
                viewBox: `0 0 ${width} ${height}`,
                version: '1.1',
                role: 'img',
                'aria-label': `Etikett für ${item.name}`
            });
            svg.appendChild(labelSvgElement('rect', { x: 0, y: 0, width, height, fill: '#ffffff' }));
            const content = labelSvgElement('g');
            svg.appendChild(content);

            if (this.layout === 'vertical') {
                const flow = this.verticalFlow(item, width);
                if (flow.header) {
                    appendLabelText(content, flow.header.value, width / 2, flow.header.y, width, {
                        fontSize: flow.header.fontSize, fontWeight: flow.header.weight, anchor: 'middle', lines: flow.header.lines
                    });
                }
                this.renderCode(content, this.payloadFor(item), 0, flow.codeY, width, flow.codeHeight);
                flow.textBlocks.forEach(block => appendLabelText(content, block.value, width / 2, block.y, width, {
                    fontSize: block.fontSize, fontWeight: block.weight, anchor: 'middle', lines: block.lines
                }));
            } else {
                const flow = this.horizontalTextFlow(item, width);
                this.renderCode(content, this.payloadFor(item), 0, 0, flow.codeWidth, flow.codeHeight);
                flow.textBlocks.forEach(block => appendLabelText(content, block.value, flow.textX, block.y, flow.textWidth, {
                    fontSize: block.fontSize, fontWeight: block.weight, lines: block.lines
                }));
            }
            return svg;
        },

        renderPreview() {
            if (!this.$refs.preview) return;
            this.$refs.preview.replaceChildren();
            this.error = '';
            try {
                this.dimensionsForItem(this.previewItem || { id: 0, name: '' });
                if (!this.previewItem) return;
                this.$refs.preview.appendChild(this.createLabelSVG(this.previewItem));
            } catch (error) {
                this.error = error.message || 'Vorschau konnte nicht erstellt werden.';
            }
        },

        exportFilename(index, item) {
            return `${String(index + 1).padStart(3, '0')}_${cleanLabelFilename(item.name)}.svg`;
        },

        exportManifest() {
            try {
                const manifest = {
                    format: 'SVG',
                    target: 'Druckerunabhängiges SVG',
                    generatedAt: new Date().toISOString(),
                    settings: this.exportSettings(),
                    excludedIncompleteLocationCount: this.selectedItems.length - this.exportItems.length,
                    labels: this.exportItems.map((item, index) => ({
                        filename: this.exportFilename(index, item),
                        id: item.id,
                        name: item.name,
                        room: item.room,
                        shelf: item.shelf,
                        payload: this.payloadFor(item),
                        widthMm: this.outputDimensionsForItem(item).width,
                        heightMm: this.outputDimensionsForItem(item).height
                    }))
                };
                this.downloadBlob(new Blob([JSON.stringify(manifest, null, 2)], { type: 'application/json' }), 'labels_manifest.json');
            } catch (error) {
                this.error = error.message || 'Manifest konnte nicht erstellt werden.';
            }
        },

        exportSettings() {
            return {
                rollType: this.rollType,
                bandWidthMm: this.rollType === 'continuous' ? parseLabelMillimeters(this.bandWidth, 'Bandbreite') : null,
                lengthMode: this.lengthMode,
                fixedLengthMm: this.rollType === 'continuous' && this.lengthMode === 'fixed'
                    ? parseLabelMillimeters(this.fixedLength, 'Etikettenlänge')
                    : null,
                dieCutSize: this.rollType === 'diecut' ? this.dieCutSize : null,
                codeType: this.codeType,
                payloadMode: this.payloadMode,
                urlTemplate: this.urlTemplate,
                layout: this.layout,
                canvasOrientation: this.layout === 'horizontal' ? 'landscape' : 'portrait',
                showBrand: this.showBrand,
                brandText: this.brandText,
                showID: this.showID,
                idTemplate: this.idTemplate,
                showTitle: this.showTitle,
                titleMaxLines: Number(this.titleMaxLines),
                titleOverflow: this.titleOverflow,
                showRoom: this.showRoom,
                roomFontSizeMm: Number(this.roomFontSize),
                showShelf: this.showShelf,
                shelfFontSizeMm: Number(this.shelfFontSize),
                onlyCompleteLocations: this.onlyCompleteLocations,
                safetyMarginMm: 0
            };
        },

        async exportZip() {
            if (this.exporting || this.exportItems.length === 0) return;
            this.exporting = true;
            this.exportProgress = 0;
            this.error = '';
            try {
                const zip = new JSZip();
                const items = this.exportItems;
                for (let index = 0; index < items.length; index++) {
                    const item = items[index];
                    const svg = this.createLabelSVG(item);
                    zip.file(this.exportFilename(index, item), new XMLSerializer().serializeToString(svg));
                    this.exportProgress = Math.round(((index + 1) / items.length) * 80);
                    if (index % 10 === 0) await new Promise(resolve => requestAnimationFrame(resolve));
                }
                const blob = await zip.generateAsync(
                    { type: 'blob', compression: 'DEFLATE', compressionOptions: { level: 6 } },
                    metadata => { this.exportProgress = 80 + Math.round(metadata.percent * 0.2); }
                );
                this.downloadBlob(blob, 'labels_svg.zip');
                this.exportProgress = 100;
            } catch (error) {
                this.error = error.message || 'Export fehlgeschlagen. Das JSON-Manifest steht als Fallback bereit.';
            } finally {
                this.exporting = false;
            }
        },

        downloadBlob(blob, filename) {
            const url = URL.createObjectURL(blob);
            const link = document.createElement('a');
            link.href = url;
            link.download = filename;
            link.click();
            setTimeout(() => URL.revokeObjectURL(url), 1000);
        }
    }));
});
