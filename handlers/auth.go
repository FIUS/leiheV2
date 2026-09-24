package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"leiheV2/db"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

type authConfig struct {
	OIDCEnabled       bool
	InitialAdminEmail string
	Provider          *oidc.Provider
	OAuth             oauth2.Config
	Verifier          *oidc.IDTokenVerifier
	AdminRole         string
	MaintainerRole    string
	LocalAuth         bool
	EmailVerified     bool
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
	oidcEnabled, err := requiredBool("ENABLE_OIDC")
	if err != nil {
		return err
	}
	localAuth, err := requiredBool("ENABLE_LOCAL_AUTH")
	if err != nil {
		return err
	}
	emailVerified, err := requiredBool("LOCAL_AUTH_REQUIRE_EMAIL_VERIFICATION")
	if err != nil {
		return err
	}
	auth = authConfig{
		OIDCEnabled:       oidcEnabled,
		InitialAdminEmail: strings.ToLower(strings.TrimSpace(os.Getenv("INITIAL_ADMIN_EMAIL"))),
		LocalAuth:         localAuth,
		EmailVerified:     emailVerified,
	}
	if !oidcEnabled {
		return nil
	}

	issuer, err := requiredEnv("OIDC_ISSUER")
	if err != nil {
		return err
	}
	clientID, err := requiredEnv("OIDC_CLIENT_ID")
	if err != nil {
		return err
	}
	clientSecret, err := requiredEnv("OIDC_CLIENT_SECRET")
	if err != nil {
		return err
	}
	redirectURL, err := requiredEnv("OIDC_REDIRECT_URL")
	if err != nil {
		return err
	}
	scopes := configuredOIDCScopes()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return fmt.Errorf("initialize OIDC provider: %w", err)
	}
	auth.Provider = provider
	auth.OAuth = oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       scopes,
	}

	auth.Verifier = provider.Verifier(&oidc.Config{ClientID: clientID})
	auth.AdminRole = os.Getenv("OIDC_ADMIN_ROLE")
	auth.MaintainerRole = os.Getenv("OIDC_MAINTAINER_ROLE")
	return nil
}

func configuredOIDCScopes() []string {
	value := strings.TrimSpace(os.Getenv("OIDC_SCOPES"))
	if value == "" {
		return []string{oidc.ScopeOpenID, "profile", "email"}
	}
	seen := make(map[string]struct{})
	scopes := make([]string, 0)
	for _, scope := range strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t'
	}) {
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		return []string{oidc.ScopeOpenID, "profile", "email"}
	}
	return scopes
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%s must be configured", key)
	}
	return value, nil
}

func requiredBool(key string) (bool, error) {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return false, fmt.Errorf("%s must be configured", key)
	}
	switch value {
	case "1", "true", "yes":
		return true, nil
	case "0", "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a boolean", key)
	}
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
	if !auth.OIDCEnabled || auth.Provider == nil {
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
	if !auth.OIDCEnabled || auth.Provider == nil || auth.Verifier == nil {
		c.String(http.StatusNotFound, "OIDC ist deaktiviert")
		return
	}
	if providerError := strings.TrimSpace(c.Query("error")); providerError != "" {
		description := strings.TrimSpace(c.Query("error_description"))
		if description == "" {
			description = providerError
		}
		c.String(http.StatusUnauthorized, "OIDC-Anmeldung wurde vom Anbieter abgelehnt: %s", description)
		return
	}
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
	if IsInitialAdminEmail(claims.Email) {
		if err := queries.PromoteUserToAdminByEmail(c.Request.Context(), claims.Email); err != nil {
			c.String(http.StatusInternalServerError, "Benutzerrolle konnte nicht gespeichert werden")
			return
		}
		user.Role = db.UserRoleAdmin
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

	if IsInitialAdminEmail(user.Email) {
		if err := queries.PromoteUserToAdminByEmail(c.Request.Context(), user.Email); err != nil {
			renderAuthPage(c, http.StatusInternalServerError, "Benutzerrolle konnte nicht gespeichert werden")
			return
		}
		user.Role = db.UserRoleAdmin
	}
	if auth.EmailVerified {
		token, err := randomState()
		if err != nil {
			renderAuthPage(c, http.StatusInternalServerError, "Registrierung fehlgeschlagen")
			return
		}
		if err := queries.CreateEmailVerificationToken(c.Request.Context(), hashToken(token), user.ID, time.Now().Add(24*time.Hour)); err != nil {
			renderAuthPage(c, http.StatusInternalServerError, "Bestätigungs-E-Mail konnte nicht vorbereitet werden")
			return
		}
		if err := sendVerificationEmail(user, token); err != nil {
			logMailError("verification", err)
			renderAuthPage(c, http.StatusInternalServerError, "Bestätigungs-E-Mail konnte nicht versendet werden")
			return
		}
		renderAuthPage(c, http.StatusOK, "Registrierung erfolgreich. Bitte bestätige deine E-Mail-Adresse über den Link in deinem Postfach.")
		return
	}
	session := sessions.Default(c)
	session.Set("user_id", user.ID)
	session.Set("user_role", string(user.Role))
	_ = session.Save()
	c.Redirect(http.StatusFound, "/")
}

func VerifyEmail(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		c.String(http.StatusBadRequest, "Ungültiger Bestätigungslink")
		return
	}
	userID, err := queries.ConsumeEmailVerificationToken(c.Request.Context(), hashToken(token))
	if err != nil {
		c.String(http.StatusBadRequest, "Der Bestätigungslink ist ungültig oder abgelaufen")
		return
	}
	if err := queries.MarkUserEmailVerified(c.Request.Context(), userID); err != nil {
		c.String(http.StatusInternalServerError, "E-Mail-Adresse konnte nicht bestätigt werden")
		return
	}
	c.Redirect(http.StatusFound, "/login")
}

func PromoteInitialAdmin(ctx context.Context) error {
	if auth.InitialAdminEmail == "" {
		return nil
	}
	return queries.PromoteUserToAdminByEmail(ctx, auth.InitialAdminEmail)
}

func IsInitialAdminEmail(email string) bool {
	return auth.InitialAdminEmail != "" &&
		strings.EqualFold(strings.TrimSpace(email), auth.InitialAdminEmail)
}

func randomState() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
