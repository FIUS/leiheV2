package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"leiheV2/db"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

type authConfig struct {
	Provider       *oidc.Provider
	OAuth          oauth2.Config
	Verifier       *oidc.IDTokenVerifier
	AdminRole      string
	MaintainerRole string
	LocalAuth      bool
	EmailVerified  bool
}

var auth authConfig

type oidcClaims struct {
	Subject    string   `json:"sub"`
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	GivenName  string   `json:"given_name"`
	FamilyName string   `json:"family_name"`
	Roles      []string `json:"roles"`
}

func InitAuth(ctx context.Context) error {
	issuer := getenv("OIDC_ISSUER", "https://auth.stuvus.uni-stuttgart.de")
	clientID := os.Getenv("OIDC_CLIENT_ID")
	clientSecret := os.Getenv("OIDC_CLIENT_SECRET")
	if clientID == "" || clientSecret == "" {
		return errors.New("OIDC_CLIENT_ID and OIDC_CLIENT_SECRET must be configured")
	}

	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return fmt.Errorf("initialize OIDC provider: %w", err)
	}
	redirectURL := getenv("OIDC_REDIRECT_URL", "http://localhost/auth/stuvus/callback")
	oauthConfig := oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email", "roles"},
	}

	auth = authConfig{
		Provider:       provider,
		OAuth:          oauthConfig,
		Verifier:       provider.Verifier(&oidc.Config{ClientID: clientID}),
		AdminRole:      getenv("OIDC_ADMIN_ROLE", "admin"),
		MaintainerRole: getenv("OIDC_MAINTAINER_ROLE", "maintainer"),
		LocalAuth:      getenvBool("ENABLE_LOCAL_AUTH", false),
		EmailVerified:  getenvBool("LOCAL_AUTH_REQUIRE_EMAIL_VERIFICATION", false),
	}
	return nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func getenvBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		role, ok := session.Get("user_role").(string)
		if !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Bitte einloggen"})
			return
		}
		if role == "admin" {
			c.Next()
			return
		}
		for _, allowed := range allowedRoles {
			if role == allowed {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Keine Berechtigung"})
	}
}

func Login(c *gin.Context) {
	c.Redirect(http.StatusFound, "/auth/stuvus/login")
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, "Sitzung konnte nicht beendet werden")
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func OIDCLogin(c *gin.Context) {
	if auth.Provider == nil {
		c.String(http.StatusServiceUnavailable, "OIDC ist nicht konfiguriert")
		return
	}
	state, err := randomState()
	if err != nil {
		c.String(http.StatusInternalServerError, "Anmeldung konnte nicht gestartet werden")
		return
	}
	session := sessions.Default(c)
	session.Set("oidc_state", state)
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, "Anmeldung konnte nicht gestartet werden")
		return
	}
	c.Redirect(http.StatusFound, auth.OAuth.AuthCodeURL(state, oauth2.AccessTypeOffline))
}

func OIDCCallback(c *gin.Context) {
	session := sessions.Default(c)
	state, _ := session.Get("oidc_state").(string)
	if state == "" || state != c.Query("state") {
		c.String(http.StatusBadRequest, "Ungültiger OIDC-Status")
		return
	}
	session.Delete("oidc_state")

	token, err := auth.OAuth.Exchange(c.Request.Context(), c.Query("code"))
	if err != nil {
		c.String(http.StatusUnauthorized, "OIDC-Anmeldung fehlgeschlagen")
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		c.String(http.StatusUnauthorized, "OIDC-Antwort enthält kein ID-Token")
		return
	}
	idToken, err := auth.Verifier.Verify(c.Request.Context(), rawIDToken)
	if err != nil {
		c.String(http.StatusUnauthorized, "OIDC-Token konnte nicht verifiziert werden")
		return
	}
	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil || claims.Subject == "" || claims.Email == "" {
		c.String(http.StatusUnauthorized, "OIDC-Token enthält unvollständige Benutzerdaten")
		return
	}

	role := roleFromOIDC(claims.Roles)
	firstName, lastName := claims.GivenName, claims.FamilyName
	if firstName == "" && claims.Name != "" {
		parts := strings.Fields(claims.Name)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = strings.Join(parts[1:], " ")
		}
	}
	if firstName == "" {
		firstName = claims.Email
	}
	user, err := queries.UpsertUser(c.Request.Context(), db.UpsertUserParams{
		OidcSub: claims.Subject, Email: claims.Email, FirstName: firstName,
		LastName: lastName, Role: db.UserRole(role),
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Benutzer konnte nicht gespeichert werden")
		return
	}
	session.Set("user_id", user.ID)
	session.Set("user_role", string(user.Role))
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, "Sitzung konnte nicht gespeichert werden")
		return
	}
	c.Redirect(http.StatusFound, "/")
}

func roleFromOIDC(roles []string) string {
	for _, role := range roles {
		switch strings.TrimSpace(role) {
		case auth.AdminRole:
			return "admin"
		case auth.MaintainerRole:
			return "maintainer"
		}
	}
	return "user"
}

func LocalLogin(c *gin.Context) {
	if !auth.LocalAuth {
		c.String(http.StatusNotFound, "Lokale Anmeldung ist deaktiviert")
		return
	}
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	user, err := queries.GetUserByEmail(c.Request.Context(), email)
	if err != nil || !user.PasswordHash.Valid || (auth.EmailVerified && !user.EmailVerified) || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(password)) != nil {
		renderAuthPage(c, http.StatusUnauthorized, "Anmeldung fehlgeschlagen")
		return
	}
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("user_role", string(user.Role))
	_ = session.Save()
	c.Redirect(http.StatusFound, "/")
}

func RegisterLocal(c *gin.Context) {
	if !auth.LocalAuth {
		c.String(http.StatusNotFound, "Lokale Anmeldung ist deaktiviert")
		return
	}
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	if len(password) < 8 {
		renderAuthPage(c, http.StatusBadRequest, "Das Passwort muss mindestens 8 Zeichen enthalten")
		return
	}
	if strings.TrimSpace(c.PostForm("first_name")) == "" || strings.TrimSpace(c.PostForm("last_name")) == "" {
		renderAuthPage(c, http.StatusBadRequest, "Vor- und Nachname sind erforderlich")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		renderAuthPage(c, http.StatusInternalServerError, "Registrierung fehlgeschlagen")
		return
	}
	user, err := queries.CreateLocalUser(c.Request.Context(), db.CreateLocalUserParams{
		Email: email, FirstName: strings.TrimSpace(c.PostForm("first_name")),
		LastName: strings.TrimSpace(c.PostForm("last_name")),
		OidcSub:  "local-" + email, PasswordHash: sql.NullString{String: string(hash), Valid: true},
		EmailVerified: !auth.EmailVerified,
	})
	if err != nil {
		renderAuthPage(c, http.StatusConflict, "Diese E-Mail-Adresse ist bereits registriert")
		return
	}
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("user_role", string(user.Role))
	_ = session.Save()
	c.Redirect(http.StatusFound, "/")
}

func randomState() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
