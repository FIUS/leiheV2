package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"leiheV2/db"

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

func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabe"})
		return
	}

	role, oidcSub := req.Role, strings.TrimSpace(req.OidcSub)
	if role != "admin" && role != "maintainer" {
		role = "user"
	}
	if oidcSub == "" {
		oidcSub = "manual-" + strings.TrimSpace(req.Email)
	}

	_, _ = queries.CreateUser(context.Background(), db.CreateUserParams{
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
		Email:     strings.TrimSpace(req.Email),
		Role:      db.UserRole(role),
		OidcSub:   oidcSub,
	})
	c.Redirect(http.StatusFound, "/admin?tab=nutzer")
}

func UpdateUser(c *gin.Context) {
	var req UpdateUserRequest
	if err := c.ShouldBind(&req); err != nil {
		c.String(http.StatusBadRequest, "Ungültige Eingabe")
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

	_ = queries.UpdateUser(context.Background(), db.UpdateUserParams{
		ID:        req.ID,
		FirstName: strings.TrimSpace(req.FirstName),
		LastName:  strings.TrimSpace(req.LastName),
		Email:     strings.TrimSpace(req.Email),
		Role:      db.UserRole(role),
		OidcSub:   oidcSub,
	})
	c.Redirect(http.StatusFound, "/admin?tab=nutzer")
}

func DeleteUser(c *gin.Context) {
	userID, _ := strconv.Atoi(c.PostForm("id"))
	if c.PostForm("confirm_name") == c.PostForm("real_name") {
		_ = queries.DeleteUser(context.Background(), int32(userID))
	}
	c.Redirect(http.StatusFound, "/admin?tab=nutzer")
}
