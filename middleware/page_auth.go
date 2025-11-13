package middleware

import (
	"net/http"

	"github.com/StellaShiina/wireguard-ui/auth"
	"github.com/gin-gonic/gin"
)

// AuthPageRequired ensures user is authenticated; otherwise redirects to /login.
// This is for HTML page routes, distinct from JSON API which returns 401 JSON.
func AuthPageRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := auth.GetTokenFromCookie(c)
		if err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		if _, err := auth.ValidateToken(token); err != nil {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}
