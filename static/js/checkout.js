document.addEventListener('alpine:init', () => {
    Alpine.data('checkoutFlow', () => ({
        checkoutStep: 1, // 1 = Auth/Gast, 2 = Datum & Zeitraum, 3 = Erfolg
        cart: [],
        checkoutData: {
            startDate: '',
            endDate: '',
            guestName: '',
            guestEmail: ''
        },
        bookingToken: '',
        errorMessage: '',
        isLoading: false,

        init() {
            // 1. Korb aus dem LocalStorage laden
            const savedCart = localStorage.getItem('fius_cart');
            if (savedCart) {
                try {
                    this.cart = JSON.parse(savedCart);
                } catch (e) {
                    this.cart = [];
                }
            }

            // 2. Wenn Korb leer ist, direkt zurück zum Katalog schicken
            if (this.cart.length === 0) {
                window.location.href = '/games';
            }

            // 3. Prüfen ob User bereits eingeloggt ist (über ein data-Attribut im HTML geregelt)
            const rootElement = document.getElementById('checkout-root');
            const userRole = rootElement ? rootElement.dataset.userRole : 'guest';

            // Wenn eingeloggt, direkt Schritt 1 überspringen
            if (userRole && userRole !== 'guest') {
                this.checkoutStep = 2;
            }
        },

        async submitCheckout() {
            this.errorMessage = '';

            // Validierung
            if (!this.checkoutData.startDate || !this.checkoutData.endDate) {
                this.errorMessage = 'Bitte wähle einen Start- und Endzeitraum aus.';
                return;
            }

            this.isLoading = true;

            try {
                const response = await fetch('/api/checkout', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        games: this.cart,
                        startDate: this.checkoutData.startDate,
                        endDate: this.checkoutData.endDate,
                        guestName: this.checkoutData.guestName,
                        guestEmail: this.checkoutData.guestEmail
                    })
                });

                const result = await response.json();

                if (response.ok) {
                    this.bookingToken = result.token;
                    this.checkoutStep = 3; // Erfolg-Ansicht

                    // Lokalen Korb leeren
                    localStorage.removeItem('fius_cart');
                } else {
                    this.errorMessage = result.error || 'Ein Fehler ist aufgetreten.';
                }
            } catch (err) {
                this.errorMessage = 'Netzwerkfehler beim Absenden der Buchung.';
            } finally {
                this.isLoading = false;
            }
        }
    }));
});