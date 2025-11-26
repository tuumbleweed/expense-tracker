// Package echomw provides Echo middlewares used across EMV services.
package echomw

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"

	tl "github.com/tuumbleweed/tintlog/logger"
	"github.com/tuumbleweed/tintlog/palette"
)

const (
	// Env var read by this middleware.
	EnvIntakeBearerToken = "EMV_INTAKE_BEARER_TOKEN"

	// Realm for WWW-Authenticate header.
	authRealm = "emv-intake"
)

var (
	tokenOnce sync.Once
	cachedTok string
)

// RequireBearerToken validates Authorization: Bearer <token> against
// the EMV_INTAKE_BEARER_TOKEN environment variable. On failure responds 401.
func RequireBearerToken(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		exp := getExpectedToken()
		if exp == "" {
			// Fail closed if not configured.
			return unauthorized(c)
		}

		auth := strings.TrimSpace(c.Request().Header.Get("Authorization"))
		if auth == "" {
			return unauthorized(c)
		}

		// Case-insensitive scheme per RFC; allow extra spaces.
		const bearer = "bearer "
		if len(auth) < len(bearer) || !strings.EqualFold(auth[:len(bearer)], bearer) {
			return unauthorized(c)
		}
		received := strings.TrimSpace(auth[len(bearer):])
		if received == "" {
			return unauthorized(c)
		}

		// Constant-time compare.
		if subtle.ConstantTimeCompare([]byte(received), []byte(exp)) != 1 {
			return unauthorized(c)
		}

		return next(c)
	}
}

func getExpectedToken() string {
	tokenOnce.Do(func() {
		cachedTok = strings.TrimSpace(os.Getenv(EnvIntakeBearerToken))
	})
	return cachedTok
}

func unauthorized(c echo.Context) error {
	LogRouteAccess(c, tl.Info, "Unauthorized access attempt", palette.Yellow) // Log the visit

	// Helpful for clients/tools; avoids browser basic-auth popups.
	c.Response().Header().Set("WWW-Authenticate", `Bearer realm="`+authRealm+`"`)
	return c.JSON(http.StatusUnauthorized, map[string]string{
		"error": "unauthorized",
	})
}
