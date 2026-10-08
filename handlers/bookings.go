package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"leiheV2/db"

	"github.com/gin-gonic/gin"
)

func RenderUserBookings(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	bookings, err := queries.ListBookingsForUser(c.Request.Context(), user.ID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Buchungen konnten nicht geladen werden")
		return
	}
	c.HTML(http.StatusOK, "user_bookings.html", pageData(c, gin.H{
		"Bookings": bookings,
		"Success":  c.Query("success"),
		"Error":    c.Query("error"),
	}))
}

func CancelUserBooking(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	bookingID, err := strconv.ParseInt(c.PostForm("id"), 10, 32)
	if err != nil {
		c.Redirect(http.StatusFound, "/bookings?error=Ungueltige-Buchung")
		return
	}
	bookingID32 := int32(bookingID)
	if err := queries.CancelUserBooking(c.Request.Context(), user.ID, bookingID32); err != nil {
		c.Redirect(http.StatusFound, "/bookings?error=Diese-Buchung-kann-nicht-mehr-storniert-werden")
		return
	}
	details, err := queries.GetBookingEmailDetailsByID(c.Request.Context(), bookingID32)
	if err != nil {
		logMailError("booking update details", err)
		c.Redirect(http.StatusFound, "/bookings?error=email-failed")
		return
	}
	if details.Email.Valid {
		if err := sendBookingUpdateEmail(details, "storniert"); err != nil {
			logMailError("booking update", err)
			c.Redirect(http.StatusFound, "/bookings?error=email-failed")
			return
		}
	} else {
		log.Printf("booking cancellation email skipped: no recipient (booking id=%d)", bookingID32)
	}
	c.Redirect(http.StatusFound, "/bookings?success=Buchung-storniert")
}

func UpdateUserBooking(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.Redirect(http.StatusFound, "/login")
		return
	}
	bookingID, err := strconv.ParseInt(c.PostForm("id"), 10, 32)
	if err != nil {
		c.Redirect(http.StatusFound, "/bookings?error=Ungueltige-Buchung")
		return
	}
	startDate, startErr := time.Parse("2006-01-02", c.PostForm("start_date"))
	endDate, endErr := time.Parse("2006-01-02", c.PostForm("end_date"))
	if startErr != nil || endErr != nil || endDate.Before(startDate) {
		c.Redirect(http.StatusFound, "/bookings?error=Ungueltiger-Zeitraum")
		return
	}
	err = queries.UpdateUserBookingDates(c.Request.Context(), user.ID, int32(bookingID), startDate, endDate)
	switch {
	case errors.Is(err, db.ErrBookingConflict):
		c.Redirect(http.StatusFound, "/bookings?error=Der-gewaehlte-Zeitraum-ist-nicht-verfuegbar")
	case errors.Is(err, db.ErrBookingNotEditable):
		c.Redirect(http.StatusFound, "/bookings?error=Nur-unbestaetigte-Buchungen-koennen-geaendert-werden")
	case err != nil:
		c.Redirect(http.StatusFound, "/bookings?error=Speichern-fehlgeschlagen")
	default:
		details, detailsErr := queries.GetBookingEmailDetailsByID(c.Request.Context(), int32(bookingID))
		if detailsErr != nil {
			logMailError("booking update details", detailsErr)
			c.Redirect(http.StatusFound, "/bookings?error=email-failed")
			return
		}
		if details.Email.Valid {
			if mailErr := sendBookingUpdateEmail(details, "geändert"); mailErr != nil {
				logMailError("booking update", mailErr)
				c.Redirect(http.StatusFound, "/bookings?error=email-failed")
				return
			}
		} else {
			log.Printf("booking date update email skipped: no recipient (booking id=%d)", bookingID)
		}
		c.Redirect(http.StatusFound, "/bookings?success=Buchung-geaendert")
	}
}

func ReviewPendingBooking(c *gin.Context) {
	bookingID, err := strconv.ParseInt(c.PostForm("id"), 10, 32)
	if err != nil || bookingID <= 0 {
		redirectAdminNotice(c, "/admin/bookings", "error", "Ungültige Buchung.")
		return
	}
	status := c.PostForm("status")
	if status != "confirmed" && status != "cancelled" {
		redirectAdminNotice(c, "/admin/bookings", "error", "Ungültiger Buchungsstatus.")
		return
	}
	bookingID32 := int32(bookingID)
	if err := queries.ReviewPendingBooking(c.Request.Context(), bookingID32, status); err != nil {
		if errors.Is(err, db.ErrBookingNotEditable) {
			redirectAdminNotice(c, "/admin/bookings", "error", "Diese Buchung wurde bereits bearbeitet.")
			return
		}
		redirectAdminNotice(c, "/admin/bookings", "error", "Buchungsstatus konnte nicht aktualisiert werden.")
		return
	}

	details, err := queries.GetBookingEmailDetailsByID(c.Request.Context(), bookingID32)
	if err != nil {
		logMailError("booking review details", err)
		redirectAdminNotice(c, "/admin/bookings", "error", "Buchung geändert, aber E-Mail konnte nicht vorbereitet werden.")
		return
	}
	if details.Email.Valid {
		if err := sendBookingReviewEmail(details, status == "confirmed"); err != nil {
			logMailError("booking review", err)
			redirectAdminNotice(c, "/admin/bookings", "error", "Buchung geändert, aber Benachrichtigungs-E-Mail konnte nicht versendet werden.")
			return
		}
	} else {
		log.Printf("booking review email skipped: no recipient (booking id=%d)", bookingID32)
	}
	redirectAdminNotice(c, "/admin/bookings", "success", "Buchungsanfrage wurde bearbeitet.")
}

func CancelConfirmedBooking(c *gin.Context) {
	bookingID, err := strconv.ParseInt(c.PostForm("id"), 10, 32)
	if err != nil || bookingID <= 0 {
		redirectAdminNotice(c, "/admin/bookings", "error", "Ungültige Buchung.")
		return
	}
	bookingID32 := int32(bookingID)
	if err := queries.CancelConfirmedBooking(c.Request.Context(), bookingID32); err != nil {
		if errors.Is(err, db.ErrBookingNotEditable) {
			redirectAdminNotice(c, "/admin/bookings", "error", "Diese Buchung ist nicht mehr bestätigt.")
			return
		}
		redirectAdminNotice(c, "/admin/bookings", "error", "Buchung konnte nicht storniert werden.")
		return
	}

	details, err := queries.GetBookingEmailDetailsByID(c.Request.Context(), bookingID32)
	if err != nil {
		logMailError("confirmed booking cancellation details", err)
		redirectAdminNotice(c, "/admin/bookings", "error", "Buchung storniert, aber E-Mail konnte nicht vorbereitet werden.")
		return
	}
	if details.Email.Valid {
		if err := sendBookingUpdateEmail(details, "vom Team storniert"); err != nil {
			logMailError("confirmed booking cancellation", err)
			redirectAdminNotice(c, "/admin/bookings", "error", "Buchung storniert, aber Benachrichtigungs-E-Mail konnte nicht versendet werden.")
			return
		}
	} else {
		log.Printf("confirmed booking cancellation email skipped: no recipient (booking id=%d)", bookingID32)
	}
	redirectAdminNotice(c, "/admin/bookings", "success", "Buchung wurde storniert.")
}
