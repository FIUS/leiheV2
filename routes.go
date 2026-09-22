package main

import (
	"leiheV2/handlers"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Auth & Dev
	r.GET("/dev-login", handlers.DevLogin)
	r.GET("/logout", handlers.Logout)

	// HTML Pages
	r.GET("/", handlers.RenderIndex)
	r.GET("/games", handlers.RenderGamesCatalog)
	r.GET("/admin", handlers.RequireRole("admin", "maintainer"), handlers.RenderAdmin)
	r.GET("/setup-bilder", handlers.RequireRole("admin", "maintainer"), handlers.RenderSetupBilder)
	r.GET("/checkout", handlers.RenderCheckoutPage) // NEU!

	// API: Items
	r.POST("/api/items", handlers.RequireRole("admin", "maintainer"), handlers.CreateItem)
	r.POST("/api/items/update", handlers.RequireRole("admin", "maintainer"), handlers.UpdateItem)
	r.POST("/api/items/delete", handlers.RequireRole("admin", "maintainer"), handlers.DeleteItem)
	r.GET("/api/items/details", handlers.GetItemDetails)
	r.GET("/api/items/history", handlers.RequireRole("admin", "maintainer"), handlers.GetItemHistory)
	r.POST("/setup-bilder", handlers.RequireRole("admin", "maintainer"), handlers.UpdateItemImage)

	// API: Checkout (Buchungsprozess)
	r.POST("/api/checkout", handlers.HandleCreateBooking) // NEU!

	// API: Users
	r.POST("/api/users", handlers.RequireRole("admin"), handlers.CreateUser)
	r.POST("/api/users/update", handlers.RequireRole("admin"), handlers.UpdateUser)
	r.POST("/api/users/delete", handlers.RequireRole("admin"), handlers.DeleteUser)

	// API: BoardGameGeek
	r.GET("/api/bgg/search", handlers.RequireRole("admin", "maintainer"), handlers.SearchBGG)
	r.GET("/api/bgg/details", handlers.RequireRole("admin", "maintainer"), handlers.GetBGGDetails)
}
