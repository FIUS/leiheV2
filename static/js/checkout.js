document.addEventListener('alpine:init', () => {
    Alpine.data('checkoutFlow', () => ({
        checkoutStep: 1,
        cart: [],
        cartItems: [],
        checkoutData: { startDate: '', endDate: '', guestName: '', guestEmail: '' },
        bookingForSomeoneElse: false,
        bookingToken: '',
        errorMessage: '',
        isLoading: false,

        // Kalender-State & Kollisionen
        calendar: { month: new Date().getMonth(), year: new Date().getFullYear(), hoverDate: null },
        blockedPeriods: [],
        conflictingGameIds: [],
        conflictWarning: '',

        init() {
            const saved = localStorage.getItem('cart');
            if (saved) {
                try {
                    const parsed = JSON.parse(saved);
                    if (Array.isArray(parsed) && parsed.length > 0) {
                        if (typeof parsed[0] === 'object' && parsed[0] !== null && parsed[0].id) {
                            this.cartItems = parsed;
                            this.cart = parsed.map(item => item.id);
                        } else {
                            this.cart = parsed.map(Number).filter(n => !isNaN(n));
                            this.cartItems = this.cart.map(id => ({ id: id, name: 'Spiel ' + id, image: '' }));
                        }
                    }
                } catch (e) {
                    this.cart = []; this.cartItems = [];
                }
            }

            if (this.cart.length === 0) {
                window.location.href = '/games';
                return;
            }

            const root = document.getElementById('checkout-root');
            const userRole = root ? root.dataset.userRole : 'guest';
            if (userRole && userRole !== 'guest' && userRole !== '') {
                this.checkoutStep = 2;
            }

            // NEU: Lade die Buchungsdaten für alle Spiele im Korb
            this.loadBookings();
        },

        removeFromCart(itemID) {
            this.cartItems = this.cartItems.filter(item => Number(item.id) !== Number(itemID));
            this.cart = this.cart.filter(id => Number(id) !== Number(itemID));
            localStorage.setItem('cart', JSON.stringify(this.cartItems));

            this.blockedPeriods = this.blockedPeriods.filter(period => Number(period.gameId) !== Number(itemID));
            this.checkConflicts();

            if (this.cart.length === 0) {
                window.location.href = '/games';
            }
        },

        async loadBookings() {
            this.blockedPeriods = [];
            if (!this.cart || this.cart.length === 0) return;

            await Promise.all(this.cart.map(async (id) => {
                try {
                    const res = await fetch('/api/items/details?id=' + id);
                    if (!res.ok) return;
                    const data = await res.json();

                    if (Array.isArray(data.bookings)) {
                        data.bookings.forEach(b => {
                            const status = (b.Status || b.status || '').toLowerCase();
                            if (status !== 'returned' && status !== 'cancelled') {
                                const rawStart = b.StartDate || b.start_date;
                                const rawEnd = b.EndDate || b.end_date;
                                if (rawStart && rawEnd) {
                                    this.blockedPeriods.push({
                                        startStr: rawStart.substring(0, 10),
                                        endStr: rawEnd.substring(0, 10),
                                        start: new Date(rawStart.substring(0, 10)).setHours(0, 0, 0, 0),
                                        end: new Date(rawEnd.substring(0, 10)).setHours(0, 0, 0, 0),
                                        gameId: id,
                                        gameName: data.name
                                    });
                                }
                            }
                        });
                    }
                } catch (e) {
                    console.error("Fehler beim Laden der Buchungen:", e);
                }
            }));
            this.checkConflicts();
        },

        isDayBlocked(day) {
            const mStr = String(this.calendar.month + 1).padStart(2, '0');
            const dStr = String(day).padStart(2, '0');
            const checkStr = `${this.calendar.year}-${mStr}-${dStr}`;

            return this.blockedPeriods.some(p => checkStr >= p.startStr && checkStr <= p.endStr);
        },

        checkConflicts() {
            this.conflictingGameIds = [];
            this.conflictWarning = '';
            if (!this.checkoutData.startDate) return;

            const start = new Date(this.checkoutData.startDate).setHours(0,0,0,0);
            const end = this.checkoutData.endDate ? new Date(this.checkoutData.endDate).setHours(0,0,0,0) : start;

            const rangeStart = Math.min(start, end);
            const rangeEnd = Math.max(start, end);
            let conflictingNames = new Set();

            this.blockedPeriods.forEach(p => {
                // Prüfen ob sich die Zeiträume überschneiden
                if (Math.max(rangeStart, p.start) <= Math.min(rangeEnd, p.end)) {
                    this.conflictingGameIds.push(p.gameId);
                    conflictingNames.add(p.gameName);
                }
            });

            if (conflictingNames.size > 0) {
                this.conflictWarning = `Zeitraum nicht möglich wegen: ${Array.from(conflictingNames).join(', ')}. Bitte wähle ein anderes Datum.`;
            }
        },

        // --- Kalender Navigation & Klicks ---
        get checkoutBlankDays() {
            let firstDay = new Date(this.calendar.year, this.calendar.month, 1).getDay();
            return firstDay === 0 ? 6 : firstDay - 1;
        },

        get checkoutDaysInMonth() {
            return new Date(this.calendar.year, this.calendar.month + 1, 0).getDate();
        },

        isPast(day) {
            return new Date(this.calendar.year, this.calendar.month, day).setHours(0,0,0,0) < new Date().setHours(0,0,0,0);
        },

        isDateSelected(day) {
            const d = new Date(this.calendar.year, this.calendar.month, day).setHours(0,0,0,0);
            const start = this.checkoutData.startDate ? new Date(this.checkoutData.startDate).setHours(0,0,0,0) : null;
            const end = this.checkoutData.endDate ? new Date(this.checkoutData.endDate).setHours(0,0,0,0) : null;
            return d === start || d === end;
        },

        isDateInRange(day) {
            if (!this.checkoutData.startDate) return false;
            const d = new Date(this.calendar.year, this.calendar.month, day).setHours(0,0,0,0);
            const start = new Date(this.checkoutData.startDate).setHours(0,0,0,0);
            const end = this.checkoutData.endDate ? new Date(this.checkoutData.endDate).setHours(0,0,0,0) : (this.calendar.hoverDate ? this.calendar.hoverDate.getTime() : null);
            return start && end ? d > Math.min(start, end) && d < Math.max(start, end) : false;
        },

        handleDateClick(day) {
            if (this.isPast(day)) return;
            const selectedStr = `${this.calendar.year}-${String(this.calendar.month + 1).padStart(2, '0')}-${String(day).padStart(2, '0')}`;
            const selectedDate = new Date(this.calendar.year, this.calendar.month, day).setHours(0,0,0,0);

            if (!this.checkoutData.startDate || (this.checkoutData.startDate && this.checkoutData.endDate)) {
                this.checkoutData.startDate = selectedStr;
                this.checkoutData.endDate = '';
            } else {
                const start = new Date(this.checkoutData.startDate).setHours(0,0,0,0);
                if (selectedDate < start) {
                    this.checkoutData.endDate = this.checkoutData.startDate;
                    this.checkoutData.startDate = selectedStr;
                } else if (selectedDate === start) {
                    this.checkoutData.endDate = this.checkoutData.startDate;
                } else {
                    this.checkoutData.endDate = selectedStr;
                }
            }
            this.checkConflicts(); // Nach jedem Klick checken!
        },

        handleDateHover(day) {
            if (!this.checkoutData.startDate || this.checkoutData.endDate) {
                this.calendar.hoverDate = null;
                return;
            }
            if (!this.isPast(day)) {
                this.calendar.hoverDate = new Date(this.calendar.year, this.calendar.month, day);
            }
        },

        formatDate(dateStr) {
            if (!dateStr) return '-';
            return new Date(dateStr).toLocaleDateString('de-DE', { day: '2-digit', month: '2-digit', year: 'numeric' });
        },

        // --- Checkout Submit ---
        async submitCheckout() {
            this.errorMessage = '';
            if (this.conflictingGameIds.length > 0) {
                this.errorMessage = 'Bitte löse erst den Datumskonflikt.';
                return;
            }
            if (!this.checkoutData.startDate || !this.checkoutData.endDate) {
                this.errorMessage = 'Bitte wähle Start- und Enddatum im Kalender aus.';
                return;
            }

            this.isLoading = true;
            try {
                const res = await fetch('/api/checkout', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        games: this.cart,
                        startDate: this.checkoutData.startDate,
                        endDate: this.checkoutData.endDate,
                        guestName: this.bookingForSomeoneElse ? this.checkoutData.guestName : '',
                        guestEmail: this.bookingForSomeoneElse ? this.checkoutData.guestEmail : ''
                    })
                });

                const data = await res.json();
                if (res.ok) {
                    this.bookingToken = data.token;
                    this.checkoutStep = 3;
                    localStorage.removeItem('cart');
                } else if (res.status === 409 && data.conflicts) {
                    this.errorMessage = data.error + ' Betroffen: ' + data.conflicts.join(', ');
                } else {
                    this.errorMessage = data.error || 'Fehler beim Buchen. Details: ' + (data.details || '');
                }
            } catch (e) {
                this.errorMessage = 'Netzwerkfehler beim Absenden.';
            } finally {
                this.isLoading = false;
            }
        }
    }));
});