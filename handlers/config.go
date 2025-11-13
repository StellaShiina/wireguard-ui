package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/StellaShiina/wireguard-ui/config"
	"github.com/StellaShiina/wireguard-ui/db"
	"github.com/StellaShiina/wireguard-ui/wireguard"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
)

// GET /api/v1/configs -> server + peers
func GetConfigs(c *gin.Context) {
	var s db.Server
	if err := db.DB.Limit(1).Find(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("query server failed: %v", err)})
		return
	}
	if s.UUID == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not initialized"})
		return
	}

	var peers []db.Peer
	if err := db.DB.Find(&peers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("query peers failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"server": s, "peers": peers})
}

// POST /api/v1/configs/server/:uuid -> Update server
type UpdateServerRequest struct {
	PublicIP   *string `json:"public_ip"`
	Port       *int    `json:"port"`
	EnableIPv6 *bool   `json:"enable_ipv6"`
	SubnetV4   *string `json:"subnet_v4"`
	SubnetV6   *string `json:"subnet_v6"`
}

func UpdateServer(c *gin.Context) {
	uuid := c.Param("uuid")
	var s db.Server
	if err := db.DB.Where("uuid = ?", uuid).First(&s).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "server not found"})
		return
	}
	var req UpdateServerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	updates := map[string]any{}
	if req.PublicIP != nil {
		updates["public_ip"] = *req.PublicIP
	}
	if req.Port != nil {
		updates["port"] = *req.Port
	}
	if req.EnableIPv6 != nil {
		updates["enable_ipv6"] = *req.EnableIPv6
	}
	if req.SubnetV4 != nil {
		updates["subnet_v4"] = *req.SubnetV4
	}
	if req.SubnetV6 != nil {
		updates["subnet_v6"] = *req.SubnetV6
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}
	if err := db.DB.Model(&db.Server{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("update server failed: %v", err)})
		return
	}
	// If the subnet changes, clear all peers (and delete their client configuration files)
	subnetChanged := (req.SubnetV4 != nil && *req.SubnetV4 != s.SubnetV4) || (req.SubnetV6 != nil && *req.SubnetV6 != s.SubnetV6)
	if subnetChanged {
		// Delete all peers in the database
		if err := db.DB.Delete(&db.Peer{}, "1=1").Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("clear peers failed: %v", err)})
			return
		}
		// Delete client configuration files
		cfg := config.LoadConfig()
		entries, _ := os.ReadDir(cfg.WGClientsDir)
		for _, e := range entries {
			// best-effort
			_ = os.Remove(filepath.Join(cfg.WGClientsDir, e.Name()))
		}
	}
	// After updating, automatically generate new Server public/private keys and write to the database
	priv, pub, err := wireguard.GenerateKeyPair()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate server key failed: %v", err)})
		return
	}
	if err := db.DB.Model(&db.Server{}).Where("uuid = ?", uuid).Updates(map[string]any{"private_key": priv, "public_key": pub}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("save server key failed: %v", err)})
		return
	}

	// Regenerate server configuration and all client configurations
	cfg := config.LoadConfig()
	var s2 db.Server
	_ = db.DB.Where("uuid = ?", uuid).First(&s2).Error
	var peers []db.Peer
	_ = db.DB.Find(&peers).Error
	if err := wireguard.GenerateServerConfig(cfg, s2, peers); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate server config failed: %v", err)})
		return
	}
	for _, p := range peers {
		_, _ = wireguard.GeneratePeerConfig(cfg, s2, p)
	}

	c.JSON(http.StatusOK, gin.H{"message": "server updated; regenerated keys and peer configs"})
}

// POST /api/v1/configs/peer -> Add peer (automatically assign IPv4/IPv6)
type CreatePeerRequest struct {
	Name *string `json:"name"`
}

func CreatePeer(c *gin.Context) {
	var req CreatePeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	// Backend generates key pair, not returned in response
	priv, pub, err := wireguard.GenerateKeyPair()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate peer key failed: %v", err)})
		return
	}
	p := db.Peer{PrivateKey: priv, PublicKey: pub, Name: req.Name}
	if err = db.DB.Clauses(clause.Returning{Columns: []clause.Column{{Name: "uuid"}, {Name: "ipv4"}, {Name: "ipv6"}, {Name: "name"}}}).Omit("uuid").Create(&p).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("create peer failed: %v", err)})
		return
	}

	// Generate client configuration file and update server configuration
	cfg := config.LoadConfig()
	var s db.Server
	_ = db.DB.Limit(1).Find(&s).Error
	// If the server key is empty or non-standard (wg key length is about 44), generate and save it
	if len(s.PrivateKey) < 40 || len(s.PublicKey) < 40 {
		sPriv, sPub, err2 := wireguard.GenerateKeyPair()
		if err2 == nil {
			_ = db.DB.Model(&db.Server{}).Where("uuid = ?", s.UUID).Updates(map[string]any{"private_key": sPriv, "public_key": sPub}).Error
			s.PrivateKey = sPriv
			s.PublicKey = sPub
		}
	}
	path, err := wireguard.GeneratePeerConfig(cfg, s, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate peer config failed: %v", err)})
		return
	}
	var peers []db.Peer
	_ = db.DB.Find(&peers).Error
	_ = wireguard.GenerateServerConfig(cfg, s, peers)

	c.JSON(http.StatusOK, gin.H{"peer": p, "path": path})
}

// PUT /api/v1/configs/peer/:uuid -> Update peer (name only)
type UpdatePeerRequest struct {
	Name *string `json:"name"`
}

func UpdatePeer(c *gin.Context) {
	uuid := c.Param("uuid")
	var req UpdatePeerRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if err := db.DB.Model(&db.Peer{}).Where("uuid = ?", uuid).Update("name", *req.Name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("update peer failed: %v", err)})
		return
	}
	var s db.Server
	_ = db.DB.Limit(1).Find(&s).Error
	var p db.Peer
	_ = db.DB.Where("uuid = ?", uuid).First(&p).Error
	cfg := config.LoadConfig()
	_, _ = wireguard.GeneratePeerConfig(cfg, s, p)
	c.JSON(http.StatusOK, gin.H{"message": "peer updated"})
}

// DELETE /api/v1/configs/peer/:uuid -> Delete peer
func DeletePeer(c *gin.Context) {
	uuid := c.Param("uuid")
	// Delete database row
	if err := db.DB.Delete(&db.Peer{}, "uuid = ?", uuid).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("delete peer failed: %v", err)})
		return
	}
	// Attempt to delete client configuration file
	cfg := config.LoadConfig()
	_ = os.Remove(filepath.Join(cfg.WGClientsDir, fmt.Sprintf("%s.conf", uuid)))
	// Update server configuration
	var s db.Server
	_ = db.DB.Limit(1).Find(&s).Error
	var peers []db.Peer
	_ = db.DB.Find(&peers).Error
	_ = wireguard.GenerateServerConfig(cfg, s, peers)
	c.JSON(http.StatusOK, gin.H{"message": "peer deleted"})
}

// GET /api/v1/configs/peer/:uuid -> Download peer configuration file
func DownloadPeerConfig(c *gin.Context) {
	uuid := c.Param("uuid")
	var s db.Server
	if err := db.DB.Limit(1).Find(&s).Error; err != nil || s.UUID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server not initialized"})
		return
	}
	var p db.Peer
	if err := db.DB.Where("uuid = ?", uuid).First(&p).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "peer not found"})
		return
	}
	cfg := config.LoadConfig()
	path, err := wireguard.GeneratePeerConfig(cfg, s, p)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("generate peer config failed: %v", err)})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.conf\"", uuid))
	c.File(path)
}
