// Package config loads local exporter configuration.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadEnv loads a single-line KEY=value file without overwriting existing
// environment variables. A missing file is allowed for container deployments.
// Values are literal: shell commands and variable interpolation are not evaluated.
func LoadEnv(path string) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("cannot open environment file")
	}
	defer f.Close()
	values := map[string]string{}
	scanner := bufio.NewScanner(f)
	line := 0
	for scanner.Scan() {
		line++
		s := strings.TrimSpace(scanner.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		s = strings.TrimPrefix(s, "export ")
		key, value, ok := strings.Cut(s, "=")
		key = strings.TrimSpace(key)
		if !ok || !validKey(key) {
			return fmt.Errorf("invalid environment assignment at line %d", line)
		}
		value, err = parseValue(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("invalid environment value at line %d", line)
		}
		values[key] = value
	}
	if scanner.Err() != nil {
		return errors.New("cannot read environment file")
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return errors.New("cannot set environment variable")
			}
		}
	}
	return nil
}

func validKey(s string) bool {
	if s == "" {
		return false
	}
	for i, c := range s {
		if c != '_' && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}

func parseValue(s string) (string, error) {
	if strings.IndexByte(s, 0) >= 0 {
		return "", errors.New("NUL in value")
	}
	if s == "" {
		return "", nil
	}
	if s[0] != '\'' && s[0] != '"' {
		for i, c := range s {
			if c == '#' && i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
				return strings.TrimSpace(s[:i]), nil
			}
		}
		return s, nil
	}
	quote := s[0]
	for i := 1; i < len(s); i++ {
		if quote == '"' && s[i] == '\\' {
			i++
			continue
		}
		if s[i] != quote {
			continue
		}
		tail := strings.TrimSpace(s[i+1:])
		if tail != "" && !strings.HasPrefix(tail, "#") {
			break
		}
		if quote == '\'' {
			return s[1:i], nil
		}
		return strconv.Unquote(s[:i+1])
	}
	return "", errors.New("invalid quoted value")
}
