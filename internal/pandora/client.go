// Package pandora implements the read-only Pandora API client.
package pandora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Object map[string]any
type Device struct {
	ID       any    `json:"id"`
	Name     string `json:"name"`
	Model    string `json:"model"`
	Firmware string `json:"firmware"`
	FuelTank any    `json:"fuel_tank"`
}

func (d Device) Key() string { return fmt.Sprint(d.ID) }

type Update struct {
	TS    json.Number       `json:"ts"`
	Stats map[string]Object `json:"stats"`
	Time  map[string]Object `json:"time"`
}
type Client struct {
	base, username, password string
	http                     *http.Client
	logged                   bool
}

func NewClient(base, username, password string, timeout time.Duration) (*Client, error) {
	u, err := url.Parse(base)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid API base URL")
	}
	jar, _ := cookiejar.New(nil)
	return &Client{base: strings.TrimRight(base, "/"), username: username, password: password, http: &http.Client{
		Timeout: timeout, Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

var errSession = errors.New("API session expired")

func (c *Client) request(ctx context.Context, path string, form url.Values, out any) (result error) {
	defer func() {
		if result != nil {
			endpoint, _, _ := strings.Cut(path, "?")
			result = fmt.Errorf("%s: %w", endpoint, result)
		}
	}()
	method := http.MethodGet
	var body io.Reader
	if form != nil {
		method = http.MethodPost
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return errors.New("create API request")
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	// Pandora's browser API requires Fetch Metadata in addition to User-Agent.
	origin := req.URL.Scheme + "://" + req.URL.Host
	req.Header.Set("Origin", origin)
	req.Header.Set("Referer", origin+"/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("API transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return errSession
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("API HTTP status %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 8*1024*1024+1))
	if err != nil {
		return errors.New("read API response")
	}
	if len(b) > 8*1024*1024 {
		return errors.New("API response too large")
	}
	var status struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(b, &status)
	switch status.Status {
	case "Session is expired", "Invalid session", "sid-expired":
		return errSession
	case "fail":
		return errors.New("API returned failure")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err = dec.Decode(out); err != nil {
		return errors.New("invalid API JSON")
	}
	if dec.Decode(new(any)) != io.EOF {
		return errors.New("trailing API data")
	}
	return nil
}
func (c *Client) login(ctx context.Context) error {
	c.logged = false
	var response struct {
		Session string `json:"session_id"`
	}
	if err := c.request(ctx, "/api/users/login", url.Values{"login": {c.username}, "password": {c.password}, "lang": {"ru"}}, &response); err != nil {
		return err
	}
	if response.Session == "" {
		return errors.New("login response lacks session_id")
	}
	c.logged = true
	return nil
}
func (c *Client) get(ctx context.Context, path string, out any) error {
	if !c.logged {
		if err := c.login(ctx); err != nil {
			return err
		}
	}
	err := c.request(ctx, path, nil, out)
	if errors.Is(err, errSession) {
		c.logged = false
		if err = c.login(ctx); err != nil {
			return err
		}
		err = c.request(ctx, path, nil, out)
		if errors.Is(err, errSession) {
			c.logged = false
		}
	}
	return err
}
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	var d []Device
	err := c.get(ctx, "/api/devices", &d)
	if err == nil && d == nil {
		err = errors.New("invalid device list")
	}
	seen := map[string]bool{}
	for _, v := range d {
		if v.ID == nil || v.Key() == "" || seen[v.Key()] {
			return nil, errors.New("invalid device ID")
		}
		seen[v.Key()] = true
	}
	return d, err
}
func (c *Client) Updates(ctx context.Context, ts int64) (Update, error) {
	var u Update
	err := c.get(ctx, "/api/updates?ts="+strconv.FormatInt(ts, 10), &u)
	if err == nil {
		n, e := u.TS.Int64()
		if e != nil || n <= 0 {
			err = errors.New("invalid update timestamp")
		}
	}
	return u, err
}
