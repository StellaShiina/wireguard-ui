package wireguard

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"

	"github.com/StellaShiina/wireguard-ui/config"
)

// GenerateKeyPair uses `wg genkey` and `wg pubkey` to produce a private/public key pair.
// It avoids shell usage and pipes the private key to `wg pubkey` via stdin.
// Note: requires WireGuard tools installed and accessible in PATH.
func GenerateKeyPair() (privateKey, publicKey string, err error) {
	// Generate private key
	cfg := config.LoadConfig()
	gen := exec.Command(cfg.WGMode, "genkey")
	privOut, err := gen.Output()
	if err != nil {
		return "", "", err
	}
	priv := strings.TrimSpace(string(privOut))
	if priv == "" {
		return "", "", errors.New("empty private key from wg genkey")
	}

	// Derive public key
	pub := exec.Command(cfg.WGMode, "pubkey")
	pub.Stdin = bytes.NewReader([]byte(priv))
	pubOut, err := pub.Output()
	if err != nil {
		return "", "", err
	}
	pubKey := strings.TrimSpace(string(pubOut))
	if pubKey == "" {
		return "", "", errors.New("empty public key from wg pubkey")
	}
	return priv, pubKey, nil
}
