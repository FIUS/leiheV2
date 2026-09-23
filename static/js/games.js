document.addEventListener('alpine:init', () => {
    Alpine.data('gamesCatalog', () => ({
        search: "",
        sortBy: "name",
        viewMode: "grid",
        playersFilter: 0,
        minDurationFilter: 5,
        maxDurationFilter: 245,
        selectedCategories: [],
        minRatingFilter: 0,

        cart: [], // Speichert jetzt ausschließlich Objekte: {id, name, image}
        cartOpen: false,
        userRole: "guest",
        itemHistory: [],

        init() {
            const root = document.getElementById('games-root');
            if (root) {
                this.userRole = root.dataset.userRole || 'guest';
            }

            // Korb aus dem LocalStorage laden & alte kaputte Daten rausfiltern
            const saved = localStorage.getItem('fius_cart');
            if (saved) {
                try {
                    const parsed = JSON.parse(saved);
                    if (Array.isArray(parsed)) {
                        this.cart = parsed.map(item => {
                            // Nur gültige Objekte übernehmen
                            if (typeof item === 'object' && item !== null && item.id) return item;
                            return null;
                        }).filter(Boolean);
                    }
                } catch (e) {
                    this.cart = [];
                }
            }

            this.$watch('sortBy', () => this.sortItems());
            this.sortItems();
        },

        // --- Korb-Logik ---

        isInCart(id) {
            return this.cart.some(item => item.id === id);
        },

        addToCart(id, name, image) {
            if (!this.isInCart(id)) {
                this.cart.push({ id: id, name: name, image: image || '' });
                localStorage.setItem('fius_cart', JSON.stringify(this.cart));
            }
        },

        removeFromCart(id) {
            this.cart = this.cart.filter(item => item.id !== id);
            localStorage.setItem('fius_cart', JSON.stringify(this.cart));
        },

        proceedToCheckout() {
            localStorage.setItem('fius_cart', JSON.stringify(this.cart));
            window.location.href = '/checkout';
        },

        // --- Filter & Ansichten ---

        validateMinDuration() {
            if (parseInt(this.minDurationFilter) > parseInt(this.maxDurationFilter)) {
                this.maxDurationFilter = this.minDurationFilter;
            }
        },

        validateMaxDuration() {
            if (parseInt(this.maxDurationFilter) < parseInt(this.minDurationFilter)) {
                this.minDurationFilter = this.maxDurationFilter;
            }
        },

        resetFilters() {
            this.search = "";
            this.playersFilter = 0;
            this.minDurationFilter = 5;
            this.maxDurationFilter = 245;
            this.selectedCategories = [];
            this.minRatingFilter = 0;
            this.sortBy = "name";
        },

        sortItems() {
            this.$nextTick(() => {
                let container = document.getElementById("games-container");
                if(!container) return;
                let items = Array.from(container.children);
                items.sort((a, b) => {
                    let vA = a.dataset[this.sortBy];
                    let vB = b.dataset[this.sortBy];
                    if (this.sortBy === "name") return vA.localeCompare(vB);
                    if (this.sortBy === "rating") return parseFloat(vB || 0) - parseFloat(vA || 0);
                    return parseFloat(vA || 0) - parseFloat(vB || 0);
                });
                items.forEach(i => container.appendChild(i));
            });
        },

        // --- Detail-Modal & Kalender ---

        detailModalOpen: false,
        activeItem: null,
        isLoadingDetails: false,
        activeImageIndex: 0,
        calendarMonth: new Date().getMonth(),
        calendarYear: new Date().getFullYear(),

        async openDetails(id) {
            this.isLoadingDetails = true;
            this.detailModalOpen = true;
            this.activeItem = null;
            this.activeImageIndex = 0;
            this.calendarMonth = new Date().getMonth();
            this.calendarYear = new Date().getFullYear();

            try {
                const res = await fetch("/api/items/details?id=" + id);
                if (res.ok) {
                    this.activeItem = await res.json();
                    if (this.userRole === "admin" || this.userRole === "maintainer") {
                        const histRes = await fetch("/api/items/history?id=" + id);
                        if (histRes.ok) this.itemHistory = await histRes.json();
                    }
                }
            } catch (e) {
                console.error("Details Fehler", e);
            }
            this.isLoadingDetails = false;
        },

        isBooked(day) {
            if (!this.activeItem || !this.activeItem.bookings) return false;
            const mStr = String(this.calendarMonth + 1).padStart(2, '0');
            const dStr = String(day).padStart(2, '0');
            const checkStr = `${this.calendarYear}-${mStr}-${dStr}`;

            for (let b of this.activeItem.bookings) {
                // Berücksichtigt StartDate / EndDate (PascalCase aus Go) und start_date / end_date
                const rawStart = b.StartDate || b.start_date;
                const rawEnd = b.EndDate || b.end_date;
                const status = (b.Status || b.status || '').toLowerCase();

                if (status === 'returned' || status === 'cancelled') continue;

                if (rawStart && rawEnd) {
                    const startStr = rawStart.substring(0, 10);
                    const endStr = rawEnd.substring(0, 10);
                    if (checkStr >= startStr && checkStr <= endStr) {
                        return true;
                    }
                }
            }
            return false;
        },

        get blankDays() {
            let firstDay = new Date(this.calendarYear, this.calendarMonth, 1).getDay();
            return firstDay === 0 ? 6 : firstDay - 1;
        },

        get daysInMonth() {
            return new Date(this.calendarYear, this.calendarMonth + 1, 0).getDate();
        }
    }));
});