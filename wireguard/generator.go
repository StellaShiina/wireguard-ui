package wireguard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/StellaShiina/wireguard-ui/config"
	"github.com/StellaShiina/wireguard-ui/db"
	"github.com/StellaShiina/wireguard-ui/netutil"
)

func ensureDirs(cfg *config.Config) error {
	if err := os.MkdirAll(cfg.WGConfDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.WGClientsDir, 0o755); err != nil {
		return err
	}
	return nil
}

func GenerateServerConfig(cfg *config.Config, s db.Server, peers []db.Peer) error {
	if err := ensureDirs(cfg); err != nil {
		return err
	}
	// Detect external interface if not set
	extIF := cfg.WGExternalIF
	if extIF == "" {
		if ifname, err := netutil.DetectDefaultInterface(); err == nil {
			extIF = ifname
		}
	}

	content := "[Interface]\n"
	if s.EnableIPv6 {
		content += fmt.Sprintf("Address = %s\n", s.SubnetV4)
		content += fmt.Sprintf("Address = %s\n", s.SubnetV6)
	} else {
		content += fmt.Sprintf("Address = %s\n", s.SubnetV4)
	}
	content += fmt.Sprintf("ListenPort = %d\n", s.Port)
	content += fmt.Sprintf("PrivateKey = %s\n", s.PrivateKey)
	// PostUp/PostDown rules for NAT and forwarding using detected interface
	if extIF != "" {
		content += fmt.Sprintf("PostUp   = iptables -A FORWARD -i %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostUp   = iptables -A FORWARD -o %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostUp   = iptables -t nat -A POSTROUTING -o %s -j MASQUERADE\n", extIF)
		content += fmt.Sprintf("PostUp   = ip6tables -A FORWARD -i %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostUp   = ip6tables -A FORWARD -o %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostUp   = ip6tables -t nat -A POSTROUTING -o %s -j MASQUERADE\n", extIF)
		content += fmt.Sprintf("PostDown = iptables -D FORWARD -i %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostDown = iptables -D FORWARD -o %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostDown = iptables -t nat -D POSTROUTING -o %s -j MASQUERADE\n", extIF)
		content += fmt.Sprintf("PostDown = ip6tables -D FORWARD -i %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostDown = ip6tables -D FORWARD -o %%i -j ACCEPT\n")
		content += fmt.Sprintf("PostDown = ip6tables -t nat -D POSTROUTING -o %s -j MASQUERADE\n\n", extIF)
	} else {
		content += "\n" // keep spacing even if extIF undetected
	}

	for _, p := range peers {
		content += "[Peer]\n"
		content += fmt.Sprintf("PublicKey = %s\n", p.PublicKey)
		// AllowedIPs include peer IPv4 and IPv6 if present
		if p.IPv6 != nil && *p.IPv6 != "" {
			content += fmt.Sprintf("AllowedIPs = %s\n", valOrEmpty(p.IPv4))
			content += fmt.Sprintf("AllowedIPs = %s\n\n", valOrEmpty(p.IPv6))
		} else {
			content += fmt.Sprintf("AllowedIPs = %s\n\n", valOrEmpty(p.IPv4))
		}
	}

	filename := cfg.WGInterface + ".conf"

	path := filepath.Join(cfg.WGConfDir, filename)
	return os.WriteFile(path, []byte(content), 0o644)
}

func GeneratePeerConfig(cfg *config.Config, s db.Server, p db.Peer) (string, error) {
	if err := ensureDirs(cfg); err != nil {
		return "", err
	}
	content := "[Interface]\n"
	content += fmt.Sprintf("PrivateKey = %s\n", p.PrivateKey)
	// Address: include IPv4 and optionally IPv6
	if s.EnableIPv6 && p.IPv6 != nil && *p.IPv6 != "" {
		content += fmt.Sprintf("Address = %s, %s\n", valOrEmpty(p.IPv4), valOrEmpty(p.IPv6))
	} else {
		content += fmt.Sprintf("Address = %s\n", valOrEmpty(p.IPv4))
	}
	content += "MTU = 1420\n\n"

	content += "[Peer]\n"
	content += fmt.Sprintf("PublicKey = %s\n", s.PublicKey)
	var endpointIP string
	if strings.Contains(s.PublicIP, ":") {
		endpointIP = "[" + s.PublicIP + "]"
	} else {
		endpointIP = s.PublicIP
	}
	content += fmt.Sprintf("Endpoint = %s:%d\n", endpointIP, s.Port)
	// Route only the server subnets through the tunnel; rely on server-side NAT
	if s.EnableIPv6 && s.SubnetV6 != "" {
		content += fmt.Sprintf("AllowedIPs = %s\n", s.SubnetV4)
		content += fmt.Sprintf("AllowedIPs = %s\n", s.SubnetV6)
	} else {
		content += fmt.Sprintf("AllowedIPs = %s\n", s.SubnetV4)
	}

	path := filepath.Join(cfg.WGClientsDir, fmt.Sprintf("%s.conf", p.UUID))
	return path, os.WriteFile(path, []byte(content), 0o644)
}

func valOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
