package tui

import (
	"regexp"
	"sort"
	"strings"
	"time"
)

type AlertSeverity string

const (
	AlertInfo     AlertSeverity = "info"
	AlertSuccess  AlertSeverity = "success"
	AlertWarning  AlertSeverity = "warning"
	AlertError    AlertSeverity = "error"
	alertLifetime               = 5 * time.Second
)

type Alert struct {
	ID        uint64
	Key       string
	Severity  AlertSeverity
	Summary   string
	Details   string
	CreatedAt time.Time
	ExpiresAt time.Time
}

type AlertManager struct {
	alerts  []Alert
	nextID  uint64
	now     func() time.Time
	secrets []string
}

func NewAlertManager(secrets ...string) AlertManager {
	return AlertManager{now: time.Now, secrets: append([]string(nil), secrets...)}
}

func (manager *AlertManager) Push(key string, severity AlertSeverity, summary string, details ...string) Alert {
	manager.ensureClock()
	manager.nextID++
	alert := Alert{ID: manager.nextID, Key: key, Severity: severity, Summary: manager.safe(summary), CreatedAt: manager.now()}
	if len(details) > 0 {
		alert.Details = manager.safeDetails(strings.Join(details, "\n"))
	}
	if severity == AlertInfo || severity == AlertSuccess {
		alert.ExpiresAt = alert.CreatedAt.Add(alertLifetime)
	}
	manager.alerts = append(manager.alerts, alert)
	return alert
}

func (manager *AlertManager) Clear(key string) {
	if key == "" {
		return
	}
	kept := manager.alerts[:0]
	for _, alert := range manager.alerts {
		if alert.Key != key {
			kept = append(kept, alert)
		}
	}
	manager.alerts = kept
}

func (manager *AlertManager) Dismiss() {
	if active, present := manager.Active(); present {
		kept := manager.alerts[:0]
		for _, alert := range manager.alerts {
			if alert.ID != active.ID {
				kept = append(kept, alert)
			}
		}
		manager.alerts = kept
	}
}

func (manager *AlertManager) Expire() {
	manager.ensureClock()
	now := manager.now()
	kept := manager.alerts[:0]
	for _, alert := range manager.alerts {
		if alert.ExpiresAt.IsZero() || now.Before(alert.ExpiresAt) {
			kept = append(kept, alert)
		}
	}
	manager.alerts = kept
}

func (manager *AlertManager) Active() (Alert, bool) {
	manager.Expire()
	if len(manager.alerts) == 0 {
		return Alert{}, false
	}
	candidates := append([]Alert(nil), manager.alerts...)
	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := alertPriority(candidates[i].Severity), alertPriority(candidates[j].Severity)
		if left != right {
			return left > right
		}
		return candidates[i].ID > candidates[j].ID
	})
	return candidates[0], true
}

func (manager *AlertManager) safe(value string) string {
	return strings.TrimSpace(manager.safeDetails(value))
}

func (manager *AlertManager) safeDetails(value string) string {
	value = Sanitize(value)
	for _, secret := range manager.secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "[REDACTED]")
		}
	}
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	return value
}

func (manager *AlertManager) ensureClock() {
	if manager.now == nil {
		manager.now = time.Now
	}
}

var bearerPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`)

func alertPriority(severity AlertSeverity) int {
	switch severity {
	case AlertError:
		return 4
	case AlertWarning:
		return 3
	case AlertSuccess:
		return 2
	default:
		return 1
	}
}

func alertPrefix(severity AlertSeverity) string {
	switch severity {
	case AlertSuccess:
		return "SUCCESS"
	case AlertWarning:
		return "WARNING"
	case AlertError:
		return "ERROR"
	default:
		return "INFO"
	}
}
