package config

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var auditMu sync.Mutex

// AuditPath is ~/.config/dockafe/audit.log.
func AuditPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "audit.log"), nil
}

// AuditUser returns the local account name for audit lines.
func AuditUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	if u := strings.TrimSpace(os.Getenv("USER")); u != "" {
		return u
	}
	return "unknown"
}

// Audit writes one append-only line to audit.log.
// Format: RFC3339 user=… host=… action=… target=… ok=true|false [err=…]
func Audit(host, action, target string, ok bool, errMsg string) error {
	action = strings.TrimSpace(action)
	if action == "" {
		return fmt.Errorf("empty audit action")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		host = "-"
	}
	target = strings.TrimSpace(target)
	if target == "" {
		target = "-"
	}
	// Keep single-line fields.
	host = strings.ReplaceAll(host, " ", "_")
	target = strings.ReplaceAll(target, " ", "_")
	action = strings.ReplaceAll(action, " ", "_")

	path, err := AuditPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	okStr := "true"
	if !ok {
		okStr = "false"
	}
	line := fmt.Sprintf("%s user=%s host=%s action=%s target=%s ok=%s",
		time.Now().UTC().Format(time.RFC3339),
		AuditUser(),
		host,
		action,
		target,
		okStr,
	)
	if !ok && strings.TrimSpace(errMsg) != "" {
		msg := strings.ReplaceAll(strings.TrimSpace(errMsg), "\n", " ")
		msg = strings.ReplaceAll(msg, " ", "_")
		line += " err=" + msg
	}
	line += "\n"

	auditMu.Lock()
	defer auditMu.Unlock()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// AuditTail returns the last maxLines of audit.log (best-effort).
func AuditTail(maxLines int) (string, error) {
	if maxLines < 1 {
		maxLines = 40
	}
	path, err := AuditPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "(audit log empty — no mutating actions yet)", nil
		}
		return "", err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "(audit log empty — no mutating actions yet)", nil
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	header := fmt.Sprintf("Audit log · %s · last %d lines\n\n", path, len(lines))
	return header + strings.Join(lines, "\n") + "\n", nil
}
