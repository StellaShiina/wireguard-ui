package netutil

import (
    "bufio"
    "errors"
    "os"
    "os/exec"
    "strings"
)

// DetectDefaultInterface tries to determine the host's default outbound interface.
// It first uses `ip route show default`, and falls back to /proc/net/route.
func DetectDefaultInterface() (string, error) {
    // Try `ip route`
    cmd := exec.Command("ip", "route", "show", "default")
    out, err := cmd.Output()
    if err == nil {
        line := strings.TrimSpace(string(out))
        // Example: "default via 192.168.1.1 dev eth0 proto dhcp src 192.168.1.23 metric 100"
        parts := strings.Fields(line)
        for i := 0; i < len(parts)-1; i++ {
            if parts[i] == "dev" {
                return parts[i+1], nil
            }
        }
    }

    // Fallback: /proc/net/route
    f, err2 := os.Open("/proc/net/route")
    if err2 != nil {
        if err != nil {
            return "", err
        }
        return "", err2
    }
    defer f.Close()
    scanner := bufio.NewScanner(f)
    // Skip header
    if !scanner.Scan() {
        return "", errors.New("empty /proc/net/route")
    }
    for scanner.Scan() {
        // Fields: Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT
        fields := strings.Fields(scanner.Text())
        if len(fields) >= 2 {
            // Destination 00000000 means default route
            if fields[1] == "00000000" {
                return fields[0], nil
            }
        }
    }
    return "", errors.New("default interface not found")
}