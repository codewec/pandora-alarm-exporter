package pandora

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPollAndMetrics(t *testing.T) {
	var login, updates int
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/users/login":
			login++
			if r.Method != "POST" {
				t.Error("login method")
			}
			r.ParseForm()
			if r.Form.Get("password") != "secret" {
				t.Error("password form")
			}
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "valid", Path: "/"})
			fmt.Fprint(w, `{"session_id":"abc","status":"success"}`)
		case "/api/devices":
			if _, err := r.Cookie("session"); err != nil {
				t.Error("missing cookie")
			}
			fmt.Fprint(w, `[{"id":1234,"name":"Car\n\"test","model":"DXL","firmware":"1","fuel_tank":50}]`)
		case "/api/updates":
			updates++
			if fail {
				w.WriteHeader(500)
				return
			}
			if updates == 1 {
				fmt.Fprint(w, `{"status":"sid-expired"}`)
				return
			}
			if updates == 2 {
				if r.URL.Query().Get("ts") != "-1" {
					t.Error("initial cursor")
				}
				fmt.Fprintf(w, `{"ts":100,"stats":{"1234":{"fuel":50,"speed":36,"mileage":"1.5","bit_state_1":"2305843009213693957","x":55,"y":83,"dtime_rec":%d,"balance":{"value":"12.34","cur":"RUB"}}},"time":{"1234":{"online":90}}}`, time.Now().Unix()-100)
			} else {
				wantCursor := "100"
				if updates == 3 {
					wantCursor = "99"
				}
				if r.URL.Query().Get("ts") != wantCursor {
					t.Error("incremental cursor")
				}
				fmt.Fprint(w, `{"ts":101,"stats":{"1234":{"speed":72}},"time":{"1234":{"online":95}}}`)
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	c, _ := NewClient(server.URL, "user", "secret", time.Second)
	e := NewExporter(c, false, time.Minute)
	if err := e.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if login != 2 {
		t.Fatalf("logins: %d", login)
	}
	if err := e.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	scrape := func() string {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		return w.Body.String()
	}
	text := scrape()
	for _, want := range []string{
		`pandora_fuel_ratio{device_id="1234"} 0.5`,
		`pandora_fuel_liters{device_id="1234"} 25`,
		`pandora_speed_meters_per_second{device_id="1234"} 20`,
		`pandora_gps_mileage_meters{device_id="1234"} 1500`,
		`pandora_state{device_id="1234",state="disarm_without_tag_disabled"} 1`,
		`pandora_state{device_id="1234",state="engine_running"} 1`,
		`pandora_state{device_id="1234",state="armed"} 1`,
		`pandora_data_stale{device_id="1234"} 1`,
		`pandora_last_online_timestamp_seconds{device_id="1234"} 95`,
		`name="Car\n\"test"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s\n%s", want, text)
		}
	}
	if strings.Contains(text, "latitude") || strings.Contains(text, "battery_voltage") {
		t.Error("absent/disabled values exported")
	}
	fail = true
	if e.Poll(context.Background()) == nil {
		t.Fatal("expected error")
	}
	text = scrape()
	if !strings.Contains(text, "pandora_scrape_success 0") || !strings.Contains(text, "pandora_poll_errors_total 1") || !strings.Contains(text, `pandora_fuel_ratio{device_id="1234"} 0.5`) {
		t.Fatal(text)
	}
	fail = false
	// A full snapshot removes absent values and devices' previous cached fields.
	e.full = time.Now().Add(-6 * time.Minute)
	updates = 1
	if err := e.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			scrape()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			if err := e.Poll(context.Background()); err != nil {
				t.Error(err)
			}
		}
	}()
	wg.Wait()
}
func TestInvalidResponsesAndBoundedRetry(t *testing.T) {
	for _, body := range []string{`{"ts":null}`, `{"ts":0}`, `{"ts":1}{}`, `not json`, `{"status":"fail","error_text":"secret"}`, `{"status":"Invalid session"}`} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path == "/api/users/login" {
					fmt.Fprint(w, `{"session_id":"ok"}`)
				} else {
					fmt.Fprint(w, body)
				}
			}))
			defer s.Close()
			c, _ := NewClient(s.URL, "u", "p", time.Second)
			_, err := c.Updates(context.Background(), -1)
			if err == nil {
				t.Fatal("expected error")
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("error leaked response")
			}
			if calls > 4 {
				t.Fatal("unbounded retry")
			}
		})
	}
}
