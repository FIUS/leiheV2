package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func UpdateAdminSettings(c *gin.Context) {
	requireDeposit, err := strconv.ParseBool(c.PostForm("require_deposit"))
	if err != nil {
		c.Redirect(http.StatusFound, "/admin/settings?error=Ungueltige-Einstellung")
		return
	}
	if err := queries.UpdateRequireDeposit(c.Request.Context(), requireDeposit); err != nil {
		c.String(http.StatusInternalServerError, "Einstellung konnte nicht gespeichert werden")
		return
	}
	c.Redirect(http.StatusFound, "/admin/settings?success=Gespeichert")
}
