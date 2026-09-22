package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"leiheV2/db"
	"leiheV2/utils"

	"github.com/gin-gonic/gin"
)

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültige Daten", "details": err.Error()})
		return
	}

	startDate, err := time.Parse("2006-01-02", req.StartDate)
	endDate, err2 := time.Parse("2006-01-02", req.EndDate)
	if err != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ungültiges Datumsformat"})
		return
	}

	token, err := utils.GenerateBookingToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Token Fehler"})
		return
	}

	ctx := context.Background()

	// String Pointer Logik
	var guestName, guestEmail sql.NullString
	if req.GuestName != "" {
		guestName = sql.NullString{String: req.GuestName, Valid: true}
	}
	if req.GuestEmail != "" {
		guestEmail = sql.NullString{String: req.GuestEmail, Valid: true}
	}

	var booking db.Booking
	err = queries.InTx(ctx, func(txQueries *db.Queries) error {
		var txErr error
		booking, txErr = txQueries.CreateBooking(ctx, db.CreateBookingParams{
			PublicToken: token,
			GuestName:   guestName,
			GuestEmail:  guestEmail,
			StartDate:   startDate,
			EndDate:     endDate,
			Status:      db.BookingStatusPending,
		})
		if txErr != nil {
			return txErr
		}

		for _, gameID := range req.Games {
			if txErr := txQueries.AddBookingItem(ctx, db.AddBookingItemParams{
				BookingID: booking.ID,
				ItemID:    gameID,
			}); txErr != nil {
				return txErr
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchung fehlgeschlagen"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Erfolgreich", "token": token})
}
