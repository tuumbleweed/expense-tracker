package echomw

import (
	"github.com/labstack/echo/v4"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

func RouteAccessLoggerMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		defer LogRouteAccess(c, tl.Info1, "Route accessed", palette.Green) // Log the visit
		LogRouteAccess(c, tl.Info, "Accessing route", palette.Blue) // Log the visit
		return next(c) // Proceed to the next handler
	}
}

// Log route access
func LogRouteAccess(c echo.Context, logLevel tl.LogLevel, actionName string, colorizer palette.Colorizer) {
	path := c.Path()
	if path == "/static*" {
		logLevel  = tl.Verbose
		colorizer = palette.CyanDim
	}
	tl.Log(logLevel, colorizer, "%s: Method='%s', Path='%s', ClientIP='%s'", actionName, c.Request().Method, c.Path(), c.RealIP())
}
