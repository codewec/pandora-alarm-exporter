package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnv(t *testing.T) {
	const key = "PANDORA_TEST_DOTENV_VALUE"
	t.Setenv(key, "existing")
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(key+"=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(key) != "existing" {
		t.Fatal("overwrote environment")
	}
	os.Unsetenv(key)
	if err := LoadEnv(path); err != nil {
		t.Fatal(err)
	}
	if os.Getenv(key) != "from-file" {
		t.Fatal("did not load value")
	}
	if err := LoadEnv(filepath.Join(t.TempDir(), "missing")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("invalid secret-password"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnv(path); err == nil || strings.Contains(err.Error(), "secret-password") {
		t.Fatal("expected sanitized error")
	}
}

func TestParseValue(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"", ""},
		{"password$literal#hash", "password$literal#hash"},
		{"password # comment", "password"},
		{"'password $ # spaces' # comment", "password $ # spaces"},
		{`"password\n\"quoted" # comment`, "password\n\"quoted"},
	} {
		got, err := parseValue(tc.input)
		if err != nil || got != tc.want {
			t.Errorf("parse value: got %q, %v; want %q", got, err, tc.want)
		}
	}
	for _, input := range []string{"'unterminated", "\"unterminated", "'value' trailing"} {
		if _, err := parseValue(input); err == nil {
			t.Error("accepted invalid value")
		}
	}
}
