package routes

import (
	"go_fiber_Zoom_Report/controller"

	"github.com/gofiber/fiber/v2"
)

func ReportRoutes(app *fiber.App) {
	app.Get("/report", controller.GetCombineReport)
	app.Get("/DailyReport", controller.DayByReportEveryStaff)
}
