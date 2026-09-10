package config

import (
	"fmt"
	"time"
)

// PollInterval resolves CLI > environment > default. Durations include units.
func PollInterval(env string, cli time.Duration, cliSet bool) (time.Duration, error) {
	interval := 5 * time.Minute
	if cliSet {
		interval = cli
	} else if env != "" {
		var err error
		interval, err = time.ParseDuration(env)
		if err != nil {
			return 0, fmt.Errorf("PANDORA_POLL_INTERVAL must be a duration such as 5m or 300s")
		}
	}
	if interval < 10*time.Second {
		return 0, fmt.Errorf("poll interval must be at least 10s")
	}
	return interval, nil
}
