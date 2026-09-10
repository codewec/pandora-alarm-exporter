package pandora

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRequestedSensors(t *testing.T) {
	e := NewExporter(nil, false, time.Minute)
	e.devices = []Device{{ID: "car"}}
	var s Object
	if err := json.Unmarshal([]byte(`{"mileage":"12.5","mileage_CAN":18,"engine_temp":90,"cabin_temp":22,"out_temp":-5,"gsm_level":3,"move":1}`), &s); err != nil {
		t.Fatal(err)
	}
	e.stats["car"] = s
	var bits uint64
	expected := map[uint]string{3: "ignition", 10: "immobilizer", 21: "front_left_door_open", 22: "front_right_door_open", 23: "rear_left_door_open", 24: "rear_right_door_open", 25: "trunk_open", 26: "hood_open", 27: "parking_brake", 28: "brake"}
	for bit := range expected {
		bits |= 1 << bit
	}
	s["bit_state_1"] = fmt.Sprint(bits)
	scrape := func() string {
		w := httptest.NewRecorder()
		e.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
		return w.Body.String()
	}
	text := scrape()
	for name, value := range map[string]string{"gps_mileage_meters": "12500", "can_mileage_meters": "18000", "engine_temperature_celsius": "90", "cabin_temperature_celsius": "22", "ambient_temperature_celsius": "-5", "gsm_level": "3", "moving": "1"} {
		want := fmt.Sprintf("pandora_%s{device_id=\"car\"} %s\n", name, value)
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, state := range expected {
		if !strings.Contains(text, fmt.Sprintf("pandora_state{device_id=\"car\",state=%q} 1\n", state)) {
			t.Errorf("missing active state %s", state)
		}
	}
	s["bit_state_1"] = "0"
	text = scrape()
	for _, state := range expected {
		if !strings.Contains(text, fmt.Sprintf("pandora_state{device_id=\"car\",state=%q} 0\n", state)) {
			t.Errorf("missing inactive state %s", state)
		}
	}
}

func TestEngineHours(t *testing.T) {
	for _, tc := range []struct {
		name, data, hours, canHours string
	}{
		{"top-level", `{"motohours":"123.5","motohours_CAN":456.25}`, "123.5", "456.25"},
		{"nested CAN priority", `{"motohours":1,"motohours_CAN":2,"can":{"motohours":"3.5","motohours_CAN":4.5}}`, "3.5", "4.5"},
		{"zero", `{"motohours":0,"motohours_CAN":0}`, "0", "0"},
		{"missing", `{}`, "", ""},
		{"invalid", `{"motohours":null,"motohours_CAN":"NaN"}`, "", ""},
		{"nested null overrides", `{"motohours_CAN":12,"can":{"motohours_CAN":null}}`, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewExporter(nil, false, time.Minute)
			e.devices = []Device{{ID: "car"}}
			var s Object
			if err := json.Unmarshal([]byte(tc.data), &s); err != nil {
				t.Fatal(err)
			}
			e.stats["car"] = s
			w := httptest.NewRecorder()
			e.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
			for name, want := range map[string]string{"engine_hours": tc.hours, "can_engine_hours": tc.canHours} {
				prefix := fmt.Sprintf("pandora_%s{device_id=\"car\"} ", name)
				if want == "" {
					if strings.Contains(w.Body.String(), prefix) {
						t.Errorf("unexpected %s", name)
					}
				} else if !strings.Contains(w.Body.String(), prefix+want+"\n") {
					t.Errorf("missing %s = %s", name, want)
				}
			}
		})
	}
}
