const SVG_NS = 'http://www.w3.org/2000/svg';
const LABEL_MARGIN_MM = 1.5;

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
    const lines = wrapLabelText(value, maxCharacters);
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

document.addEventListener('alpine:init', () => {
    Alpine.data('barcodeDesigner', () => ({
        rollType: 'continuous',
        bandWidth: 29,
        lengthMode: 'dynamic',
        fixedLength: 60,
        dieCutSize: '29x90',
        codeType: 'qr',
        payloadMode: 'id',
        urlTemplate: 'https://verleih.fius.de/{id}',
        layout: 'vertical',
        showBrand: true,
        brandText: 'FIUS Verleih',
        showID: true,
        idTemplate: 'ID: #{id}',
        showTitle: true,
        items: [],
        categories: [],
        selectedIds: [],
        search: '',
        categoryFilter: '',
        exporting: false,
        exportProgress: 0,
        error: '',

        async init() {
            const updatePreview = () => this.renderPreview();
            [
                'rollType', 'bandWidth', 'lengthMode', 'fixedLength', 'dieCutSize',
                'codeType', 'payloadMode', 'urlTemplate', 'layout', 'showBrand',
                'brandText', 'showID', 'idTemplate', 'showTitle', 'selectedIds'
            ].forEach((key) => this.$watch(key, updatePreview));

            try {
                const response = await fetch('/admin/api/etiketten/spiele', {
                    headers: { Accept: 'application/json' }
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Spiele konnten nicht geladen werden.');
                this.items = data.items || [];
                this.categories = [...new Set(this.items.flatMap(item =>
                    (item.categories || '').split(',').map(category => category.trim()).filter(Boolean)
                ))].sort((a, b) => a.localeCompare(b, 'de'));
                this.renderPreview();
            } catch (error) {
                this.error = error.message || 'Spiele konnten nicht geladen werden.';
            }
        },

        get filteredItems() {
            const search = this.search.trim().toLocaleLowerCase('de');
            return this.items.filter(item =>
                (!search || `${item.name} ${item.id}`.toLocaleLowerCase('de').includes(search)) &&
                (!this.categoryFilter || (item.categories || '').split(',').map(value => value.trim()).includes(this.categoryFilter))
            );
        },

        get selectedItems() {
            const selected = new Set(this.selectedIds);
            return this.items.filter(item => selected.has(item.id));
        },

        get previewItem() {
            return this.selectedItems[0] || null;
        },

        get allVisibleSelected() {
            return this.filteredItems.length > 0 && this.filteredItems.every(item => this.selectedIds.includes(item.id));
        },

        get labelDimensions() {
            const item = this.previewItem || { id: 0, name: '' };
            return this.dimensionsForItem(item);
        },

        dimensionsForItem(item) {
            if (this.rollType === 'diecut') {
                const [width, height] = this.dieCutSize.split('x').map(Number);
                return { width, height };
            }
            const width = Number(this.bandWidth);
            if (this.lengthMode === 'fixed') {
                return { width, height: Math.max(20, Math.min(500, Number(this.fixedLength) || 60)) };
            }
            const titleLines = this.showTitle ? wrapLabelText(item.name || '', Math.max(4, Math.floor((width - 3) / 2))).length : 0;
            const textHeight = (this.showBrand ? 6 : 0) + (this.showID ? 6 : 0) + titleLines * 4 + 4;
            const codeHeight = this.codeType === 'qr' ? Math.max(12, width - 7) : 16;
            const height = this.layout === 'vertical'
                ? Math.max(22, Math.ceil(textHeight + codeHeight + 5))
                : Math.max(22, Math.ceil(width * 0.7));
            return { width, height: Math.min(height, 160) };
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
                    preserveAspectRatio: 'none',
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
            const dimensions = this.dimensionsForItem(item);
            const { width, height } = dimensions;
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

            const margin = LABEL_MARGIN_MM;
            const content = labelSvgElement('g');
            svg.appendChild(content);
            let top = margin + 2.5;
            const maxTextWidth = width - margin * 2;
            if (this.showBrand && this.brandText.trim()) {
                appendLabelText(content, this.brandText.trim(), width / 2, top, maxTextWidth, {
                    fontSize: Math.min(3.4, maxTextWidth / 8), fontWeight: '700', anchor: 'middle'
                });
                top += 4.5;
            }

            const idLine = this.idTemplate.replaceAll('{id}', String(item.id));
            const titleLines = this.showTitle ? wrapLabelText(item.name, Math.max(4, Math.floor(maxTextWidth / 2))) : [];
            const titleHeight = titleLines.length * 4;
            const idHeight = this.showID ? 4.5 : 0;
            const remainingHeight = Math.max(8, height - top - margin - idHeight - titleHeight - 2);

            if (this.layout === 'vertical') {
                const codeSize = this.codeType === 'qr'
                    ? Math.min(width - margin * 2, remainingHeight, Math.max(10, height * 0.55))
                    : Math.min(width - margin * 2, Math.max(8, remainingHeight * 0.65));
                const codeHeight = this.codeType === 'qr' ? codeSize : Math.min(13, remainingHeight);
                const codeY = top + Math.max(0, (remainingHeight - codeHeight) / 2);
                this.renderCode(svg, this.payloadFor(item), (width - codeSize) / 2, codeY, codeSize, codeHeight);
                let textY = top + remainingHeight + 3;
                if (this.showID) {
                    appendLabelText(content, idLine, width / 2, textY, maxTextWidth, { fontSize: 3.2, fontWeight: '700', anchor: 'middle' });
                    textY += 4.5;
                }
                if (this.showTitle) appendLabelText(content, item.name, width / 2, textY, maxTextWidth, { fontSize: 2.8, anchor: 'middle' });
            } else {
                const codeWidth = Math.max(7, Math.min(width * 0.42, height * 0.62));
                const codeHeight = this.codeType === 'qr' ? Math.min(codeWidth, remainingHeight) : Math.min(15, remainingHeight);
                const codeX = margin;
                const codeY = top + Math.max(0, (remainingHeight - codeHeight) / 2);
                this.renderCode(svg, this.payloadFor(item), codeX, codeY, codeWidth, codeHeight);
                const textX = codeX + codeWidth + 2;
                const textWidth = Math.max(4, width - margin - textX);
                let textY = top + 3;
                if (this.showID) {
                    appendLabelText(content, idLine, textX, textY, textWidth, { fontSize: Math.min(3.2, textWidth / 4), fontWeight: '700' });
                    textY += 5;
                }
                if (this.showTitle) appendLabelText(content, item.name, textX, textY, textWidth, { fontSize: Math.min(3, textWidth / 4) });
            }
            return svg;
        },

        renderPreview() {
            if (!this.$refs.preview) return;
            this.$refs.preview.replaceChildren();
            this.error = '';
            if (!this.previewItem) return;
            try {
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
                    printer: 'Brother QL series',
                    generatedAt: new Date().toISOString(),
                    settings: this.exportSettings(),
                    labels: this.selectedItems.map((item, index) => ({
                        filename: this.exportFilename(index, item),
                        id: item.id,
                        name: item.name,
                        categories: item.categories ? item.categories.split(',').map(value => value.trim()).filter(Boolean) : [],
                        payload: this.payloadFor(item),
                        widthMm: this.dimensionsForItem(item).width,
                        heightMm: this.dimensionsForItem(item).height
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
                bandWidthMm: Number(this.bandWidth),
                lengthMode: this.lengthMode,
                fixedLengthMm: Number(this.fixedLength),
                dieCutSize: this.dieCutSize,
                codeType: this.codeType,
                payloadMode: this.payloadMode,
                urlTemplate: this.urlTemplate,
                layout: this.layout,
                showBrand: this.showBrand,
                brandText: this.brandText,
                showID: this.showID,
                idTemplate: this.idTemplate,
                showTitle: this.showTitle,
                safetyMarginMm: LABEL_MARGIN_MM
            };
        },

        async exportZip() {
            if (this.exporting || this.selectedItems.length === 0) return;
            this.exporting = true;
            this.exportProgress = 0;
            this.error = '';
            try {
                const zip = new JSZip();
                const items = this.selectedItems;
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
