document.addEventListener('alpine:init', () => {
    Alpine.data('gamesCatalog', () => {
        let filteredGamesCacheKey = '';
        let filteredGamesCache = [];
        let filteredGamePositions = new Map();

        return ({
        search: "",
        sortBy: "name",
        pageSize: "30",
        currentPage: 1,
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
            const saved = localStorage.getItem('cart');
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

            ['search', 'sortBy', 'playersFilter', 'minDurationFilter', 'maxDurationFilter', 'selectedCategories', 'minRatingFilter', 'pageSize']
                .forEach((filter) => this.$watch(filter, () => { this.currentPage = 1; }));
            this.$watch('sortBy', () => this.$nextTick(() => this.sortItems()));
            this.$nextTick(() => this.sortItems());
        },

        // --- Korb-Logik ---

        isInCart(id) {
            return this.cart.some(item => item.id === id);
        },

        addToCart(id, name, image) {
            if (!this.isInCart(id)) {
                this.cart.push({ id: id, name: name, image: image || '' });
                localStorage.setItem('cart', JSON.stringify(this.cart));
            }
        },

        removeFromCart(id) {
            this.cart = this.cart.filter(item => item.id !== id);
            localStorage.setItem('cart', JSON.stringify(this.cart));
        },

        proceedToCheckout() {
            localStorage.setItem('cart', JSON.stringify(this.cart));
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
            this.pageSize = "30";
            this.currentPage = 1;
        },

        getFilteredGames() {
            const key = JSON.stringify([
                this.search.toLowerCase(),
                Number(this.playersFilter),
                Number(this.minDurationFilter),
                Number(this.maxDurationFilter),
                [...this.selectedCategories].sort(),
                Number(this.minRatingFilter),
                this.sortBy
            ]);
            if (key === filteredGamesCacheKey) return filteredGamesCache;

            const container = document.getElementById("games-container");
            const games = container ? Array.from(container.children) : [];
            const search = this.search.trim().toLowerCase();
            filteredGamesCache = games.filter((game) => {
                const data = game.dataset;
                const minPlayers = Number(data.minPlayers);
                const maxPlayers = Number(data.maxPlayers);
                const minPlaytime = Number(data.minPlaytime);
                const maxPlaytime = Number(data.maxPlaytime);
                const categories = data.categories ? data.categories.split(",").map((category) => category.trim()) : [];

                return (!search || data.name.toLowerCase().includes(search)) &&
                    (!this.playersFilter || (minPlayers <= this.playersFilter && maxPlayers >= this.playersFilter)) &&
                    (this.maxDurationFilter > 240
                        ? maxPlaytime >= this.minDurationFilter
                        : minPlaytime <= this.maxDurationFilter && maxPlaytime >= this.minDurationFilter) &&
                    (!this.selectedCategories.length || this.selectedCategories.some((category) => categories.includes(category))) &&
                    this.minRatingFilter === 0;
            }).map((game) => ({
                id: Number(game.dataset.gameId),
                name: game.dataset.name,
                duration: Number(game.dataset.duration),
                players: Number(game.dataset.players),
                rating: Number(game.dataset.rating)
            }));

            filteredGamesCache.sort((a, b) => {
                const aValue = a[this.sortBy];
                const bValue = b[this.sortBy];
                if (this.sortBy === "name") return a.name.localeCompare(b.name);
                if (this.sortBy === "rating") return bValue - aValue;
                return aValue - bValue;
            });
            filteredGamePositions = new Map(filteredGamesCache.map((game, index) => [game.id, index]));
            filteredGamesCacheKey = key;
            return filteredGamesCache;
        },

        sortItems() {
            const container = document.getElementById("games-container");
            if (!container) return;
            const items = Array.from(container.children);
            items.sort((a, b) => {
                const aValue = Number(a.dataset[this.sortBy]);
                const bValue = Number(b.dataset[this.sortBy]);
                if (this.sortBy === "name") return a.dataset.name.localeCompare(b.dataset.name);
                if (this.sortBy === "rating") return bValue - aValue;
                return aValue - bValue;
            });
            items.forEach((item) => container.appendChild(item));
            filteredGamesCacheKey = "";
        },

        isGameVisible(id) {
            const games = this.getFilteredGames();
            const index = filteredGamePositions.get(Number(id));
            if (index === undefined) return false;
            const pageSize = this.pageSize === "all" ? games.length : Number(this.pageSize);
            return index >= (this.currentPage - 1) * pageSize &&
                index < this.currentPage * pageSize;
        },

        get totalPages() {
            const games = this.getFilteredGames();
            const pageSize = this.pageSize === "all" ? games.length : Number(this.pageSize);
            return Math.max(1, Math.ceil(games.length / (pageSize || 1)));
        },

        get pageNumbers() {
            const first = Math.max(1, this.currentPage - 2);
            const last = Math.min(this.totalPages, first + 4);
            return Array.from({ length: last - first + 1 }, (_, index) => first + index);
        },

        get resultSummary() {
            const count = this.getFilteredGames().length;
            if (count === 0) return "Keine Spiele gefunden";
            const pageSize = this.pageSize === "all" ? count : Number(this.pageSize);
            const first = (this.currentPage - 1) * pageSize + 1;
            const last = Math.min(this.currentPage * pageSize, count);
            return `Spiele ${first}–${last} von ${count}`;
        },

        goToPage(page) {
            this.currentPage = Math.min(Math.max(1, page), this.totalPages);
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
        });
    });
});