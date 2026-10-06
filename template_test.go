package main

import (
	"bytes"
	"database/sql"
	"html/template"
	"leiheV2/db"
	"testing"
	"time"
)

func TestTemplatesParseAndExecute(t *testing.T) {
	tmpl, err := template.ParseGlob("templates/*.html")
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
		"UserRole": "admin",
		"Games":    testGames,
		"Users":    testUsers,
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

func TestAdminGamesRendersSingleDeleteDialog(t *testing.T) {
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		t.Fatalf("Failed to parse templates: %v", err)
	}

	games := []db.ListItemsWithCategoriesRow{
		{ID: 1, Name: "First Game", Status: "available"},
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
}
