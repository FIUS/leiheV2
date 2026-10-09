document.addEventListener('alpine:init', () => {
    Alpine.data('adminLocationManager', () => ({
        locations: [],
        rooms: [],
        selectedRoom: '',
        targetRoom: '',
        newShelfCode: '',
        shelfEdits: {},
        locationIndex: 0,
        items: [],
        search: '',
        selectedIDs: [],
        roomRepairSearch: '',
        roomRepairSelectedIDs: [],
        error: '',
        message: '',
        busy: false,

        init() {
            try {
                this.locations = JSON.parse(this.$el.dataset.locations || '[]');
                this.shelfEdits = Object.fromEntries(this.locations.map(code => [code, code]));
                this.rooms = JSON.parse(this.$el.dataset.rooms || '[]');
                this.selectedRoom = String(this.$el.dataset.selectedRoom || '').trim();
                this.targetRoom = this.rooms[0] || '';
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
            return this.items.filter(item =>
                this.sameShelf(item.shelf, this.currentLocation) &&
                this.normalize(item.room) === this.normalize(this.targetRoom)
            );
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

        get itemsMissingRoom() {
            return this.items.filter(item => !String(item.room || '').trim() && String(item.shelf || '').trim());
        },

        get roomRepairResults() {
            const query = this.normalize(this.roomRepairSearch);
            return this.itemsMissingRoom
                .filter(item => !query || this.normalize(`${item.name} ${item.id} ${item.shelf}`).includes(query))
                .slice(0, 500);
        },

        normalize(value) {
            return String(value || '').trim().toLocaleLowerCase('de').normalize('NFD').replace(/[\u0300-\u036f]/g, '');
        },

        sameShelf(left, right) {
            return String(left || '').trim().toLocaleLowerCase('de') === String(right || '').trim().toLocaleLowerCase('de');
        },

        isAtCurrentLocation(item) {
            return this.sameShelf(item.shelf, this.currentLocation) &&
                this.normalize(item.room) === this.normalize(this.targetRoom);
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

        toggleRoomRepairItem(id, checked) {
            const itemID = Number(id);
            this.roomRepairSelectedIDs = checked
                ? [...new Set([...this.roomRepairSelectedIDs, itemID])]
                : this.roomRepairSelectedIDs.filter(selectedID => selectedID !== itemID);
        },

        selectRoomRepairResults(checked) {
            const resultIDs = new Set(this.roomRepairResults.map(item => Number(item.id)));
            this.roomRepairSelectedIDs = checked
                ? [...new Set([...this.roomRepairSelectedIDs, ...resultIDs])]
                : this.roomRepairSelectedIDs.filter(id => !resultIDs.has(id));
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

        async addShelf() {
            const code = this.newShelfCode.trim();
            if (!code || code.length > 100) {
                this.error = 'Der Regalname muss zwischen 1 und 100 Zeichen lang sein.';
                return;
            }
            this.busy = true;
            this.error = '';
            this.message = '';
            try {
                const response = await fetch('/admin/api/locations/shelves', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ code }),
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Regal konnte nicht hinzugefügt werden.');
                this.locations = [...this.locations, data.code];
                this.shelfEdits[data.code] = data.code;
                this.newShelfCode = '';
                this.message = `Regal ${data.code} wurde hinzugefügt.`;
            } catch (error) {
                this.error = error.message;
            } finally {
                this.busy = false;
            }
        },

        async renameShelf(oldCode) {
            const newCode = String(this.shelfEdits[oldCode] || '').trim();
            if (!newCode || newCode.length > 100) {
                this.error = 'Der Regalname muss zwischen 1 und 100 Zeichen lang sein.';
                return;
            }
            if (newCode === oldCode) return;

            this.busy = true;
            this.error = '';
            this.message = '';
            try {
                const response = await fetch('/admin/api/locations/shelves/rename', {
                    method: 'PATCH',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ old_code: oldCode, new_code: newCode }),
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Regal konnte nicht umbenannt werden.');
                this.locations = this.locations.map(code => code === oldCode ? data.new_code : code);
                this.items.forEach(item => {
                    if (String(item.shelf || '').trim() === oldCode) item.shelf = data.new_code;
                });
                delete this.shelfEdits[oldCode];
                this.shelfEdits[data.new_code] = data.new_code;
                this.message = `Regal ${oldCode} wurde in ${data.new_code} umbenannt.`;
            } catch (error) {
                this.shelfEdits[oldCode] = oldCode;
                this.error = error.message;
            } finally {
                this.busy = false;
            }
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
            this.targetRoom = this.targetRoom.trim();
            if (!this.targetRoom || this.targetRoom.length > 100) {
                this.error = 'Bitte gib einen gültigen Raum an (maximal 100 Zeichen).';
                return;
            }
            const selected = this.items.filter(item => this.selectedIDs.includes(Number(item.id)) && !this.isAtCurrentLocation(item));
            if (selected.length === 0) {
                this.error = 'Wähle mindestens ein Spiel aus, das noch nicht in diesem Fach liegt.';
                return;
            }

            const moving = selected.filter(item =>
                (String(item.shelf || '').trim() || String(item.room || '').trim()) &&
                (!this.sameShelf(item.shelf, this.currentLocation) ||
                    this.normalize(item.room) !== this.normalize(this.targetRoom))
            );
            if (moving.length > 0 && !window.confirm(`${moving.length} Spiel(e) werden in Raum ${this.targetRoom}, Fach ${this.currentLocation} verschoben. Fortfahren?`)) {
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
                        room: this.targetRoom,
                        location: this.currentLocation,
                    }),
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Lagerorte konnten nicht gespeichert werden.');
                this.message = `${data.updated} Spiel(e) Raum ${this.targetRoom}, Fach ${this.currentLocation} zugeordnet.`;
                this.selectedIDs = [];
                await this.loadItems();
                this.rooms = [...new Set([...this.rooms, this.targetRoom])]
                    .sort((left, right) => left.localeCompare(right, 'de', {numeric: true}));
            } catch (error) {
                this.error = error.message;
            } finally {
                this.busy = false;
            }
        },

        async fillMissingRooms() {
            this.targetRoom = this.targetRoom.trim();
            if (!this.targetRoom || this.targetRoom.length > 100) {
                this.error = 'Bitte gib einen gültigen Raum an (maximal 100 Zeichen).';
                return;
            }
            const selected = this.itemsMissingRoom
                .filter(item => this.roomRepairSelectedIDs.includes(Number(item.id)));
            if (selected.length === 0) {
                this.error = 'Wähle mindestens ein Spiel mit fehlendem Raum aus.';
                return;
            }
            if (selected.length > 500) {
                this.error = 'Bitte wähle höchstens 500 Spiele auf einmal aus.';
                return;
            }

            this.busy = true;
            this.error = '';
            this.message = '';
            try {
                const response = await fetch('/admin/api/locations/fill-missing-rooms', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        item_ids: selected.map(item => Number(item.id)),
                        room: this.targetRoom,
                    }),
                });
                const data = await response.json();
                if (!response.ok) throw new Error(data.error || 'Räume konnten nicht gespeichert werden.');
                this.message = `Raum ${this.targetRoom} für ${data.updated} Spiel(e) ergänzt; die Fachzuordnungen blieben unverändert.`;
                this.roomRepairSelectedIDs = [];
                await this.loadItems();
                this.rooms = [...new Set([...this.rooms, this.targetRoom])]
                    .sort((left, right) => left.localeCompare(right, 'de', {numeric: true}));
            } catch (error) {
                this.error = error.message;
            } finally {
                this.busy = false;
            }
        },
    }));
});
