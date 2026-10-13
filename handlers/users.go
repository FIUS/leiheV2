package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"leiheV2/db"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type CreateUserRequest struct {
	FirstName string `form:"first_name" binding:"required"`
	LastName  string `form:"last_name" binding:"required"`
	Email     string `form:"email" binding:"required"`
	Role      string `form:"role"`
	OidcSub   string `form:"oidc_sub"`
}

type UpdateUserRequest struct {
	ID        int32  `form:"id" binding:"required"`
	FirstName string `form:"first_name" binding:"required"`
	LastName  string `form:"last_name" binding:"required"`
	Email     string `form:"email" binding:"required"`
	Role      string `form:"role" binding:"required"`
	OidcSub   string `form:"oidc_sub"`
}

type UpdateUserSettingsRequest struct {
	FirstName string `form:"first_name" binding:"required"`
	LastName  string `form:"last_name" binding:"required"`
	Email     string `form:"email" binding:"required"`
}

func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		redirectAdminNotice(c, "/admin/users", "error", "Ungültige Eingabe.")
		return
	}

	role, oidcSub := req.Role, strings.TrimSpace(req.OidcSub)
	if role != "admin" && role != "maintainer" {
		role = "user"
	}
	if oidcSub == "" {
		oidcSub = "manual-" + strings.TrimSpace(req.Email)
	}

	if _, err := queries.CreateUser(c.Request.Context(), db.CreateUserParams{
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
		Email:     strings.TrimSpace(req.Email),
		Role:      db.UserRole(role),
		OidcSub:   oidcSub,
	}); err != nil {
		redirectAdminNotice(c, "/admin/users", "error", "Nutzer konnte nicht angelegt werden.")
		return
	}
	redirectAdminNotice(c, "/admin/users", "success", "Nutzer wurde angelegt.")
}

func UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		redirectAdminNotice(c, "/admin/users", "error", "Ungültige Eingabe.")
		return
	}

	role, oidcSub := req.Role, strings.TrimSpace(req.OidcSub)
	if role != "admin" && role != "maintainer" {
		role = "user"
	}
	if oidcSub == "" {
		if existing, err := queries.GetUser(context.Background(), req.ID); err == nil {
			oidcSub = existing.OidcSub
		} else {
			oidcSub = "manual-" + strings.TrimSpace(req.Email)
		}
	}
	existing, err := queries.GetUser(context.Background(), req.ID)
	if err != nil {
		redirectAdminNotice(c, "/admin/users", "error", "Nutzer nicht gefunden.")
		return
	}
	if isOIDCUser(existing) {
		req.FirstName = existing.FirstName
		req.LastName = existing.LastName
		req.Email = existing.Email
		oidcSub = existing.OidcSub
	}

	if err := queries.UpdateUser(context.Background(), db.UpdateUserParams{
		ID:        req.ID,
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
		Email:     strings.TrimSpace(req.Email),
		Role:      db.UserRole(role),
		OidcSub:   oidcSub,
	}); err != nil {
		redirectAdminNotice(c, "/admin/users", "error", "Nutzerrolle konnte nicht gespeichert werden.")
		return
	}
	redirectAdminNotice(c, "/admin/users", "success", "Nutzer wurde gespeichert.")
}

func UpdateUserSettings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	if isOIDCUser(user) {
		c.Redirect(http.StatusFound, "/settings?error=OIDC-Konten-koennen-diese-Daten-nicht-aendern")
		return
	}
	var req UpdateUserSettingsRequest
	if err := c.ShouldBind(&req); err != nil {
		c.Redirect(http.StatusFound, "/settings?error=Ungueltige-Eingabe")
		return
	}
	if err := queries.UpdateUser(context.Background(), db.UpdateUserParams{
		ID: user.ID, FirstName: strings.TrimSpace(req.FirstName),
		LastName: strings.TrimSpace(req.LastName), Email: strings.TrimSpace(req.Email),
		Role: user.Role, OidcSub: user.OidcSub,
	}); err != nil {
		c.Redirect(http.StatusFound, "/settings?error=Speichern-fehlgeschlagen")
		return
	}
	c.Redirect(http.StatusFound, "/settings?success=Gespeichert")
}

func isOIDCUser(user db.User) bool {
	sub := strings.TrimSpace(user.OidcSub)
	return sub != "" && !strings.HasPrefix(sub, "local-") && !strings.HasPrefix(sub, "manual-")
}

func DeleteUser(c *gin.Context) {
	userID, _ := strconv.Atoi(c.PostForm("id"))
	if c.PostForm("confirm_name") != c.PostForm("real_name") {
		redirectAdminNotice(c, "/admin/users", "error", "Bestätigungsname stimmt nicht überein.")
		return
	}
	if err := queries.DeleteUser(c.Request.Context(), int32(userID)); err != nil {
		redirectAdminNotice(c, "/admin/users", "error", "Nutzer konnte nicht gelöscht werden.")
		return
	}
	redirectAdminNotice(c, "/admin/users", "success", "Nutzer wurde gelöscht.")
}

func DeleteOwnAccount(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	if err := queries.DeleteUserAccount(c.Request.Context(), user.ID); err != nil {
		if errors.Is(err, db.ErrUserHasActiveLoans) {
			c.Redirect(http.StatusFound, "/settings?error=Konto-kann-nicht-geloescht-werden-solange-Ausleihen-aktiv-sind")
			return
		}
		c.Redirect(http.StatusFound, "/settings?error=Konto-konnte-nicht-geloescht-werden")
		return
	}
	session := sessions.Default(c)
	session.Clear()
	if err := session.Save(); err != nil {
		c.String(http.StatusInternalServerError, "Sitzung konnte nicht beendet werden")
		return
	}
	c.Redirect(http.StatusFound, "/")
}
