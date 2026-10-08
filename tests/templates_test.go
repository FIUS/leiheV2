package tests

import (
	"bytes"
	"database/sql"
	"html/template"
	"leiheV2/db"
	"strings"
	"testing"
	"time"
)

func TestTemplatesParseAndExecute(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	testUsers := []db.User{
		{
			ID:        1,
			FirstName: "Max",
			LastName:  "O'Connor",
			Email:     "max@fius.de",
			Role:      db.UserRoleAdmin,
			OidcSub:   "manual-max@fius.de",
			CreatedAt: time.Now(),
			LastLogin: sql.NullTime{Time: time.Now(), Valid: true},
		},
		{
			ID:        2,
			FirstName: "Erika",
			LastName:  "Musterfrau",
			Email:     "erika@fius.de",
			Role:      db.UserRoleMaintainer,
			OidcSub:   "manual-erika@fius.de",
			CreatedAt: time.Now(),
			LastLogin: sql.NullTime{Valid: false},
		},
	}

	testGames := []db.Item{
		{
			ID:          1,
			Name:        "Pandemic: It's Coming",
			MinPlayers:  3,
			MaxPlayers:  4,
			MinPlaytime: 60,
			MaxPlaytime: 90,
			Status:      "available",
		},
	}

	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "managment.html", map[string]interface{}{
		"UserRole":              "admin",
		"ModuleBarcodesEnabled": true,
		"PendingBookingCount":   int64(3),
		"Games":                 testGames,
		"Users":                 testUsers,
	})
	if err != nil {
		t.Fatalf("Failed to execute managment.html: %v", err)
	}

	output := buf.String()
	t.Log(output)
	if !bytes.Contains([]byte(output), []byte("Max O&#39;Connor")) && !bytes.Contains([]byte(output), []byte("Max O'Connor")) {
		t.Errorf("Expected output to contain 'Max O'Connor'")
	}
	if !bytes.Contains([]byte(output), []byte("erika@fius.de")) {
		t.Errorf("Expected output to contain 'erika@fius.de'")
	}
	if !bytes.Contains([]byte(output), []byte("Nutzerverwaltung")) {
		t.Errorf("Expected output to contain 'Nutzerverwaltung'")
	}
	for _, expected := range []string{
		`aria-label="Menü öffnen"`, `lg:hidden`, `/admin/ausgabe#issue`, `/admin/ausgabe#return`,
		`Buchungen`, `Nutzerkonto`, `Meine Buchungen`,
		`aria-label="Buchungen, 3 unbearbeitet"`, `bg-red-600`,
		`/static/js/admin_feedback.js`,
	} {
		if !bytes.Contains([]byte(output), []byte(expected)) {
			t.Errorf("Expected responsive header output to contain %q", expected)
		}
		if count := bytes.Count([]byte(output), []byte(`>3</span>`)); count != 2 {
			t.Errorf("Expected pending booking count in desktop and mobile navigation, got %d badges", count)
		}
	}
	if bytes.Contains([]byte(output), []byte("Etiketten-Designer")) {
		t.Error("barcode designer link must not appear in the global header")
	}
	if bytes.Contains([]byte(output), []byte("bggQuery = '")) {
		t.Errorf("Found unsafe inline string interpolation in bggQuery")
	}
	if bytes.Contains([]byte(output), []byte("typedName !== '")) {
		t.Errorf("Found unsafe inline string interpolation in typedName check")
	}

	// Test with nil and empty slices
	var nilBuf bytes.Buffer
	err = tmpl.ExecuteTemplate(&nilBuf, "managment.html", map[string]interface{}{
		"UserRole": "admin",
		"Games":    nil,
		"Users":    nil,
	})
	if err != nil {
		t.Fatalf("Failed to execute managment.html with nil data: %v", err)
	}

	nilOutput := nilBuf.String()
	if !bytes.Contains([]byte(nilOutput), []byte("Keine Spiele vorhanden.")) {
		t.Errorf("Expected output to contain 'Keine Spiele vorhanden.'")
	}
	if !bytes.Contains([]byte(nilOutput), []byte("Keine Nutzer vorhanden.")) {
		t.Errorf("Expected output to contain 'Keine Nutzer vorhanden.'")
	}
}

func TestAdminDashboardRendersMetricsAndCatalogChecks(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}
	var output bytes.Buffer
	dashboard := db.DashboardData{
		Metrics: db.DashboardMetrics{
			TotalGames: 40, AvailableGames: 28, TotalUsers: 100,
			ActiveLoans: 8, OverdueLoans: 2, PendingPickupsToday: 3,
			ConfirmedPickupsToday: 4, TodayPickups: 7, PendingBookings: 5,
			ReturnsToday: 2, AverageBGGRating: sql.NullFloat64{Float64: 3.9, Valid: true},
			MostBorrowedGame: "Testspiel", MostBorrowedCount: 12,
			GamesNeedingBGGSyncCount: 1, GamesMissingLocationCount: 1,
			GamesMissingCustomImageCount: 1,
		},
		GamesNeedingBGGSync:     []db.DashboardGame{{ID: 1, Name: "BGG fehlt"}},
		GamesMissingLocation:    []db.DashboardGame{{ID: 2, Name: "Lagerort fehlt"}},
		GamesMissingCustomImage: []db.DashboardGame{{ID: 3, Name: "Bild fehlt"}},
	}
	if err := tmpl.ExecuteTemplate(&output, "admin_dashboard.html", map[string]interface{}{
		"UserRole":  "admin",
		"ActiveTab": "dashboard",
		"Dashboard": dashboard,
	}); err != nil {
		t.Fatalf("Failed to execute admin_dashboard.html: %v", err)
	}
	for _, expected := range []string{
		"Aktive Ausleihen", "Überfällige Rückgaben", "Abholung heute", "Offene Anfragen",
		"Verfügbare Spiele", "Rückgaben heute", "3.9", "Testspiel", "BGG fehlt",
		"Lagerort fehlt", "Bild fehlt", "/setup-bilder",
	} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected dashboard output to contain %q", expected)
		}
	}
}

func TestAdminGamesRendersSingleDeleteDialog(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	games := []db.ListItemsWithCategoriesRow{
		{ID: 1, Name: "First Game", Status: "available", Room: sql.NullString{String: "V38.008", Valid: true}, Shelf: sql.NullString{String: "A1", Valid: true}},
		{ID: 2, Name: "Second Game", Status: "available"},
	}
	var output bytes.Buffer
	err = tmpl.ExecuteTemplate(&output, "admin_games.html", map[string]interface{}{
		"UserRole":   "admin",
		"ActiveTab":  "games",
		"Games":      games,
		"Categories": []db.Category{},
	})
	if err != nil {
		t.Fatalf("Failed to execute admin_games.html: %v", err)
	}

	if count := bytes.Count(output.Bytes(), []byte(`x-show="deleteModalOpen"`)); count != 1 {
		t.Errorf("Expected one shared delete dialog, got %d", count)
	}
	if count := bytes.Count(output.Bytes(), []byte(`x-show="editModalOpen"`)); count != 1 {
		t.Errorf("Expected one edit dialog, got %d", count)
	}
	for _, expected := range []string{`name="room" x-model="editGame.room"`, `name="shelf" x-model="editGame.shelf"`, `data-room="V38.008"`, `data-shelf="A1"`} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected admin game editor to contain %q", expected)
		}
	}
}

func TestAdminBookingsRendersAllBookingStatesAndOrigins(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	bookings := []db.BookingOverview{
		{
			ID: 1, Token: "PENDING1", PersonName: "User One",
			Email:  sql.NullString{String: "user@example.com", Valid: true},
			Status: "pending", ItemNames: []string{"First Game"},
		},
		{
			ID: 2, Token: "CONFIRM1", PersonName: "Staff Borrower",
			Status: "confirmed", CreatedByStaff: true, ItemNames: []string{"Second Game"},
		},
		{
			ID: 3, Token: "CANCELLED1", PersonName: "User Two",
			Status: "cancelled", ItemNames: []string{"Third Game"},
		},
	}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "admin_bookings.html", map[string]interface{}{
		"UserRole":        "admin",
		"UserName":        "Admin",
		"ActiveTab":       "bookings",
		"Bookings":        bookings,
		"PendingBookings": bookings[:1],
	}); err != nil {
		t.Fatalf("Failed to execute admin_bookings.html: %v", err)
	}
	for _, expected := range []string{"Zu bearbeitende Buchungen", "Alle Buchungen", "Buchungsnummer, Name oder E-Mail suchen", "matchesBooking", "PENDING1", "CONFIRM1", "CANCELLED1", "Ausstehend", "Bestätigt", "Storniert", "Erstellt durch Team", `action="/admin/bookings/review"`, "Bestätigen", "Ablehnen", `action="/admin/bookings/confirmed/cancel"`} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected admin bookings output to contain %q", expected)
		}
	}

	output.Reset()
	if err := tmpl.ExecuteTemplate(&output, "admin_bookings.html", map[string]interface{}{
		"UserRole":        "maintainer",
		"UserName":        "Maintainer",
		"ActiveTab":       "bookings",
		"Bookings":        bookings,
		"PendingBookings": bookings[:1],
	}); err != nil {
		t.Fatalf("Failed to execute maintainer admin_bookings.html: %v", err)
	}
	if bytes.Contains(output.Bytes(), []byte(`action="/admin/bookings/confirmed/cancel"`)) {
		t.Error("maintainers must not see confirmed-booking cancellation")
	}
	if !bytes.Contains(output.Bytes(), []byte(`action="/admin/bookings/review"`)) {
		t.Error("maintainers should be able to review pending bookings")
	}
}

func TestCheckoutAccountViewDoesNotShowOtherBorrowerFields(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "checkout.html", map[string]interface{}{
		"UserRole": "user",
		"UserName": "Account User",
	}); err != nil {
		t.Fatalf("Failed to execute checkout.html: %v", err)
	}
	if bytes.Contains(output.Bytes(), []byte("bookingForSomeoneElse")) {
		t.Error("regular account checkout should not render the staff booking toggle")
	}
	if bytes.Contains(output.Bytes(), []byte("Vor- und Nachname")) {
		t.Error("regular account checkout should use the signed-in account instead of guest details")
	}

	output.Reset()
	if err := tmpl.ExecuteTemplate(&output, "checkout.html", map[string]interface{}{
		"UserRole": "maintainer",
		"UserName": "Staff User",
	}); err != nil {
		t.Fatalf("Failed to execute staff checkout.html: %v", err)
	}
	if bytes.Contains(output.Bytes(), []byte("bookingForSomeoneElse")) {
		t.Error("staff checkout should not render an option to book on someone else's behalf")
	}
	if bytes.Contains(output.Bytes(), []byte("Vor- und Nachname")) {
		t.Error("staff checkout should use the staff account instead of borrower details")
	}
}

func TestStaffLoansTemplateRendersIssueAndReturnFlows(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "staff_loans.html", map[string]interface{}{
		"UserRole":       "maintainer",
		"UserName":       "Staff",
		"RequireDeposit": true,
	}); err != nil {
		t.Fatalf("Failed to execute staff_loans.html: %v", err)
	}
	for _, expected := range []string{"Buchungen", "Ausgeben", "Zurücknehmen", "Vorbestellung", "Jetzt ausleihen", "Nach Spielnamen suchen", "Barcode / System-ID eingeben", "System-ID", "Lagerort", "Bitte die Spiele gleich wieder an ihren Lagerort aufräumen", "Aktive Ausleihe suchen", "Abgegebenes Pfand", "booking.deposit || 'Kein Pfand erfasst'", "data-require-deposit=\"true\"", "Kamera"} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected staff lending page to contain %q", expected)
		}
	}
}

func TestAdminSettingsTemplateRendersDepositSettingWithoutOverflow(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "admin_settings.html", map[string]interface{}{
		"UserRole":  "admin",
		"ActiveTab": "settings",
		"Settings": db.AppSettings{
			RequireDeposit: true, ModuleBarcodesEnabled: true,
			EmailBookingReceived: true, EmailBookingReview: true, EmailBookingUpdates: true,
			IgnoredEmailAddresses: "noreply@example.org",
		},
	}); err != nil {
		t.Fatalf("Failed to execute admin_settings.html: %v", err)
	}
	for _, expected := range []string{
		"Pfand bei jeder Ausgabe erforderlich", `name="require_deposit" value="true" checked`,
		"Etiketten-Designer aktivieren", `name="module_barcodes_enabled" value="true" checked`,
		"E-Mail-Benachrichtigungen", `name="email_booking_received" value="true" checked`,
		`name="email_booking_review" value="true" checked`, `name="email_booking_updates" value="true" checked`,
		`id="ignored-email-addresses"`, "noreply@example.org", "max-w-full min-w-0", `action="/admin/settings"`,
	} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected admin settings output to contain %q", expected)
		}
	}
}

func TestBarcodeDesignerTemplateAndAdminNavigation(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "barcode_designer.html", map[string]interface{}{
		"UserRole": "admin", "UserName": "Admin", "ActiveTab": "barcodes",
		"ModuleBarcodesEnabled": true,
	}); err != nil {
		t.Fatalf("Failed to execute barcode_designer.html: %v", err)
	}
	for _, expected := range []string{
		"Etiketten-Designer", "barcodeDesigner",
		"Endlosband", "Einzel-Etikett", "Code-128", "URL-Template", "Alle angezeigten auswählen",
		"Manifest JSON", "qrcode-generator.js", "JsBarcode.all.min.js", "jszip.min.js", "/static/js/label_designer.js",
		`x-model="bandWidth"`, `x-model="dieCutSize"`, `placeholder="z. B. 29 x 90 oder 50 × 30"`,
		"Titel", "Etikettenausrichtung", "Horizontal", "Vertikal", "Zeilen für Spielenamen",
		"Schrift verkleinern", "Mit … kürzen", "ohne zusätzliches Rand-Padding",
	} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected barcode designer output to contain %q", expected)
		}
	}
	if bytes.Contains(output.Bytes(), []byte("Kategorie")) {
		t.Error("barcode designer should not show game categories")
	}

	for _, role := range []string{"admin", "maintainer"} {
		output.Reset()
		if err := tmpl.ExecuteTemplate(&output, "admin_nav", map[string]interface{}{
			"UserRole": role, "ActiveTab": "barcodes", "ModuleBarcodesEnabled": true,
		}); err != nil {
			t.Fatalf("Failed to execute admin navigation for %s: %v", role, err)
		}
		if !bytes.Contains(output.Bytes(), []byte(`href="/admin/labels"`)) {
			t.Errorf("barcode designer link should be available to %s in the internal admin navigation", role)
		}
	}
	output.Reset()
	if err := tmpl.ExecuteTemplate(&output, "admin_nav", map[string]interface{}{
		"UserRole": "admin", "ActiveTab": "dashboard", "ModuleBarcodesEnabled": false,
	}); err != nil {
		t.Fatalf("Failed to execute disabled admin navigation: %v", err)
	}
	if bytes.Contains(output.Bytes(), []byte("Etiketten-Designer")) {
		t.Error("barcode designer link must be hidden while the module is disabled")
	}
}

func TestAdminLocationModuleAndNavigation(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "admin_locations.html", map[string]interface{}{
		"UserRole":      "admin",
		"ActiveTab":     "locations",
		"LocationsJSON": `["A1L","A1R","B0"]`,
		"SelectedRoom":  "V38.008",
	}); err != nil {
		t.Fatalf("Failed to execute admin_locations.html: %v", err)
	}
	for _, expected := range []string{
		"Lagerort-Assistent", "Spiele hinzufügen", "In diesem Fach",
		"Ausgewählte Spiele zuweisen", "/static/js/admin_locations.js",
		`/admin/locations`, `data-selected-room="V38.008"`, "roomShelves",
	} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected location module output to contain %q", expected)
		}
	}

	output.Reset()
	if err := tmpl.ExecuteTemplate(&output, "admin_nav", map[string]interface{}{
		"UserRole": "admin", "ActiveTab": "locations", "ModuleBarcodesEnabled": true,
		"StorageRooms": []string{"V38.008", "Haus & Hof"},
	}); err != nil {
		t.Fatalf("Failed to execute admin navigation: %v", err)
	}
	nav := output.String()
	labelsIndex := strings.Index(nav, `href="/admin/labels"`)
	locationsIndex := strings.Index(nav, `href="/admin/locations"`)
	usersIndex := strings.Index(nav, `href="/admin/users"`)
	settingsIndex := strings.Index(nav, `href="/admin/settings"`)
	if labelsIndex < 0 || locationsIndex < 0 || usersIndex < 0 || settingsIndex < 0 {
		t.Fatal("admin navigation is missing a module or admin-only link")
	}
	if !(labelsIndex < locationsIndex && locationsIndex < usersIndex && usersIndex < settingsIndex) {
		t.Error("modules should precede admin-only links, with settings rightmost")
	}
	for _, roomLink := range []string{`href="/admin/locations?room=V38.008"`, `href="/admin/locations?room=Haus&#43;%26&#43;Hof"`} {
		if !strings.Contains(nav, roomLink) {
			t.Errorf("admin location menu should include encoded room link %q", roomLink)
		}
	}

	output.Reset()
	if err := tmpl.ExecuteTemplate(&output, "admin_nav", map[string]interface{}{
		"UserRole": "maintainer", "ActiveTab": "barcodes", "ModuleBarcodesEnabled": true,
		"StorageRooms": []string{"V38.008"},
	}); err != nil {
		t.Fatalf("Failed to execute maintainer navigation: %v", err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`href="/admin/labels"`)) {
		t.Error("maintainers should retain access to the barcode designer")
	}
	if bytes.Contains(output.Bytes(), []byte(`href="/admin/locations"`)) {
		t.Error("maintainers must not see the admin-only location module")
	}
	if bytes.Contains(output.Bytes(), []byte(`href="/admin/settings"`)) {
		t.Error("maintainers must not see admin settings")
	}
}

func TestGamesTemplateRendersStoredBGGRatingAndCategoryTags(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}
	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "games.html", map[string]interface{}{
		"UserRole": "guest",
		"Games": []db.ListItemsWithCategoriesRow{
			{
				ID: 1, Name: "Test Game", MinPlayers: 2, MaxPlayers: 4,
				MinPlaytime: 45, MaxPlaytime: 45,
				ImageUrl:        sql.NullString{String: "https://cf.geekdo-images.com/test.jpg", Valid: true},
				BGGRating:       sql.NullFloat64{Float64: 7.8, Valid: true},
				CategoryNames:   "Strategy,Cooperative",
				Status:          "available",
				HasBookingToday: true,
			},
			{ID: 2, Name: "Maintenance Game", Status: db.ItemStatusMaintenance},
		},
		"Categories": []db.Category{},
	}); err != nil {
		t.Fatalf("Failed to execute games.html: %v", err)
	}
	for _, expected := range []string{`data-rating="7.8"`, `data-categories="Strategy,Cooperative"`, `data-status="maintenance"`, `source=0`, `v=3`, "Nicht verfügbar", "activeItem?.categories?.length > 0", "categories || []", `aria-label="Vorheriges Bild"`, `aria-label="Nächstes Bild"`, "previousImage()", "nextImage()", "rating / 2", "Math.max(0, Math.min(100", "tags.slice(0, 2)", "45 Min."} {
		if !bytes.Contains(output.Bytes(), []byte(expected)) {
			t.Errorf("Expected games output to contain %q", expected)
		}
	}
	if !bytes.Contains(output.Bytes(), []byte("Heute ausgeliehen")) {
		t.Error("games cards should visually identify items with a confirmed or active booking today")
	}
	if !bytes.Contains(output.Bytes(), []byte("addToCart(1")) {
		t.Error("a booking today should not prevent adding an otherwise available game to the cart")
	}
	if bytes.Contains(output.Bytes(), []byte("45 - 45 Min.")) {
		t.Error("games cards should display equal minimum and maximum playtimes only once")
	}
	if bytes.Contains(output.Bytes(), []byte(`>7.8 / 10</span>`)) {
		t.Error("games cards should display the mapped five-star rating rather than a hard-coded ten-star rating")
	}
}

func TestGamesTemplateRendersEqualMinAndMaxPlayersAsSingleValue(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	game := db.ListItemsWithCategoriesRow{
		ID:          1,
		Name:        "2-Player Duel",
		MinPlayers:  2,
		MaxPlayers:  2,
		MinPlaytime: 30,
		MaxPlaytime: 30,
		Status:      "available",
	}

	var output bytes.Buffer
	if err := tmpl.ExecuteTemplate(&output, "games.html", map[string]interface{}{
		"UserRole":   "guest",
		"Games":      []db.ListItemsWithCategoriesRow{game},
		"Categories": []db.Category{},
	}); err != nil {
		t.Fatalf("Failed to execute games.html: %v", err)
	}

	rendered := output.String()
	if bytes.Contains(output.Bytes(), []byte("2 - 2")) {
		t.Errorf("games.html should not render '2 - 2' when min and max players are equal: %s", rendered)
	}
	if !bytes.Contains(output.Bytes(), []byte(`Spieler:</strong> 2</span>`)) {
		t.Errorf("games.html should render 'Spieler: 2' when min and max players are 2: %s", rendered)
	}

	// Verify admin_games.html
	var adminOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&adminOutput, "admin_games.html", map[string]interface{}{
		"UserRole":   "admin",
		"ActiveTab":  "games",
		"Games":      []db.ListItemsWithCategoriesRow{game},
		"Categories": []db.Category{},
	}); err != nil {
		t.Fatalf("Failed to execute admin_games.html: %v", err)
	}
	if bytes.Contains(adminOutput.Bytes(), []byte("2 - 2")) {
		t.Errorf("admin_games.html should not render '2 - 2' when min and max players are equal")
	}
}

func TestCameraScannerAndGameIDFeaturesRendered(t *testing.T) {
	tmpl, err := template.ParseGlob("../templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	// 1. Header includes the maintained scanner decoder before camera_scanner.js.
	var headerOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&headerOutput, "header", map[string]interface{}{
		"UserRole": "admin",
	}); err != nil {
		t.Fatalf("Failed to execute header template: %v", err)
	}
	if !bytes.Contains(headerOutput.Bytes(), []byte("/static/js/camera_scanner.js")) {
		t.Error("header should include /static/js/camera_scanner.js")
	}
	if !bytes.Contains(headerOutput.Bytes(), []byte("/static/js/html5-qrcode.min.js")) {
		t.Error("header should include /static/js/html5-qrcode.min.js")
	}

	// 2. Staff loans includes camera buttons and game capture indicators for prebookings, spontaneous loans, and returns
	var staffOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&staffOutput, "staff_loans.html", map[string]interface{}{
		"UserRole":       "maintainer",
		"RequireDeposit": true,
	}); err != nil {
		t.Fatalf("Failed to execute staff_loans.html: %v", err)
	}
	for _, expected := range []string{
		"openScannerForPrebooking()", "openScannerForSpontaneous()", "openScannerForReturn()",
		"isPrebookedItemScanned", "isReturnItemScanned", "canIssueSelectedBooking()",
	} {
		if !bytes.Contains(staffOutput.Bytes(), []byte(expected)) {
			t.Errorf("staff_loans.html missing expected element: %s", expected)
		}
	}

	// 3. Admin bookings has text search but NO camera scan button
	var bookingsOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&bookingsOutput, "admin_bookings.html", map[string]interface{}{
		"UserRole": "admin",
		"Bookings": []db.BookingOverview{
			{ID: 1, Token: "TEST1234", PersonName: "Test User", Status: "active", Email: sql.NullString{String: "test@example.com", Valid: true}},
		},
	}); err != nil {
		t.Fatalf("Failed to execute admin_bookings.html: %v", err)
	}
	if bytes.Contains(bookingsOutput.Bytes(), []byte("openScanner()")) || bytes.Contains(bookingsOutput.Bytes(), []byte("Kamera-Scan")) {
		t.Error("admin_bookings.html should NOT include camera scan button")
	}
	if !bytes.Contains(bookingsOutput.Bytes(), []byte(`id="booking-search"`)) {
		t.Error("admin_bookings.html should include text search input")
	}

	// 4. Admin games has text search but NO camera scan button
	var adminGamesOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&adminGamesOutput, "admin_games.html", map[string]interface{}{
		"UserRole":  "admin",
		"ActiveTab": "games",
		"Games": []db.ListItemsWithCategoriesRow{
			{ID: 1, Name: "Test Game", Status: "available"},
		},
		"Categories": []db.Category{},
	}); err != nil {
		t.Fatalf("Failed to execute admin_games.html: %v", err)
	}
	if bytes.Contains(adminGamesOutput.Bytes(), []byte("openScanner()")) || bytes.Contains(adminGamesOutput.Bytes(), []byte("Kamera-Scan")) {
		t.Error("admin_games.html should NOT include camera scan button")
	}
	if !bytes.Contains(adminGamesOutput.Bytes(), []byte(`id="admin-game-search"`)) {
		t.Error("admin_games.html should include text search input")
	}

	// 5. Games catalog has text search but NO camera scan button
	var gamesOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&gamesOutput, "games.html", map[string]interface{}{
		"UserRole": "guest",
		"Games": []db.ListItemsWithCategoriesRow{
			{ID: 1, Name: "Test Game", Status: "available"},
		},
		"Categories": []db.Category{},
	}); err != nil {
		t.Fatalf("Failed to execute games.html: %v", err)
	}
	if bytes.Contains(gamesOutput.Bytes(), []byte("openScanner()")) {
		t.Error("games.html should NOT include camera scan button")
	}
	if !bytes.Contains(gamesOutput.Bytes(), []byte(`x-model="search"`)) {
		t.Error("games.html should include text search input")
	}

	// 6. Barcode designer has text search but NO camera scan button
	var designerOutput bytes.Buffer
	if err := tmpl.ExecuteTemplate(&designerOutput, "barcode_designer.html", map[string]interface{}{
		"UserRole":  "admin",
		"ActiveTab": "barcode-designer",
	}); err != nil {
		t.Fatalf("Failed to execute barcode_designer.html: %v", err)
	}
	if bytes.Contains(designerOutput.Bytes(), []byte("openScanner()")) {
		t.Error("barcode_designer.html should NOT include camera scan button")
	}
}
