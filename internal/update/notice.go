package update

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const checkInterval = 24 * time.Hour

// checkState is persisted at ~/.config/itapu/update-check.json so the
// passive check hits the network at most once per checkInterval. The last
// resolved tag is cached, so the hint keeps showing between checks until
// the user actually updates.
type checkState struct {
	CheckedAt time.Time `json:"checkedAt"`
	LatestTag string    `json:"latestTag,omitempty"`
}

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "itapu", "update-check.json"), nil
}

// Notice returns a one-line hint when a release newer than current exists,
// or "" otherwise. It is best-effort by design: throttled, a 2-second
// network timeout, and any error just means no hint — a slow or offline
// network must never delay or break a command.
func Notice(current string) string {
	if _, ok := parseVersion(current); !ok {
		return "" // dev build — nothing meaningful to compare against
	}
	path, err := statePath()
	if err != nil {
		return ""
	}
	var st checkState
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &st)
	}

	if time.Since(st.CheckedAt) >= checkInterval {
		st.CheckedAt = time.Now()
		if tag, err := LatestTag(&http.Client{Timeout: 2 * time.Second}); err == nil {
			st.LatestTag = tag
		}
		if data, err := json.Marshal(st); err == nil {
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
				_ = os.WriteFile(path, append(data, '\n'), 0o644)
			}
		}
	}

	if newer, ok := Newer(st.LatestTag, current); ok && newer {
		return fmt.Sprintf("A new version of itapu is available (%s → %s) — run `itapu update`.", DisplayVersion(current), st.LatestTag)
	}
	return ""
}
