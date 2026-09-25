document.addEventListener('alpine:init', () => {
    Alpine.data('adminGamesManagement', () => ({
        showAddForm: false,
        editModalOpen: false,
        editGame: {},
        addCategories: [],
        categorySearch: '',
        editCategorySearch: '',
        categoryPicker: '',
        bggModalOpen: false,
        bggQuery: '',
        bggResults: [],
        isSearching: false,
        bggError: '',

        openEditGame(dataset) {
            this.editGame = {
                id: dataset.id,
                name: dataset.name,
                min_players: dataset.minPlayers,
                max_players: dataset.maxPlayers,
                min_playtime: dataset.minPlaytime,
                max_playtime: dataset.maxPlaytime,
                room: dataset.room,
                shelf: dataset.shelf,
                categories: dataset.categories ? dataset.categories.split(',').map(value => value.trim()).filter(Boolean) : []
            };
            this.editModalOpen = true;
        },

        categoryMatches(query, selected) {
            const normalizedQuery = (query || '').trim().toLowerCase();
            const categories = Array.from(document.querySelectorAll('#category-options option'))
                .map(option => option.value);
            return categories.filter(category => {
                return (!normalizedQuery || category.toLowerCase().includes(normalizedQuery))
                    && !(selected || []).some(value => value.toLowerCase() === category.toLowerCase());
            }).map(name => ({name}));
        },

        addCategory(target, value) {
            const name = (value || '').trim();
            if (!name) return;
            const categories = target === 'edit' ? this.editGame.categories : this.addCategories;
            if (!categories.some(category => category.toLowerCase() === name.toLowerCase())) {
                categories.push(name);
            }
            if (target === 'edit') this.editGame.categories = categories;
            if (target === 'edit') {
                this.editCategorySearch = '';
            } else {
                this.categorySearch = '';
            }
            this.categoryPicker = target;
        },

        removeCategory(target, value) {
            if (target === 'edit') {
                this.editGame.categories = this.editGame.categories.filter(category => category !== value);
            } else {
                this.addCategories = this.addCategories.filter(category => category !== value);
            }
        },

        openBGGModal(name) {
            this.bggQuery = name;
            this.bggResults = [];
            this.bggModalOpen = true;
            this.searchBGG();
        },

        async searchBGG() {
            if (this.bggQuery.length < 2) return;
            this.isSearching = true;
            this.bggResults = [];
            this.bggError = '';
            try {
                const res = await fetch('/api/bgg/search?q=' + encodeURIComponent(this.bggQuery));
                if (res.ok) {
                    this.bggResults = (await res.json()) || [];
                } else {
                    const data = await res.json().catch(() => ({}));
                    this.bggError = data.detail
                        ? `${data.error || 'BGG API Fehler'} (${data.status || res.status}): ${data.detail}`
                        : (data.error || `BGG API Fehler (${res.status})`);
                }
            } catch (e) {
                this.bggError = 'BGG API ist nicht erreichbar.';
            }
            this.isSearching = false;
        },

        async fetchBGGDetails(bggId) {
            try {
                const res = await fetch('/api/bgg/details?id=' + bggId);
                if (res.ok) {
                    const data = await res.json();
                    this.editGame.min_players = data.min_players;
                    this.editGame.max_players = data.max_players;
                    this.editGame.min_playtime = data.min_playtime;
                    this.editGame.max_playtime = data.max_playtime;
                    this.bggModalOpen = false;
                    this.editModalOpen = true;
                }
            } catch (e) {
                console.error(e);
            }
        }
    }));
});