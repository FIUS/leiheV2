package handlers

import (
	"context"
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
	var guestName, guestEmail *string
	if req.GuestName != "" {
		guestName = &req.GuestName
		guestEmail = &req.GuestEmail
	}

	booking, err := queries.CreateBooking(ctx, db.CreateBookingParams{
		PublicToken: token,
		GuestName:   guestName,
		GuestEmail:  guestEmail,
		StartDate:   startDate,
		EndDate:     endDate,
		Status:      db.BookingStatusPending,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Buchung fehlgeschlagen"})
		return
	}

	for _, gameID := range req.Games {
		_ = queries.AddBookingItem(ctx, db.AddBookingItemParams{
			BookingID: booking.ID,
			ItemID:    gameID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"message": "Erfolgreich", "token": token})
}
