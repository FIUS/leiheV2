package handlers

import (
	"errors"
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
		}
		c.Redirect(http.StatusFound, "/bookings?success=Buchung-geaendert")
	}
}

func ReviewPendingBooking(c *gin.Context) {
	bookingID, err := strconv.ParseInt(c.PostForm("id"), 10, 32)
	if err != nil || bookingID <= 0 {
		c.Redirect(http.StatusFound, "/admin/bookings?error=invalid-booking")
		return
	}
	status := c.PostForm("status")
	if status != "confirmed" && status != "cancelled" {
		c.Redirect(http.StatusFound, "/admin/bookings?error=invalid-status")
		return
	}
	bookingID32 := int32(bookingID)
	if err := queries.ReviewPendingBooking(c.Request.Context(), bookingID32, status); err != nil {
		if errors.Is(err, db.ErrBookingNotEditable) {
			c.Redirect(http.StatusFound, "/admin/bookings?error=booking-not-pending")
			return
		}
		c.String(http.StatusInternalServerError, "Buchungsstatus konnte nicht aktualisiert werden")
		return
	}

	details, err := queries.GetBookingEmailDetailsByID(c.Request.Context(), bookingID32)
	if err != nil {
		logMailError("booking review details", err)
		c.Redirect(http.StatusFound, "/admin/bookings?error=email-failed")
		return
	}
	if details.Email.Valid {
		change := "bestätigt"
		if status == "cancelled" {
			change = "abgelehnt und storniert"
		}
		if err := sendBookingUpdateEmail(details, change); err != nil {
			logMailError("booking review", err)
			c.Redirect(http.StatusFound, "/admin/bookings?error=email-failed")
			return
		}
	}
	c.Redirect(http.StatusFound, "/admin/bookings?success=booking-reviewed")
}
