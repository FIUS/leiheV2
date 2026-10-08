package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRedirectAdminNoticeReturnsToInternalAdminPage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/action", func(c *gin.Context) {
		redirectAdminNotice(c, "/admin/games", "success", "Spiel wurde gespeichert.")
	})

	request := httptest.NewRequest(http.MethodPost, "http://localhost/action", nil)
	request.Header.Set("Referer", "http://localhost/admin/games?search=test#results")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusSeeOther)
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if location.Path != "/admin/games" || location.Query().Get("search") != "test" ||
		location.Query().Get("notice") != "Spiel wurde gespeichert." ||
		location.Query().Get("notice_type") != "success" || location.Fragment != "results" {
		t.Errorf("unexpected redirect location: %s", location)
	}
}

func TestRedirectAdminNoticeRejectsExternalReferrer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/action", func(c *gin.Context) {
		redirectAdminNotice(c, "/admin/games", "success", "Saved.")
	})

	request := httptest.NewRequest(http.MethodPost, "/action", nil)
	request.Host = "localhost"
	request.Header.Set("Referer", "https://attacker.example/admin/users")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if location.Path != "/admin/games" {
		t.Errorf("external referrer was not rejected: %s", location)
	}
}
