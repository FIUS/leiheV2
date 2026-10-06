package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	stdmail "net/mail"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"leiheV2/db"
	"leiheV2/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func RenderStaffLoans(c *gin.Context) {
	settings, err := queries.GetAppSettings(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Einstellungen konnten nicht geladen werden")
		return
	}
	c.HTML(http.StatusOK, "staff_loans.html", pageData(c, gin.H{"RequireDeposit": settings.RequireDeposit}))
}

func validateStaffLoanDeposit(deposit string, requireDeposit bool) (string, error) {
	deposit = strings.TrimSpace(deposit)
	if requireDeposit && deposit == "" {
		return "", errors.New("Bitte das abgegebene Pfand erfassen")
	}
	if utf8.RuneCountInString(deposit) > 1000 {
		return "", errors.New("Pfandangabe darf höchstens 1000 Zeichen lang sein")
	}
	return deposit, nil
}

func SearchStaffLoans(c *gin.Context) {
	mode := c.Query("mode")
	if mode != "issue" && mode != "return" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Suche"})
		return
	}
	bookings, err := queries.SearchStaffLoanBookings(c.Request.Context(), c.Query("q"), mode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchungen konnten nicht gesucht werden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookings": bookings})
}

func SearchStaffLoanItems(c *gin.Context) {
	endDate, err := staffLoanEndDate(c.Query("end_date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges Rückgabedatum"})
		return
	}
	items, err := queries.SearchStaffLoanItems(c.Request.Context(), c.Query("q"), endDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Spiele konnten nicht gesucht werden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func LookupStaffLoanSystemID(c *gin.Context) {
	rawID := strings.TrimSpace(c.Query("id"))
	id, err := strconv.ParseInt(rawID, 10, 32)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bitte eine gültige System-ID eingeben"})
		return
	}
	endDate, err := staffLoanEndDate(c.Query("end_date"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges Rückgabedatum"})
		return
	}
	item, err := queries.FindStaffLoanItemBySystemID(c.Request.Context(), int32(id), endDate)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Kein Spiel mit dieser System-ID gefunden"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "System-ID konnte nicht gesucht werden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"item": item})
}

func staffLoanEndDate(date string) (time.Time, error) {
	endDate, err := time.ParseInLocation("2006-01-02", date, time.Local)
	if err != nil || date < time.Now().Format("2006-01-02") {
		return time.Time{}, errors.New("invalid end date")
	}
	return endDate.AddDate(0, 0, 1).Add(-time.Nanosecond), nil
}

type issuePrebookingRequest struct {
	BookingID int32   `json:"booking_id" binding:"required,gt=0"`
	ItemIDs   []int32 `json:"item_ids" binding:"required,min=1"`
	Deposit   string  `json:"deposit"`
}

func IssuePrebookedBooking(c *gin.Context) {
	var req issuePrebookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Buchung und mindestens ein Spiel auswählen"})
		return
	}
	settings, err := queries.GetAppSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Einstellungen konnten nicht geladen werden"})
		return
	}
	deposit, err := validateStaffLoanDeposit(req.Deposit, settings.RequireDeposit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := queries.IssuePrebookedLoan(c.Request.Context(), req.BookingID, req.ItemIDs, deposit); err != nil {
		if errors.Is(err, db.ErrBookingNotEditable) {
			c.JSON(http.StatusConflict, gin.H{"error": "Buchung ist nicht mehr ausstehend oder die Spielauswahl ist ungültig"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ausgabe konnte nicht gespeichert werden"})
		return
	}
	if err := notifyStaffLoanChange(c, req.BookingID, "ausgegeben"); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Spiele wurden ausgegeben", "warning": "E-Mail konnte nicht versendet werden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Spiele wurden ausgegeben"})
}

type spontaneousLoanRequest struct {
	ItemIDs []int32 `json:"item_ids" binding:"required,min=1"`
	Name    string  `json:"name" binding:"required"`
	Email   string  `json:"email"`
	Deposit string  `json:"deposit"`
	EndDate string  `json:"end_date" binding:"required"`
}

func CreateSpontaneousStaffLoan(c *gin.Context) {
	var req spontaneousLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name, Rückgabedatum und mindestens ein Spiel sind erforderlich"})
		return
	}
	settings, err := queries.GetAppSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Einstellungen konnten nicht geladen werden"})
		return
	}
	deposit, err := validateStaffLoanDeposit(req.Deposit, settings.RequireDeposit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	endDate, err := time.ParseInLocation("2006-01-02", req.EndDate, time.Local)
	if err != nil || req.EndDate < time.Now().Format("2006-01-02") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges Rückgabedatum"})
		return
	}
	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bitte den Namen der ausleihenden Person eingeben"})
		return
	}
	if email != "" {
		if address, parseErr := stdmail.ParseAddress(email); parseErr != nil || address.Address != email {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige E-Mail-Adresse"})
			return
		}
	}
	endDate = endDate.AddDate(0, 0, 1).Add(-time.Nanosecond)
	itemIDs := make([]int32, 0, len(req.ItemIDs))
	seen := make(map[int32]struct{}, len(req.ItemIDs))
	for _, id := range req.ItemIDs {
		if id <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Spielauswahl"})
			return
		}
		if _, exists := seen[id]; exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Ein Spiel wurde mehrfach ausgewählt"})
			return
		}
		seen[id] = struct{}{}
		itemIDs = append(itemIDs, id)
	}
	id, ok := sessions.Default(c).Get("user_id").(int32)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Bitte erneut anmelden"})
		return
	}
	token, err := utils.GenerateBookingToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchungscode konnte nicht erzeugt werden"})
		return
	}
	bookingID, err := queries.CreateSpontaneousLoan(c.Request.Context(), token, id,
		name, email, deposit, endDate, itemIDs)
	if err != nil {
		if errors.Is(err, db.ErrBookingConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "Mindestens eines der Spiele ist im Zeitraum bereits reserviert"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Spontane Ausgabe konnte nicht gespeichert werden"})
		return
	}
	if email != "" {
		if err := notifyStaffLoanChange(c, bookingID, "ausgegeben"); err != nil {
			c.JSON(http.StatusOK, gin.H{"message": "Spiele wurden ausgegeben", "token": token, "warning": "E-Mail konnte nicht versendet werden"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "Spiele wurden ausgegeben", "token": token})
}

type returnStaffLoanRequest struct {
	BookingID int32 `json:"booking_id" binding:"required,gt=0"`
}

func ReturnStaffLoan(c *gin.Context) {
	var req returnStaffLoanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Buchung"})
		return
	}
	if err := queries.ReturnStaffLoan(c.Request.Context(), req.BookingID); err != nil {
		if errors.Is(err, db.ErrBookingNotEditable) {
			c.JSON(http.StatusConflict, gin.H{"error": "Diese Buchung ist nicht mehr aktiv"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Rücknahme konnte nicht gespeichert werden"})
		return
	}
	if err := notifyStaffLoanChange(c, req.BookingID, "zurückgenommen"); err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Buchung zurückgenommen", "warning": "E-Mail konnte nicht versendet werden"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Buchung zurückgenommen"})
}

func notifyStaffLoanChange(c *gin.Context, bookingID int32, change string) error {
	details, err := queries.GetBookingEmailDetailsByID(c.Request.Context(), bookingID)
	if err != nil {
		logMailError("staff loan email details", err)
		return err
	}
	if !details.Email.Valid || strings.TrimSpace(details.Email.String) == "" {
		logMailError("staff loan email", errors.New("no recipient address"))
		return nil
	}
	if err := sendBookingUpdateEmail(details, change); err != nil {
		logMailError("staff loan update", err)
		return err
	}
	return nil
}
