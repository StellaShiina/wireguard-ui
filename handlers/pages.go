package handlers

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

// LoginPage renders a simple login form.
// Critical: authentication is cookie-based, JS posts to /auth/login and redirects.
func LoginPage(c *gin.Context) {
    c.HTML(http.StatusOK, "login.html", gin.H{})
}

// IndexPage renders the admin panel shell; data is fetched client-side.
// Critical: API calls require authentication; fetch includes credentials by default for same-origin.
func IndexPage(c *gin.Context) {
    c.HTML(http.StatusOK, "index.html", gin.H{})
}