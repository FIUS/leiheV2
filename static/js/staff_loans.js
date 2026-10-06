document.addEventListener('alpine:init', () => {
    Alpine.data('staffLoans', () => ({
        mode: window.location.hash === '#return' ? 'return' : 'issue',
        issueSource: 'prebooked',
        issueSearch: '',
        returnSearch: '',
        issueBookings: [],
        returnBookings: [],
        selectedBooking: null,
        selectedItemIDs: [],
        spontaneousItems: [],
        barcode: '',
        borrowerName: '',
        borrowerEmail: '',
        endDate: '',
        error: '',
        message: '',
        busy: false,
        cameraActive: false,
        cameraStream: null,
        barcodeDetector: null,

        init() {
            const today = new Date();
            const returnDate = new Date(today.getFullYear(), today.getMonth(), today.getDate() + 7);
            this.endDate = `${returnDate.getFullYear()}-${String(returnDate.getMonth() + 1).padStart(2, '0')}-${String(returnDate.getDate()).padStart(2, '0')}`;
            this.searchBookings('issue');
            if (this.mode === 'return') this.searchBookings('return');
            window.addEventListener('hashchange', () => this.setMode(window.location.hash === '#return' ? 'return' : 'issue'));
        },

        async setMode(mode) {
            this.mode = mode;
            window.location.hash = mode;
            this.error = '';
            this.message = '';
            if (mode === 'return' && this.returnBookings.length === 0) await this.searchBookings('return');
        },

        clearSelection() {
            this.selectedBooking = null;
            this.selectedItemIDs = [];
            this.error = '';
            this.message = '';
        },

        async searchBookings(mode) {
            const query = mode === 'issue' ? this.issueSearch : this.returnSearch;
            try {
                const response = await fetch(`/admin/api/ausgabe/bookings?mode=${mode}&q=${encodeURIComponent(query)}`);
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Buchungen konnten nicht gesucht werden');
                if (mode === 'issue') this.issueBookings = data.bookings || [];
                else this.returnBookings = data.bookings || [];
            } catch (error) {
                this.error = error.message;
            }
        },

        selectBooking(booking) {
            this.selectedBooking = booking;
            this.selectedItemIDs = booking.items.map(item => String(item.id));
            this.error = '';
            this.message = '';
        },

        async issueSelectedBooking() {
            if (!this.selectedBooking || this.selectedItemIDs.length === 0) return;
            await this.postJSON('/admin/api/ausgabe/prebooked', {
                booking_id: this.selectedBooking.id,
                item_ids: this.selectedItemIDs.map(Number)
            }, 'Ausgabe abgeschlossen.');
            if (!this.error) {
                this.clearSelection();
                this.issueSearch = '';
                await this.searchBookings('issue');
            }
        },

        async lookupBarcode() {
            const code = this.barcode.trim();
            if (!code) return;
            this.error = '';
            try {
                const response = await fetch(`/admin/api/ausgabe/barcode?code=${encodeURIComponent(code)}`);
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Spiel konnte nicht gefunden werden');
                if (this.spontaneousItems.some(item => Number(item.id) === Number(data.item.id))) {
                    throw new Error('Dieses Spiel ist bereits ausgewählt');
                }
                this.spontaneousItems.push(data.item);
                this.barcode = '';
            } catch (error) {
                this.error = error.message;
            }
        },

        removeSpontaneousItem(itemID) {
            this.spontaneousItems = this.spontaneousItems.filter(item => Number(item.id) !== Number(itemID));
        },

        async toggleCamera() {
            if (this.cameraActive) {
                this.stopCamera();
                return;
            }
            if (!navigator.mediaDevices?.getUserMedia || !('BarcodeDetector' in window)) {
                this.error = 'Dieser Browser unterstützt keinen Kamerascanner. Bitte den Barcode manuell eingeben.';
                return;
            }
            try {
                this.barcodeDetector = new window.BarcodeDetector({ formats: ['ean_13', 'ean_8', 'upc_a', 'upc_e', 'code_128'] });
                this.cameraStream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: { ideal: 'environment' } } });
                this.$refs.video.srcObject = this.cameraStream;
                await this.$refs.video.play();
                this.cameraActive = true;
                this.scanFrame();
            } catch (error) {
                this.stopCamera();
                this.error = `Kamera konnte nicht gestartet werden: ${error.message}`;
            }
        },

        async scanFrame() {
            if (!this.cameraActive || !this.barcodeDetector) return;
            try {
                const barcodes = await this.barcodeDetector.detect(this.$refs.video);
                if (barcodes.length > 0) {
                    this.barcode = barcodes[0].rawValue;
                    this.stopCamera();
                    await this.lookupBarcode();
                    return;
                }
            } catch (error) {
                this.stopCamera();
                this.error = `Barcode konnte nicht gelesen werden: ${error.message}`;
                return;
            }
            window.setTimeout(() => this.scanFrame(), 250);
        },

        stopCamera() {
            this.cameraActive = false;
            if (this.cameraStream) {
                this.cameraStream.getTracks().forEach(track => track.stop());
                this.cameraStream = null;
            }
            if (this.$refs.video) this.$refs.video.srcObject = null;
        },

        async createSpontaneousLoan() {
            if (this.spontaneousItems.length === 0) return;
            await this.postJSON('/admin/api/ausgabe/spontaneous', {
                item_ids: this.spontaneousItems.map(item => Number(item.id)),
                name: this.borrowerName,
                email: this.borrowerEmail,
                end_date: this.endDate
            }, 'Spontanausleihe wurde ausgegeben.');
            if (!this.error) {
                this.spontaneousItems = [];
                this.borrowerName = '';
                this.borrowerEmail = '';
                await this.searchBookings('return');
            }
        },

        async returnBooking(booking) {
            if (!window.confirm(`Ausleihe von ${booking.person_name} wirklich zurücknehmen?`)) return;
            await this.postJSON('/admin/api/ausgabe/return', { booking_id: booking.id }, 'Buchung zurückgenommen; die Spiele sind wieder verfügbar.');
            if (!this.error) await this.searchBookings('return');
        },

        async postJSON(url, payload, successMessage) {
            this.error = '';
            this.message = '';
            this.busy = true;
            try {
                const response = await fetch(url, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload)
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Aktion konnte nicht abgeschlossen werden');
                this.message = data.token ? `${successMessage} Buchungscode: ${data.token}` : successMessage;
                if (data.warning) this.message += ` Hinweis: ${data.warning}.`;
            } catch (error) {
                this.error = error.message;
            } finally {
                this.busy = false;
            }
        }
    }));
});
