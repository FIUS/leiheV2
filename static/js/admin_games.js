document.addEventListener('alpine:init', () => {
    Alpine.data('adminGamesManagement', () => ({
        showAddForm: false,
        editModalOpen: false,
        editGame: {},
        bggModalOpen: false,
        bggQuery: '',
        bggResults: [],
        isSearching: false,

        openEditGame(dataset) {
            this.editGame = {
                id: dataset.id,
                name: dataset.name,
                min_players: dataset.minPlayers,
                max_players: dataset.maxPlayers,
                min_playtime: dataset.minPlaytime,
                max_playtime: dataset.maxPlaytime,
                room: dataset.room,
                shelf: dataset.shelf
            };
            this.editModalOpen = true;
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
            try {
                const res = await fetch('/api/bgg/search?q=' + encodeURIComponent(this.bggQuery));
                if (res.ok) {
                    this.bggResults = (await res.json()) || [];
                }
            } catch (e) {
                console.error(e);
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