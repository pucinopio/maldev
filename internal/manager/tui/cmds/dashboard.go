// Package cmds contains bubbletea Cmd factories that bridge service layer
// calls into async messages consumed by the root TUI model.
package cmds

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/oioio-space/maldev/internal/manager/service"
	"github.com/oioio-space/maldev/internal/manager/store/ent/license"
)

// DashboardSnapshot carries all data the dashboard view needs to render.
type DashboardSnapshot struct {
	Active       int
	Revoked      int
	Expired      int
	ExpiringSoon int
	Superseded   int

	ActiveKeyID          string
	ActiveKeyName        string
	ActiveKeyFingerprint string

	Servers []ServerStatus

	RecentAudit []AuditEntry

	// HeatmapIssuance/HeatmapExpiry hold per-day counts for the last 91 days
	// (13 weeks × 7 days). Index 0 = today, index 90 = 90 days ago. Days with
	// no licence are left as 0 so the dashboard heatmap can render mute cells.
	HeatmapIssuance [91]int
	HeatmapExpiry   [91]int

	Err error
}

// ServerStatus is a single server's runtime status for the dashboard.
type ServerStatus struct {
	Name     string
	On       bool
	URL      string
	Requests uint64
	Uptime   string // human-readable uptime, e.g. "2h 41m" — empty when stopped
}

// AuditEntry is a trimmed audit event for the recent-events list.
type AuditEntry struct {
	At       time.Time
	Kind     string
	TargetID string
	Actor    string
	Note     string
}

// DashboardSnapshotMsg wraps DashboardSnapshot as a tea.Msg.
type DashboardSnapshotMsg DashboardSnapshot

// DashboardSnapshotCmd gathers counters, active key, server status, and
// recent audit entries from the service layer in a single async command.
func DashboardSnapshotCmd(svc *service.Services) tea.Cmd {
	return func() tea.Msg {
		if svc == nil {
			return DashboardSnapshotMsg{Err: nil} // nil services → empty snapshot
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		snap := DashboardSnapshot{}

		// Counters — list all licences and bucket by status.
		now := time.Now()
		soonWindow := now.Add(30 * 24 * time.Hour)

		active, err := svc.License.List(ctx, service.ListFilter{Status: "active", Limit: 10000})
		if err != nil {
			snap.Err = err
			return DashboardSnapshotMsg(snap)
		}
		bucketDay := func(t time.Time) int {
			d := int(now.Sub(t).Hours() / 24)
			if d < 0 || d >= 91 {
				return -1
			}
			return d
		}
		for _, lic := range active {
			if lic.NotAfter.Before(now) {
				snap.Expired++
			} else if lic.NotAfter.Before(soonWindow) {
				snap.ExpiringSoon++
			} else {
				snap.Active++
			}
			if d := bucketDay(lic.CreatedAt); d >= 0 {
				snap.HeatmapIssuance[d]++
			}
			if d := bucketDay(lic.NotAfter); d >= 0 {
				snap.HeatmapExpiry[d]++
			}
		}

		revoked, err := svc.License.List(ctx, service.ListFilter{Status: "revoked", Limit: 10000})
		if err != nil {
			snap.Err = err
			return DashboardSnapshotMsg(snap)
		}
		snap.Revoked = len(revoked)

		expired, err := svc.License.List(ctx, service.ListFilter{Status: string(license.StatusExpired), Limit: 10000})
		if err == nil {
			snap.Expired += len(expired)
		}

		// Active issuer key.
		issuers, err := svc.Issuer.List(ctx)
		if err == nil {
			for _, iss := range issuers {
				if iss.Active {
					snap.ActiveKeyID = iss.KeyID
					snap.ActiveKeyName = iss.Name
					snap.ActiveKeyFingerprint = iss.KeyID // placeholder until fingerprint helper exists
					break
				}
			}
		}

		// Server status — bundle is wired in Phase 4; show placeholder rows.
		snap.Servers = []ServerStatus{
			{Name: "Revocation", On: false, URL: "—"},
			{Name: "Heartbeat", On: false, URL: "—"},
			{Name: "Probe", On: false, URL: "—"},
		}

		// Recent audit events.
		events, err := svc.Audit.List(ctx, 5)
		if err == nil {
			for _, e := range events {
				snap.RecentAudit = append(snap.RecentAudit, AuditEntry{
					At:       e.CreatedAt,
					Kind:     e.Kind,
					TargetID: e.TargetID,
					Actor:    e.Actor,
				})
			}
		}

		return DashboardSnapshotMsg(snap)
	}
}
