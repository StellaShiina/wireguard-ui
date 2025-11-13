package middleware

import (
	"net/http"

	"github.com/StellaShiina/wireguard-ui/auth"
	"github.com/gin-gonic/gin"
)

// AuthRequired is a middleware that checks if the user is authenticated
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from cookie
		token, err := auth.GetTokenFromCookie(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Validate token
		claims, err := auth.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set username in context
		c.Set("username", claims.Username)

		c.Next()
	}
}

// RedirectIfAuthenticated redirects to the todo list if the user is already authenticated
func RedirectIfAuthenticated() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from cookie
		token, err := auth.GetTokenFromCookie(c)
		if err != nil {
			// No token, continue to login page
			c.Next()
			return
		}

		// Validate token
		_, err = auth.ValidateToken(token)
		if err != nil {
			// Invalid token, continue to login page
			c.Next()
			return
		}

		// User is authenticated, redirect to todo list
		c.Redirect(http.StatusFound, "/")
		c.Abort()
	}
}
