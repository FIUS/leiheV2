package handlers

import (
	"fmt"
	stdmail "net/mail"
	"strconv"
	"strings"

	"leiheV2/db"

	"github.com/gin-gonic/gin"
)

func UpdateAdminSettings(c *gin.Context) {
	boolSetting := func(key string) (bool, bool) {
		value, err := strconv.ParseBool(c.PostForm(key))
		return value, err == nil
	}
	requireDeposit, okDeposit := boolSetting("require_deposit")
	barcodesEnabled, okBarcodes := boolSetting("module_barcodes_enabled")
	emailReceived, okEmailReceived := boolSetting("email_booking_received")
	emailReview, okEmailReview := boolSetting("email_booking_review")
	emailUpdates, okEmailUpdates := boolSetting("email_booking_updates")
	if !okDeposit || !okBarcodes || !okEmailReceived || !okEmailReview || !okEmailUpdates {
		redirectAdminNotice(c, "/admin/settings", "error", "Ungültige Einstellung.")
		return
	}
	ignoredAddresses, err := normalizeIgnoredEmailAddresses(c.PostForm("ignored_email_addresses"))
	if err != nil {
		redirectAdminNotice(c, "/admin/settings", "error", "Bitte prüfe die Liste der E-Mail-Adressen.")
		return
	}
	if err := queries.UpdateAppSettings(c.Request.Context(), db.AppSettings{
		RequireDeposit:        requireDeposit,
		ModuleBarcodesEnabled: barcodesEnabled,
		EmailBookingReceived:  emailReceived,
		EmailBookingReview:    emailReview,
		EmailBookingUpdates:   emailUpdates,
		IgnoredEmailAddresses: ignoredAddresses,
	}); err != nil {
		redirectAdminNotice(c, "/admin/settings", "error", "Einstellung konnte nicht gespeichert werden.")
		return
	}
	redirectAdminNotice(c, "/admin/settings", "success", "Einstellungen wurden gespeichert.")
}

func normalizeIgnoredEmailAddresses(value string) (string, error) {
	var addresses []string
	seen := make(map[string]struct{})
	for _, address := range strings.FieldsFunc(value, func(r rune) bool {
		return r == '\n' || r == ',' || r == ';'
	}) {
		address = strings.TrimSpace(address)
		if address == "" {
			continue
		}
		parsed, err := stdmail.ParseAddress(address)
		if err != nil || parsed.Address != address || !strings.Contains(address, "@") {
			return "", fmt.Errorf("ungültige E-Mail-Adresse")
		}
		address = strings.ToLower(address)
		if _, exists := seen[address]; exists {
			continue
		}
		seen[address] = struct{}{}
		addresses = append(addresses, address)
	}
	return strings.Join(addresses, "\n"), nil
}
