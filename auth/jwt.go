package auth

import (
	"errors"
	"time"

	"github.com/StellaShiina/wireguard-ui/config"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// App configuration
var appConfig *config.Config

// Initialize config
func init() {
	appConfig = config.LoadConfig()
}

// User represents the authenticated user
type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Claims represents the JWT claims
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// ValidateCredentials validates user credentials
func ValidateCredentials(user User) bool {
	return user.Username == appConfig.AuthUsername && user.Password == appConfig.AuthPassword
}

// GenerateToken generates a JWT token for the given username
func GenerateToken(username string) (string, error) {
	// Set expiration time to 7 days
	expirationTime := time.Now().Add(7 * 24 * time.Hour)

	// Create the JWT claims
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   username,
		},
	}

	// Create the token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString([]byte(appConfig.JWTSecret))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates the JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return []byte(appConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// GetTokenFromCookie extracts the JWT token from the cookie
func GetTokenFromCookie(c *gin.Context) (string, error) {
	cookie, err := c.Cookie("token")
	if err != nil {
		return "", err
	}
	return cookie, nil
}

// SetTokenCookie sets the JWT token in a cookie
func SetTokenCookie(c *gin.Context, token string) {
	// Set cookie to expire in 7 days
	c.SetCookie("token", token, 7*24*60*60, "/", "", false, true)
}

// ClearTokenCookie clears the JWT token cookie
func ClearTokenCookie(c *gin.Context) {
	// Set cookie with negative MaxAge to delete it
	c.SetCookie("token", "", -1, "/", "", false, true)
}
