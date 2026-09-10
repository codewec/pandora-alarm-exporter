package config

import (
	"testing"
	"time"
)

func TestPollInterval(t *testing.T) {
	for _, tc := range []struct {
		name, env string
		cli       time.Duration
		set       bool
		want      time.Duration
		invalid   bool
	}{
		{name: "default", want: 5 * time.Minute},
		{name: "env minutes", env: "2m", want: 2 * time.Minute},
		{name: "env seconds", env: "300s", want: 5 * time.Minute},
		{name: "CLI priority", env: "bad", cli: time.Minute, set: true, want: time.Minute},
		{name: "minimum", env: "10s", want: 10 * time.Second},
		{name: "too short", env: "9s", invalid: true},
		{name: "missing units", env: "300", invalid: true},
		{name: "negative", env: "-1m", invalid: true},
		{name: "invalid CLI", env: "5m", cli: 0, set: true, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PollInterval(tc.env, tc.cli, tc.set)
			if (err != nil) != tc.invalid || (!tc.invalid && got != tc.want) {
				t.Fatalf("got %s, %v; want %s, invalid=%t", got, err, tc.want, tc.invalid)
			}
		})
	}
}
