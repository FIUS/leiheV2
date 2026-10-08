document.addEventListener('alpine:init', () => {
    Alpine.data('adminLocationManager', () => ({
        locations: [],
        selectedRoom: '',
        locationIndex: 0,
        items: [],
        search: '',
        selectedIDs: [],
        error: '',
        message: '',
        busy: false,

        init() {
            try {
                this.locations = JSON.parse(this.$el.dataset.locations || '[]');
                this.selectedRoom = String(this.$el.dataset.selectedRoom || '').trim();
            } catch (error) {
                this.error = 'Lagerorte konnten nicht geladen werden.';
                console.error('Invalid storage location configuration:', error);
                return;
            }
            if (this.locations.length === 0) {
                this.error = 'Es sind keine Lagerorte konfiguriert.';
                return;
            }
            this.loadItems();
        },

        get currentLocation() {
            return this.locations[this.locationIndex] || '';
        },

        get currentItems() {
            return this.items.filter(item => String(item.shelf || '').trim().toUpperCase() === this.currentLocation);
        },

        get roomItems() {
            return this.items.filter(item => this.normalize(item.room) === this.normalize(this.selectedRoom));
        },

        get roomShelves() {
            return [...new Set(this.roomItems.map(item => String(item.shelf || '').trim()))]
                .sort((left, right) => left.localeCompare(right, 'de', {numeric: true}));
        },

        get searchResults() {
            const query = this.normalize(this.search);
            if (!query) return [];
            return this.items
                .filter(item => this.normalize(`${item.name} ${item.id}`).includes(query))
                .slice(0, 50);
        },

        normalize(value) {
            return String(value || '').trim().toLocaleLowerCase('de').normalize('NFD').replace(/[\u0300-\u036f]/g, '');
        },

        isAtCurrentLocation(item) {
            return String(item.shelf || '').trim().toUpperCase() === this.currentLocation;
        },

        itemsForShelf(shelf) {
            return this.roomItems.filter(item => String(item.shelf || '').trim() === shelf);
        },

        isSelected(id) {
            return this.selectedIDs.includes(Number(id));
        },

        toggleItem(id, checked) {
            const itemID = Number(id);
            this.selectedIDs = checked
                ? [...new Set([...this.selectedIDs, itemID])]
                : this.selectedIDs.filter(selectedID => selectedID !== itemID);
        },

        selectLocation(index) {
            this.locationIndex = Number(index);
            this.selectedIDs = [];
            this.error = '';
            this.message = '';
        },

        goToLocation(offset) {
            this.selectLocation(Math.max(0, Math.min(this.locations.length - 1, this.locationIndex + offset)));
        },

        async loadItems() {
            this.error = '';
            try {
                const response = await fetch('/admin/api/locations/items');
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Spiele konnten nicht geladen werden.');
                this.items = data.items || [];
                if (this.selectedRoom && !this.items.some(item => this.normalize(item.room) === this.normalize(this.selectedRoom))) {
                    this.error = 'Dieser Raum ist nicht bekannt oder enthält keine Spiele.';
                }
            } catch (error) {
                this.error = error.message;
            }
        },

        async assignSelected() {
            const selected = this.items.filter(item => this.selectedIDs.includes(Number(item.id)) && !this.isAtCurrentLocation(item));
            if (selected.length === 0) {
                this.error = 'Wähle mindestens ein Spiel aus, das noch nicht in diesem Fach liegt.';
                return;
            }

            const moving = selected.filter(item => String(item.shelf || '').trim());
            if (moving.length > 0 && !window.confirm(`${moving.length} Spiel(e) werden von ihrem bisherigen Fach in ${this.currentLocation} verschoben. Fortfahren?`)) {
                return;
            }

            this.busy = true;
            this.error = '';
            this.message = '';
            try {
                const response = await fetch('/admin/api/locations/assign', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        item_ids: selected.map(item => Number(item.id)),
                        location: this.currentLocation,
                    }),
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Lagerorte konnten nicht gespeichert werden.');
                this.message = `${data.updated} Spiel(e) ${this.currentLocation} zugeordnet.`;
                this.selectedIDs = [];
                await this.loadItems();
            } catch (error) {
                this.error = error.message;
            } finally {
                this.busy = false;
            }
        },
    }));
});
