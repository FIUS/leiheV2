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
        scannedItemIDs: [],
        prebookingScanInput: '',
        spontaneousItems: [],
        itemSearch: '',
        itemResults: [],
        systemID: '',
        borrowerName: '',
        borrowerEmail: '',
        endDate: '',
        issueDeposit: '',
        spontaneousDeposit: '',
        requireDeposit: false,
        error: '',
        message: '',
        itemWarning: '',
        busy: false,
        returnScanInput: '',
        returnScannedItemIDs: {},

        init() {
            this.requireDeposit = this.$el.dataset.requireDeposit === 'true';
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
            this.itemWarning = '';
            if (mode === 'return' && this.returnBookings.length === 0) await this.searchBookings('return');
        },

        clearSelection() {
            this.selectedBooking = null;
            this.selectedItemIDs = [];
            this.scannedItemIDs = [];
            this.prebookingScanInput = '';
            this.issueDeposit = '';
            this.error = '';
            this.message = '';
            this.itemWarning = '';
        },

        cleanID(input) {
            return (window.extractGameID ? window.extractGameID(input) : String(input).replace(/\D/g, '')).trim();
        },

        async searchBookings(mode) {
            let query = mode === 'issue' ? this.issueSearch.trim() : this.returnSearch.trim();
            if (mode === 'return' && !query) {
                this.returnBookings = [];
                return;
            }
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
            this.scannedItemIDs = [];
            this.prebookingScanInput = '';
            this.error = '';
            this.message = '';
            this.itemWarning = '';
        },

        isPrebookedItemScanned(itemID) {
            return this.scannedItemIDs.includes(String(itemID)) || this.scannedItemIDs.includes(Number(itemID));
        },

        scanPrebookedGame(rawInput) {
            this.error = '';
            this.itemWarning = '';
            this.message = '';
            if (!this.selectedBooking) {
                this.error = 'Bitte zuerst eine Vorbestellung auswählen.';
                return;
            }
            const id = this.cleanID(rawInput);
            if (!id) {
                this.error = 'Bitte eine gültige Spiele-ID oder Barcode eingeben.';
                return;
            }

            const item = this.selectedBooking.items.find(i => String(i.id) === id);
            if (!item) {
                this.itemWarning = `Das Spiel mit ID ${id} gehört nicht zu dieser Vorbestellung (${this.selectedBooking.person_name})!`;
                return;
            }

            if (this.isPrebookedItemScanned(id)) {
                this.itemWarning = `Spiel „${item.name}“ (ID ${id}) wurde bereits erfasst.`;
                this.prebookingScanInput = '';
                return;
            }

            this.scannedItemIDs.push(String(id));
            // Make sure the item is also selected for checkout
            if (!this.selectedItemIDs.includes(String(id))) {
                this.selectedItemIDs.push(String(id));
            }
            this.message = `Spiel „${item.name}“ (ID ${id}) erfolgreich erfasst ✓`;
            this.prebookingScanInput = '';
        },

        openScannerForPrebooking() {
            if (window.openCameraScanner) {
                window.openCameraScanner((id) => this.scanPrebookedGame(id));
            }
        },

        canIssueSelectedBooking() {
            if (!this.selectedBooking || this.selectedItemIDs.length === 0) return false;
            if (this.requireDeposit && !this.issueDeposit.trim()) return false;
            // Every selected game MUST be physically scanned/captured!
            return this.selectedItemIDs.every(id => this.isPrebookedItemScanned(id));
        },

        async issueSelectedBooking() {
            if (!this.canIssueSelectedBooking()) return;
            await this.postJSON('/admin/api/ausgabe/prebooked', {
                booking_id: this.selectedBooking.id,
                item_ids: this.selectedItemIDs.map(Number),
                deposit: this.issueDeposit
            }, 'Ausgabe abgeschlossen.');
            if (!this.error) {
                this.clearSelection();
                this.issueSearch = '';
                await this.searchBookings('issue');
            }
        },

        async searchItems() {
            this.itemWarning = '';
            const query = this.itemSearch.trim();
            if (!query) {
                this.itemResults = [];
                return;
            }
            try {
                const params = new URLSearchParams({ q: query, end_date: this.endDate });
                const response = await fetch(`/admin/api/ausgabe/items?${params}`);
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Spiele konnten nicht gesucht werden');
                this.itemResults = data.items || [];
            } catch (error) {
                this.error = error.message;
            }
        },

        async lookupSystemID(rawInput = null) {
            const raw = rawInput !== null ? rawInput : this.systemID;
            const id = this.cleanID(raw);
            if (!id) {
                this.error = 'Bitte eine gültige Spiele-ID eingeben';
                return;
            }
            this.error = '';
            this.itemWarning = '';
            try {
                const params = new URLSearchParams({ id, end_date: this.endDate });
                const response = await fetch(`/admin/api/ausgabe/system-id?${params}`);
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Spiel konnte nicht gefunden werden');
                if (!data.item.available) {
                    this.itemWarning = data.item.warning;
                    return;
                }
                this.addSpontaneousItem(data.item);
                this.systemID = '';
            } catch (error) {
                this.error = error.message;
            }
        },

        openScannerForSpontaneous() {
            if (window.openCameraScanner) {
                window.openCameraScanner((id) => this.lookupSystemID(id));
            }
        },

        addSpontaneousItem(item) {
            this.error = '';
            this.itemWarning = '';
            if (!item.available) {
                this.itemWarning = item.warning;
                return;
            }
            if (this.isItemSelected(item.id)) {
                this.error = 'Dieses Spiel ist bereits ausgewählt.';
                return;
            }
            this.spontaneousItems.push(item);
            this.message = `Spiel „${item.name}“ hinzugefügt ✓`;
        },

        isItemSelected(itemID) {
            return this.spontaneousItems.some(item => Number(item.id) === Number(itemID));
        },

        removeSpontaneousItem(itemID) {
            this.spontaneousItems = this.spontaneousItems.filter(item => Number(item.id) !== Number(itemID));
        },

        async refreshSelectedItems() {
            this.itemWarning = '';
            await Promise.all(this.spontaneousItems.map(async item => {
                try {
                    const params = new URLSearchParams({ id: String(item.id), end_date: this.endDate });
                    const response = await fetch(`/admin/api/ausgabe/system-id?${params}`);
                    const data = await response.json();
                    if (!response.ok) throw new Error(data.error || 'Spielverfügbarkeit konnte nicht geprüft werden');
                    Object.assign(item, data.item);
                } catch (error) {
                    this.error = error.message;
                }
            }));
        },

        async refreshAvailability() {
            await Promise.all([this.refreshSelectedItems(), this.searchItems()]);
        },

        async createSpontaneousLoan() {
            if (this.spontaneousItems.length === 0 || this.spontaneousItems.some(item => !item.available)) return;
            await this.postJSON('/admin/api/ausgabe/spontaneous', {
                item_ids: this.spontaneousItems.map(item => Number(item.id)),
                name: this.borrowerName,
                email: this.borrowerEmail,
                deposit: this.spontaneousDeposit,
                end_date: this.endDate
            }, 'Ausleihe wurde ausgegeben.');
            if (!this.error) {
                this.spontaneousItems = [];
                this.borrowerName = '';
                this.borrowerEmail = '';
                this.spontaneousDeposit = '';
                await this.searchBookings('return');
            }
        },

        isReturnItemScanned(bookingID, itemID) {
            const list = this.returnScannedItemIDs[bookingID] || [];
            return list.includes(Number(itemID)) || list.includes(String(itemID));
        },

        getReturnScannedCount(booking) {
            const list = this.returnScannedItemIDs[booking.id] || [];
            return (booking.items || []).filter(item => list.includes(Number(item.id)) || list.includes(String(item.id))).length;
        },

        isBookingReadyForReturn(booking) {
            if (!booking || !booking.items || booking.items.length === 0) return false;
            const list = this.returnScannedItemIDs[booking.id] || [];
            return booking.items.every(item => list.includes(Number(item.id)) || list.includes(String(item.id)));
        },

        async scanReturnGame(rawInput, targetBooking = null) {
            this.error = '';
            this.message = '';
            this.itemWarning = '';
            const id = this.cleanID(rawInput);
            if (!id) {
                this.error = 'Bitte eine gültige Spiele-ID oder Barcode eingeben.';
                return;
            }

            let booking = targetBooking;
            if (!booking) {
                // Look in currently loaded returnBookings first
                booking = this.returnBookings.find(b => b.items && b.items.some(i => String(i.id) === id));
                if (!booking) {
                    // Fetch from server by searching for this game ID
                    try {
                        const response = await fetch(`/admin/api/ausgabe/bookings?mode=return&q=${encodeURIComponent(id)}`);
                        const data = await response.json();
                        if (response.ok && data.bookings && data.bookings.length > 0) {
                            this.returnBookings = data.bookings;
                            booking = this.returnBookings.find(b => b.items && b.items.some(i => String(i.id) === id)) || this.returnBookings[0];
                        }
                    } catch (_) {}
                }
            }

            if (!booking) {
                this.error = `Keine aktive Ausleihe für Spiel-ID ${id} gefunden.`;
                return;
            }

            const item = booking.items.find(i => String(i.id) === id);
            if (!item) {
                this.error = `Spiel mit ID ${id} gehört nicht zur Ausleihe von ${booking.person_name}!`;
                return;
            }

            if (!this.returnScannedItemIDs[booking.id]) {
                this.returnScannedItemIDs[booking.id] = [];
            }

            if (this.isReturnItemScanned(booking.id, id)) {
                this.message = `Spiel „${item.name}“ (ID ${id}) wurde bereits erfasst.`;
                this.returnScanInput = '';
                return;
            }

            this.returnScannedItemIDs[booking.id].push(Number(id));
            this.returnScanInput = '';

            if (this.isBookingReadyForReturn(booking)) {
                this.message = `Alle Spiele von ${booking.person_name} erfasst ✓ Rücknahme kann abgeschlossen werden.`;
            } else {
                const count = this.getReturnScannedCount(booking);
                this.message = `Spiel „${item.name}“ für ${booking.person_name} erfasst (${count} von ${booking.items.length} Spielen erfasst) ✓`;
            }
        },

        openScannerForReturn(booking = null) {
            if (window.openCameraScanner) {
                window.openCameraScanner((id) => this.scanReturnGame(id, booking));
            }
        },

        async returnBooking(booking) {
            if (!this.isBookingReadyForReturn(booking)) {
                this.error = `Bitte vor der Rücknahme alle ${booking.items.length} Spiele physisch per Scanner erfassen.`;
                return;
            }
            if (!window.confirm(`Ausleihe von ${booking.person_name} wirklich zurücknehmen? Alle ${booking.items.length} Spiele wurden erfasst.`)) return;
            await this.postJSON('/admin/api/ausgabe/return', { booking_id: booking.id }, 'Buchung zurückgenommen; die Spiele sind wieder verfügbar.');
            if (!this.error) {
                delete this.returnScannedItemIDs[booking.id];
                await this.searchBookings('return');
            }
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
