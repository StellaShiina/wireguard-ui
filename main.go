package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/StellaShiina/wireguard-ui/config"
	"github.com/StellaShiina/wireguard-ui/db"
	"github.com/StellaShiina/wireguard-ui/handlers"
	"github.com/StellaShiina/wireguard-ui/middleware"
	"github.com/StellaShiina/wireguard-ui/netutil"
)

type Server struct {
	UUID       string `gorm:"type:uuid;primaryKey"`
	PublicIP   string `gorm:"type:inet;not null"`
	Port       int    `gorm:"not null"`
	EnableIPv6 bool   `gorm:"not null"`
	SubnetV4   string `gorm:"type:cidr;not null"`
	SubnetV6   string `gorm:"type:cidr;not null"`
	PrivateKey string `gorm:"not null"`
	PublicKey  string `gorm:"not null"`
}

type Peer struct {
	UUID       string  `gorm:"type:uuid;primaryKey"`
	IPv4       *string `gorm:"type:cidr;unique"`
	IPv6       *string `gorm:"type:cidr;unique"`
	PrivateKey string  `gorm:"not null"`
	PublicKey  string  `gorm:"not null"`
	Name       *string
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	// Prioritize loading environment variables from /etc/wireguard-ui/.env, then fall back to the project root directory
	if err := godotenv.Load("/etc/wireguard-ui/.env"); err != nil {
		if err2 := godotenv.Load(); err2 != nil {
			log.Println("No .env found in /etc/wireguard-ui or project root; using defaults.")
		} else {
			log.Println("Loaded .env from project root.")
		}
	} else {
		log.Println("Loaded .env from /etc/wireguard-ui/.env")
	}
	// Initialize database connection (normal operation mode)
	cfg := config.LoadConfig()
	// Automatically detect the default external network interface name when the program starts
	if cfg.WGExternalIF == "" {
		if ifname, err := netutil.DetectDefaultInterface(); err == nil && ifname != "" {
			cfg.WGExternalIF = ifname
			fmt.Printf("[WG] Detected external interface: %s\n", ifname)
		} else {
			fmt.Printf("[WG] Could not detect external interface automatically: %v\n", err)
		}
	}
	if err := db.Init(cfg); err != nil {
		fmt.Printf("failed to init database: %v\n", err)
		os.Exit(1)
	}

	r := gin.Default()

	// Load HTML templates for /login and /
	r.LoadHTMLGlob("templates/*")

	r.SetTrustedProxies([]string{"127.0.0.1", "::1"})

	auth := r.Group("/auth")
	{
		auth.POST("/login", handlers.LoginHandler)
		auth.GET("/logout", handlers.LogoutHandler)
		auth.GET("/check", handlers.CheckAuthHandler)
	}

	// Pages
	r.GET("/login", middleware.RedirectIfAuthenticated(), handlers.LoginPage)
	r.GET("/", middleware.AuthPageRequired(), handlers.IndexPage)

	api := r.Group("/api/v1", middleware.AuthRequired())
	{
		configs := api.Group("/configs")
		{
			configs.GET("", handlers.GetConfigs)
			configs.POST("/server/:uuid", handlers.UpdateServer)
			configs.POST("/peer", handlers.CreatePeer)
			configs.PUT("/peer/:uuid", handlers.UpdatePeer)
			configs.DELETE("/peer/:uuid", handlers.DeletePeer)
			configs.GET("/peer/:uuid", handlers.DownloadPeerConfig)
		}
		wg := api.Group("/wg")
		{
			wg.POST("/start", handlers.WGStart)
			wg.POST("/stop", handlers.WGStop)
			wg.POST("/restart", handlers.WGRestart)
			wg.GET("/status", handlers.WGStatus)
			wg.GET("/show", handlers.WGShow)
		}
	}

	// Allow specifying the listening address via UI_ADDR (e.g., 0.0.0.0:9999); otherwise, fall back to UI_PORT managed by config or default localhost:9999
	addr := cfg.UIAddr + ":" + cfg.UIPort
	r.Run(addr)
}

func (Server) TableName() string { return "server" }
func (Peer) TableName() string   { return "peer" }
