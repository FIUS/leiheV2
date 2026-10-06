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
        itemSearch: '',
        itemResults: [],
        systemID: '',
        borrowerName: '',
        borrowerEmail: '',
        endDate: '',
        error: '',
        message: '',
        itemWarning: '',
        busy: false,

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

        async lookupSystemID() {
            const id = this.systemID.trim();
            if (!id) return;
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
                end_date: this.endDate
            }, 'Ausleihe wurde ausgegeben.');
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
