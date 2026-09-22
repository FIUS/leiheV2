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

        cart: [],
        cartOpen: false,
        gamesList: [],
        userRole: "guest",
        itemHistory: [],

        init() {
            // Read the cart metadata from the rendered game cards so it stays
            // available even when the page serializes other data attributes.
            const root = document.getElementById('games-root');
            if (root) {
                this.userRole = root.dataset.userRole || 'guest';
            }
            this.gamesList = Array.from(document.querySelectorAll('[data-game-id]')).map(game => ({
                id: Number(game.dataset.gameId),
                name: game.dataset.gameName || '',
                image: game.dataset.gameImage || ''
            }));
            if (this.gamesList.length === 0 && root) {
                try {
                    this.gamesList = JSON.parse(root.dataset.games || '[]');
                } catch (e) {
                    this.gamesList = [];
                }
            }

            // 2. Event-Listener für Sortierung & Initialisierung
            this.$watch('sortBy', () => this.sortItems());
            this.sortItems();
        },

        get cartDetails() {
            return this.cart
                .map(id => this.gamesList.find(g => Number(g.id) === Number(id)))
                .filter(Boolean);
        },

        addToCart(id, name, image) {
            const numericID = Number(id);
            if (!this.gamesList.some(game => Number(game.id) === numericID)) {
                this.gamesList.push({ id: numericID, name, image });
            }
            if (!this.cart.includes(numericID)) {
                this.cart.push(numericID);
            }
        },

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
            const checkDate = new Date(this.calendarYear, this.calendarMonth, day).setHours(0,0,0,0);
            for (let b of this.activeItem.bookings) {
                let start = new Date(b.start_date).setHours(0,0,0,0);
                let end = new Date(b.end_date).setHours(0,0,0,0);
                if (checkDate >= start && checkDate <= end) return true;
            }
            return false;
        },

        get blankDays() {
            let firstDay = new Date(this.calendarYear, this.calendarMonth, 1).getDay();
            return firstDay === 0 ? 6 : firstDay - 1;
        },

        get daysInMonth() {
            return new Date(this.calendarYear, this.calendarMonth + 1, 0).getDate();
        },

        proceedToCheckout() {
            localStorage.setItem('fius_cart', JSON.stringify(this.cart));
            window.location.href = '/checkout';
        }
    }));
});