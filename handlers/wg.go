package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/StellaShiina/wireguard-ui/config"
	"github.com/gin-gonic/gin"
)

func runSystemctl(args ...string) (string, error) {
	cmd := exec.Command("systemctl", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Sprintf("%s%s", out.String(), stderr.String()), err
	}
	return out.String(), nil
}

// Start: enable and start wg-quick@wg0
func WGStart(c *gin.Context) {
	cfg := config.LoadConfig()
	confPath := filepath.Join(cfg.WGConfDir, fmt.Sprintf("%s.conf", cfg.WGInterface))
	svc := fmt.Sprintf("%s-quick@%s", cfg.WGMode, cfg.WGInterface)
	// Ensure config exists
	if _, err := os.Stat(confPath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("%s not found in %s", filepath.Base(confPath), cfg.WGConfDir)})
		return
	}
	out, err := runSystemctl("--now", "enable", svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("systemctl enable failed: %v", err), "output": out, "service": svc})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "wireguard started", "output": out, "service": svc})
}

// Stop: disable and stop wg-quick@wg0
func WGStop(c *gin.Context) {
	cfg := config.LoadConfig()
	svc := fmt.Sprintf("%s-quick@%s", cfg.WGMode, cfg.WGInterface)
	out, err := runSystemctl("--now", "disable", svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("systemctl disable failed: %v", err), "output": out, "service": svc})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "wireguard stopped", "output": out, "service": svc})
}

// Restart: restart wg-quick@wg0
func WGRestart(c *gin.Context) {
	cfg := config.LoadConfig()
	svc := fmt.Sprintf("%s-quick@%s", cfg.WGMode, cfg.WGInterface)
	out, err := runSystemctl("restart", svc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("systemctl restart failed: %v", err), "output": out, "service": svc})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "wireguard restarted", "output": out, "service": svc})
}

// Status: status wg-quick@wg0
func WGStatus(c *gin.Context) {
	cfg := config.LoadConfig()
	svc := fmt.Sprintf("%s-quick@%s", cfg.WGMode, cfg.WGInterface)
	out, err := runSystemctl("status", svc)
	if err != nil {
		// status returns non-zero when inactive; still return output
		c.JSON(http.StatusOK, gin.H{"status": "error", "output": out, "error": err.Error(), "service": svc})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "output": out, "service": svc})
}

// Show: run `wg show` and return its output for detailed status
func WGShow(c *gin.Context) {
	cfg := config.LoadConfig()
	cmd := exec.Command(cfg.WGMode, "show")
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "output": out.String(), "stderr": stderr.String()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"output": out.String()})
}
