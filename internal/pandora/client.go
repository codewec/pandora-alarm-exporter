// Package pandora implements the read-only Pandora API client.
package pandora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
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
	accessToken              string
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
	requestURL := c.base + path
	if c.accessToken != "" && !strings.HasPrefix(path, "/oauth/") {
		separator := "?"
		if strings.Contains(requestURL, "?") {
			separator = "&"
		}
		requestURL += separator + "access_token=" + url.QueryEscape(c.accessToken)
	}
	req, err := http.NewRequestWithContext(ctx, method, requestURL, body)
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
	if strings.Contains(c.base, "pro.p-on.ru") {
		var token struct {
			AccessToken string `json:"access_token"`
		}
		if err := c.requestWithAuth(ctx, "/oauth/token", "Basic cGNvbm5lY3Q6SW5mXzRlUm05X2ZfaEhnVl9zNg==", &token); err != nil {
			return err
		}
		if token.AccessToken == "" {
			return errors.New("OAuth response lacks access_token")
		}
		c.accessToken = token.AccessToken
	}
	var response struct {
		Session string `json:"session_id"`
	}
	form := url.Values{"login": {c.username}, "password": {c.password}, "lang": {"ru"}}
	if c.accessToken != "" {
		form.Set("v", "3")
		_, offset := time.Now().Zone()
		form.Set("utc_offset", strconv.Itoa(offset/60))
		form.Set("access_token", c.accessToken)
	}
	if err := c.request(ctx, "/api/users/login", form, &response); err != nil {
		return err
	}
	if response.Session == "" && c.accessToken == "" {
		return errors.New("login response lacks session_id")
	}
	c.logged = true
	return nil
}

func (c *Client) requestWithAuth(ctx context.Context, path, authorization string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, nil)
	if err != nil {
		return errors.New("create API request")
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := c.http.Do(req)
	if err != nil {
		return errors.New("API transport failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API HTTP status %d", resp.StatusCode)
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024))
	dec.UseNumber()
	if err := dec.Decode(out); err != nil {
		return errors.New("invalid API JSON")
	}
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

// WebSocketStates reads the initial/state messages used by the extended HA
// integration. Some values (notably motohours) are not present in HTTP stats.
func (c *Client) WebSocketStates(ctx context.Context, timeout time.Duration) (map[string]Object, error) {
	if !strings.Contains(c.base, "pro.p-on.ru") {
		return nil, nil
	}
	if !c.logged {
		if err := c.login(ctx); err != nil {
			return nil, err
		}
	}
	u := strings.Replace(c.base, "https://", "wss://", 1)
	u = strings.Replace(u, "http://", "ws://", 1) + "/api/v4/updates/ws?access_token=" + url.QueryEscape(c.accessToken)
	dialer := websocket.Dialer{HandshakeTimeout: timeout}
	ws, _, err := dialer.DialContext(ctx, u, http.Header{"Origin": []string{c.base}, "User-Agent": []string{"Mozilla/5.0"}})
	if err != nil {
		return nil, errors.New("WebSocket connection failed")
	}
	defer ws.Close()
	_ = ws.SetReadDeadline(time.Now().Add(timeout))
	states := map[string]Object{}
	for {
		_, body, err := ws.ReadMessage()
		if err != nil {
			break
		}
		var message struct {
			Type string `json:"type"`
			Data Object `json:"data"`
		}
		if json.Unmarshal(body, &message) != nil || message.Data == nil {
			continue
		}
		if message.Type != "initial-state" && message.Type != "state" {
			continue
		}
		id := fmt.Sprint(message.Data["dev_id"])
		if id == "<nil>" {
			id = fmt.Sprint(message.Data["id"])
		}
		if id == "<nil>" || id == "" {
			continue
		}
		if states[id] == nil {
			states[id] = Object{}
		}
		states[id] = mergeState(states[id], message.Data)
		// Initial state is sent for every device; once all are received the read
		// deadline still protects us from waiting for an idle socket.
	}
	return states, nil
}
