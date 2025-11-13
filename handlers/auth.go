package handlers

import (
	"net/http"

	"github.com/StellaShiina/wireguard-ui/auth"
	"github.com/gin-gonic/gin"
)

// LoginHandler handles user login
func LoginHandler(c *gin.Context) {
	var user auth.User

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate user credentials
	if !auth.ValidateCredentials(user) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT token
	token, err := auth.GenerateToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Set token in cookie
	auth.SetTokenCookie(c, token)

	c.JSON(http.StatusOK, gin.H{"message": "Login successful", "user": user.Username})
}

// LogoutHandler handles user logout
func LogoutHandler(c *gin.Context) {
	// Clear the token cookie
	auth.ClearTokenCookie(c)

	c.JSON(http.StatusOK, gin.H{"message": "Logout successful"})
}

// CheckAuthHandler checks if user is authenticated
func CheckAuthHandler(c *gin.Context) {
	// Get token from cookie
	token, err := auth.GetTokenFromCookie(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		return
	}

	// Validate token
	claims, err := auth.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"authenticated": false})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"username":      claims.Username,
	})
}
