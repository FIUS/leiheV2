package main

import (
	"leiheV2/handlers"
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.Engine) {
	// Authentication
	r.GET("/login", handlers.RenderLoginPage)
	r.GET("/register", handlers.RenderRegisterPage)
	r.GET("/auth/stuvus/login", handlers.OIDCLogin)
	r.GET("/auth/stuvus/callback", handlers.OIDCCallback)
	r.POST("/auth/local/login", handlers.LocalLogin)
	r.POST("/auth/local/register", handlers.RegisterLocal)
	r.GET("/auth/local/verify", handlers.VerifyEmail)
	r.GET("/logout", handlers.Logout)

	// HTML Pages
	r.GET("/", handlers.RenderIndex)
	r.GET("/games", handlers.RenderGamesCatalog)
	r.GET("/setup-bilder", handlers.RequireRole("admin", "maintainer"), handlers.RenderSetupBilder)
	r.GET("/checkout", handlers.RenderCheckoutPage)

	// Admin HTML Pages
	admin := r.Group("/admin")
	admin.Use(handlers.RequireRole("maintainer", "admin"))
	{
		admin.GET("", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/admin/dashboard") })
		admin.GET("/dashboard", handlers.RenderAdminDashboard)
		admin.GET("/games", handlers.RenderAdminGames)
		admin.GET("/bookings", handlers.RenderAdminBookings)

		admin.GET("/users", handlers.RequireRole("admin"), handlers.RenderAdminUsers)
		admin.GET("/settings", handlers.RequireRole("admin"), handlers.RenderAdminSettings)
	}

	// API: Items
	r.POST("/api/items", handlers.RequireRole("admin", "maintainer"), handlers.CreateItem)
	r.POST("/api/items/update", handlers.RequireRole("admin", "maintainer"), handlers.UpdateItem)
	r.POST("/api/items/delete", handlers.RequireRole("admin", "maintainer"), handlers.DeleteItem)
	r.POST("/api/categories", handlers.RequireRole("admin", "maintainer"), handlers.CreateCategory)
	r.POST("/api/categories/delete", handlers.RequireRole("admin", "maintainer"), handlers.DeleteCategory)
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
