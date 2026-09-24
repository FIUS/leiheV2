package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"leiheV2/db"
	"leiheV2/utils"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

var errBookingConflict = errors.New("booking conflict")

type CheckoutRequest struct {
	Games      []int32 `json:"games" binding:"required,min=1"`
	StartDate  string  `json:"startDate" binding:"required"`
	EndDate    string  `json:"endDate" binding:"required"`
	GuestName  string  `json:"guestName"`
	GuestEmail string  `json:"guestEmail"`
}

func HandleCreateBooking(c *gin.Context) {
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Eingabedaten", "details": err.Error()})
		return
	}

	startDate, err1 := time.Parse("2006-01-02", req.StartDate)
	endDate, err2 := time.Parse("2006-01-02", req.EndDate)
	if err1 != nil || err2 != nil || endDate.Before(startDate) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiger Zeitraum"})
		return
	}

	ctx := context.Background()

	println("Spiele-IDs:", fmt.Sprintf("%v", req.Games))
	println("Start-Datum:", startDate.Format("2006-01-02"))
	println("End-Datum:", endDate.Format("2006-01-02"))
	// Token generieren
	token, err := utils.GenerateBookingToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token-Generierung fehlgeschlagen"})
		return
	}

	var guestName, guestEmail sql.NullString
	if strings.TrimSpace(req.GuestName) != "" {
		guestName = sql.NullString{String: strings.TrimSpace(req.GuestName), Valid: true}
	}
	if strings.TrimSpace(req.GuestEmail) != "" {
		guestEmail = sql.NullString{String: strings.TrimSpace(req.GuestEmail), Valid: true}
	}
	session := sessions.Default(c)
	var userID sql.NullInt32
	var recipientEmail, recipientName string
	if id, ok := session.Get("user_id").(int32); ok {
		userID = sql.NullInt32{Int32: id, Valid: true}
		user, userErr := queries.GetUser(ctx, id)
		if userErr == nil {
			recipientEmail = user.Email
			recipientName = user.FirstName
		}
	}
	if recipientEmail == "" && guestEmail.Valid {
		recipientEmail = guestEmail.String
		recipientName = guestName.String
	}

	// Sperren, prüfen und speichern müssen in derselben Transaktion erfolgen,
	// damit parallele Buchungen nicht dieselben Spiele reservieren können.
	var createdToken string
	var collisions []db.CheckItemCollisionsRow
	err = queries.InTx(ctx, func(qTx *db.Queries) error {
		if err := qTx.LockItemsForBooking(ctx, req.Games); err != nil {
			return err
		}

		var err error
		collisions, err = qTx.CheckItemCollisions(ctx, db.CheckItemCollisionsParams{
			Column1:   req.Games,
			StartDate: startDate,
			EndDate:   endDate,
		})
		if err != nil {
			return err
		}
		if len(collisions) > 0 {
			return errBookingConflict
		}

		booking, err := qTx.CreateBooking(ctx, db.CreateBookingParams{
			PublicToken: token,
			UserID:      userID,
			GuestName:   guestName,
			GuestEmail:  guestEmail,
			StartDate:   startDate,
			EndDate:     endDate,
			Status:      db.BookingStatusPending,
		})
		if err != nil {
			return err
		}

		for _, gameID := range req.Games {
			err = qTx.AddBookingItem(ctx, db.AddBookingItemParams{
				BookingID: booking.ID,
				ItemID:    gameID,
			})
			if err != nil {
				return err
			}
		}

		createdToken = booking.PublicToken
		return nil
	})

	if err != nil {
		if errors.Is(err, errBookingConflict) {
			var conflictNames []string
			for _, col := range collisions {
				conflictNames = append(conflictNames, col.Name)
			}
			c.JSON(http.StatusConflict, gin.H{
				"error":     "Ein oder mehrere Spiele sind im gewählten Zeitraum bereits reserviert.",
				"conflicts": conflictNames,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchung konnte nicht gespeichert werden"})
		return
	}
	if recipientEmail != "" {
		details, detailsErr := queries.GetBookingEmailDetails(ctx, createdToken)
		if detailsErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchungs-E-Mail konnte nicht vorbereitet werden"})
			return
		}
		if mailErr := sendOrderEmail(recipientEmail, recipientName, details); mailErr != nil {
			logMailError("order", mailErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchung gespeichert, aber Bestätigungs-E-Mail konnte nicht versendet werden", "token": createdToken})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Erfolgreich reserviert",
		"token":   createdToken,
	})
}
