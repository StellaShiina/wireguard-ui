package handlers

import (
    "net/http"
    "os"
    "github.com/gin-gonic/gin"
)

func LoginPage(c *gin.Context) {
    c.HTML(http.StatusOK, "login.html", gin.H{})
}

func IndexPage(c *gin.Context) {
    if _, err := os.Stat("frontend/dist/index.html"); err == nil {
        c.File("frontend/dist/index.html")
        return
    }
    c.HTML(http.StatusOK, "index.html", gin.H{})
}
