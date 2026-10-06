package ufw

import (
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/we1bboard/we1bboard/internal/database"
	"github.com/we1bboard/we1bboard/internal/database/model"
)

// Enabled reports whether UFW integration is on (Linux only).
func Enabled() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	v := strings.ToLower(strings.TrimSpace(database.GetSetting("ufwEnable")))
	return v == "" || v == "true" || v == "1" || v == "yes" || v == "on"
}

func available() bool {
	_, err := exec.LookPath("ufw")
	return err == nil
}

func run(args ...string) error {
	cmd := exec.Command("ufw", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ufw %v: %w (%s)", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// EnsureActive enables UFW if inactive (non-interactive), preserving SSH.
func EnsureActive() error {
	if !Enabled() || !available() {
		return nil
	}
	status, _ := exec.Command("ufw", "status").CombinedOutput()
	s := strings.ToLower(string(status))
	if strings.Contains(s, "status: active") {
		return nil
	}
	_ = AllowTCP(detectSSHPort(), "we1bboard-ssh")
	_ = run("--force", "enable")
	return nil
}

func detectSSHPort() int {
	// Prefer listening sshd port
	out, err := exec.Command("ss", "-lntp").CombinedOutput()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if !strings.Contains(line, "sshd") {
				continue
			}
			// e.g. LISTEN 0 128 *:22 *:* users:(("sshd",pid=1,fd=3))
			for _, p := range []string{":22 ", "]:22 "} {
				if strings.Contains(line, p) {
					return 22
				}
			}
			// generic :PORT
			if i := strings.LastIndex(line, ":"); i >= 0 {
				rest := line[i+1:]
				num := ""
				for _, c := range rest {
					if c >= '0' && c <= '9' {
						num += string(c)
					} else {
						break
					}
				}
				if n, e := strconv.Atoi(num); e == nil && n > 0 && n < 65536 {
					return n
				}
			}
		}
	}
	return 22
}

// AllowTCP opens a TCP port if not already allowed.
func AllowTCP(port int, comment string) error {
	if !Enabled() || !available() || port < 1 || port > 65535 {
		return nil
	}
	if err := EnsureActive(); err != nil {
		return err
	}
	cmt := strings.TrimSpace(comment)
	if cmt == "" {
		cmt = "we1bboard"
	}
	// idempotent: ufw allow is fine to repeat
	return run("allow", strconv.Itoa(port) + "/tcp", "comment", cmt)
}

// DeleteTCP removes a rule for the port (best-effort).
func DeleteTCP(port int) {
	if !Enabled() || !available() || port < 1 || port > 65535 {
		return
	}
	_ = run("delete", "allow", strconv.Itoa(port)+"/tcp")
}

// SyncPanel opens panel + subscription + optional ACME 80/443.
func SyncPanel() error {
	if !Enabled() || !available() {
		return nil
	}
	if err := EnsureActive(); err != nil {
		return err
	}
	panelPort, _ := strconv.Atoi(database.GetSetting("panelPort"))
	if panelPort == 0 {
		panelPort = 2053
	}
	_ = AllowTCP(panelPort, "we1bboard-panel")

	subPort, _ := strconv.Atoi(database.GetSetting("subPort"))
	if subPort == 0 {
		subPort = 2096
	}
	if subPort != panelPort {
		_ = AllowTCP(subPort, "we1bboard-sub")
	}
	// HTTP-01 / HTTPS commonly needed
	_ = AllowTCP(80, "we1bboard-http")
	_ = AllowTCP(443, "we1bboard-https")
	return nil
}

// SyncInbound opens (or closes) the inbound listen port.
func SyncInbound(in *model.Inbound, remove bool) {
	if in == nil || in.Port < 1 {
		return
	}
	if remove || !in.Enable {
		DeleteTCP(in.Port)
		return
	}
	_ = AllowTCP(in.Port, fmt.Sprintf("we1bboard-in-%d", in.ID))
}

// SyncAllInbounds refreshes rules for every enabled inbound.
func SyncAllInbounds() {
	if !Enabled() || !available() {
		return
	}
	_ = SyncPanel()
	var rows []model.Inbound
	if err := database.DB.Find(&rows).Error; err != nil {
		return
	}
	for i := range rows {
		SyncInbound(&rows[i], false)
	}
}
