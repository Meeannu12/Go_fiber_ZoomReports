package main

import (
	"fmt"
	"go_fiber_Zoom_Report/config"
	"go_fiber_Zoom_Report/routes"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/joho/godotenv"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// Get port from .env
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000" // fallback default
	}
	fmt.Println("new port", port)
	// Create a new Fiber app

	// config.ConnectMongoDB()
	app := fiber.New()

	config.ConnectMongo()

	routes.ReportRoutes(app)

	app.Get("/api/v1", func(c *fiber.Ctx) error {
		return c.SendString("Hello Fiber")
	})

	// --- GET with Params ---
	app.Get("/user/:id", func(c *fiber.Ctx) error {
		id := c.Params("id") // get route param e.g. /user/123
		return c.JSON(fiber.Map{"message": "User found", "userId": id})
	})

	// --- GET with Query Params ---
	app.Get("/search", func(c *fiber.Ctx) error {
		query := c.Query("q")        // e.g. /search?q=golang
		page := c.Query("page", "1") // default value = 1
		return c.JSON(fiber.Map{"query": query, "page": page})
	})

	// --- POST Request (body data) ---
	app.Post("/user", func(c *fiber.Ctx) error {
		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		var user User
		if err := c.BodyParser(&user); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}

		return c.JSON(fiber.Map{
			"message": "User created successfully!",
			"user":    user,
		})
	})

	// --- PUT Request (update data) ---
	app.Put("/user/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")

		type UpdateData struct {
			Name string `json:"name"`
		}
		var data UpdateData
		if err := c.BodyParser(&data); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}

		return c.JSON(fiber.Map{
			"message": "User updated successfully",
			"id":      id,
			"newName": data.Name,
		})
	})

	// --- DELETE Request ---
	app.Delete("/user/:id", func(c *fiber.Ctx) error {
		id := c.Params("id")
		return c.JSON(fiber.Map{
			"message": "User deleted successfully",
			"userId":  id,
		})
	})

	app.Listen(":" + port)
}
