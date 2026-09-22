package handlers

import (
	"context"
	"net/http"
	"strings"

	"leiheV2/db"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func RequireRole(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		userRole := session.Get("user_role")

		if userRole == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Bitte einloggen"})
			return
		}

		roleStr := userRole.(string)
		hasAccess := roleStr == "admin"
		for _, allowed := range allowedRoles {
			if roleStr == allowed {
				hasAccess = true
				break
			}
		}

		if !hasAccess {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Keine Berechtigung"})
			return
		}
		c.Next()
	}
}

func DevLogin(c *gin.Context) {
	role := c.Query("role")
	if role != "admin" && role != "maintainer" {
		role = "user"
	}

	dummySub := "dev-dummy-" + role
	_, err := queries.UpsertUser(context.Background(), db.UpsertUserParams{
		OidcSub:   dummySub,
		Email:     role + "@fius.local",
		FirstName: "Test",
		LastName:  strings.ToUpper(role),
		Role:      db.UserRole(role),
	})

	if err != nil {
		c.String(http.StatusInternalServerError, "DB Fehler beim Dev-Login: "+err.Error())
		return
	}

	session := sessions.Default(c)
	session.Set("user_id", dummySub)
	session.Set("user_role", role)
	session.Save()
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func Logout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusTemporaryRedirect, "/")
}
