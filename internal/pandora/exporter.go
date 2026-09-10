package pandora

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Exporter struct {
	client      *Client
	mu          sync.RWMutex
	devices     []Device
	stats       map[string]Object
	times       map[string]Object
	cursor      int64
	full        time.Time
	last        time.Time
	duration    float64
	success     bool
	errors      uint64
	coordinates bool
	maxAge      time.Duration
}

func NewExporter(c *Client, coordinates bool, maxAge time.Duration) *Exporter {
	return &Exporter{client: c, stats: map[string]Object{}, times: map[string]Object{}, coordinates: coordinates, maxAge: maxAge}
}

// Poll must be called by a single background goroutine. Scrapes never call the API.
func (e *Exporter) Poll(ctx context.Context) (err error) {
	start := time.Now()
	defer func() {
		e.mu.Lock()
		defer e.mu.Unlock()
		e.duration = time.Since(start).Seconds()
		e.success = err == nil
		if err != nil {
			e.errors++
		} else {
			e.last = time.Now()
		}
	}()
	full := e.full.IsZero() || time.Since(e.full) >= 5*time.Minute
	devices := e.devices
	cursor := e.cursor - 1
	if full {
		devices, err = e.client.Devices(ctx)
		if err != nil {
			return err
		}
		cursor = -1
	}
	u, err := e.client.Updates(ctx, cursor)
	if err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if full {
		e.devices = devices
		oldStats := e.stats
		e.stats = map[string]Object{}
		// Retain last valid position for still-present devices, like the HA tracker.
		for _, d := range devices {
			old := oldStats[d.Key()]
			if _, _, ok := validCoordinates(old); ok {
				e.stats[d.Key()] = Object{"x": old["x"], "y": old["y"]}
			}
		}
		e.times = map[string]Object{}
		e.full = time.Now()
	}
	for _, d := range devices {
		id := d.Key()
		if e.stats[id] == nil {
			e.stats[id] = Object{}
		}
		e.stats[id] = mergeState(e.stats[id], u.Stats[id])
		if e.times[id] == nil {
			e.times[id] = Object{}
		}
		for k, v := range u.Time[id] {
			e.times[id][k] = v
		}
	}
	e.cursor, _ = u.TS.Int64()
	return nil
}
func number(v any) (float64, bool) {
	var n float64
	var err error
	switch x := v.(type) {
	case json.Number:
		n, err = x.Float64()
	case string:
		n, err = strconv.ParseFloat(x, 64)
	case float64:
		n = x
	case bool:
		if x {
			n = 1
		}
	default:
		return 0, false
	}
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

type field struct {
	key, name, help string
	scale           float64
}

var fields = []field{
	{"voltage", "battery_voltage_volts", "Battery voltage in volts.", 1},
	{"engine_temp", "engine_temperature_celsius", "Engine temperature in Celsius.", 1},
	{"cabin_temp", "cabin_temperature_celsius", "Cabin temperature in Celsius.", 1},
	{"out_temp", "ambient_temperature_celsius", "Ambient temperature in Celsius.", 1},
	{"fuel", "fuel_ratio", "Fuel level as a ratio.", 0.01},
	{"speed", "speed_meters_per_second", "Speed in meters per second.", 1.0 / 3.6},
	{"mileage", "gps_mileage_meters", "GPS mileage in meters.", 1000},
	{"mileage_CAN", "can_mileage_meters", "CAN mileage in meters.", 1000},
	{"engine_rpm", "engine_rpm", "Engine revolutions per minute.", 1},
	{"gsm_level", "gsm_level", "GSM level in API native units.", 1},
	{"dtime_rec", "data_timestamp_seconds", "API stats recording Unix timestamp.", 1},
	{"dtime", "device_timestamp_seconds", "Device Unix timestamp; may differ from server clock.", 1},
	{"active_sim", "active_sim", "Active SIM index.", 1},
}
var flags = map[uint]string{
	0: "armed", 1: "alarm", 2: "engine_running", 3: "ignition", 4: "autostart_active",
	5: "handsfree_lock", 6: "handsfree_unlock", 7: "gsm_enabled", 8: "gps_enabled", 9: "tracking_enabled", 10: "immobilizer",
	11: "extra_sensor_warning_disabled", 12: "extra_sensor_main_disabled", 13: "shock_sensor_warning_disabled", 14: "shock_sensor_main_disabled",
	15: "autostart_scheduled", 16: "sms_enabled", 17: "calls_enabled", 18: "lights", 19: "siren_warning_disabled", 20: "siren_disabled",
	21: "front_left_door_open", 22: "front_right_door_open", 23: "rear_left_door_open", 24: "rear_right_door_open",
	25: "trunk_open", 26: "hood_open", 27: "parking_brake", 28: "brake", 29: "coolant_heater", 30: "active_security",
	31: "heater_scheduled", 33: "evacuation_mode", 34: "service_mode", 35: "stay_home", 60: "tag_polling_disabled", 61: "disarm_without_tag_disabled",
}

func quote(s string) string {
	return `"` + strings.NewReplacer("\\", "\\\\", "\n", "\\n", `"`, `\"`).Replace(s) + `"`
}

type metric struct {
	help, kind string
	samples    []string
}
type exposition map[string]*metric

func (m exposition) add(name, help, kind, labels string, value float64) {
	name = "pandora_" + name
	if m[name] == nil {
		m[name] = &metric{help: help, kind: kind}
	}
	m[name].samples = append(m[name].samples, name+labels+" "+strconv.FormatFloat(value, 'g', -1, 64)+"\n")
}
func (e *Exporter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	m := exposition{}
	success := 0.
	if e.success {
		success = 1
	}
	m.add("scrape_success", "Whether the latest background API poll succeeded.", "gauge", "", success)
	m.add("poll_errors_total", "Failed background polls.", "counter", "", float64(e.errors))
	m.add("poll_duration_seconds", "Duration of the last background poll.", "gauge", "", e.duration)
	last := 0.
	if !e.last.IsZero() {
		last = float64(e.last.Unix())
	}
	m.add("last_success_timestamp_seconds", "Last successful API poll Unix timestamp.", "gauge", "", last)
	m.add("devices", "Number of discovered devices.", "gauge", "", float64(len(e.devices)))
	for _, d := range e.devices {
		id := d.Key()
		s := normalizeState(e.stats[id])
		l := "{device_id=" + quote(id) + "}"
		add := func(name, help string, v float64) { m.add(name, help, "gauge", l, v) }
		m.add("device_info", "Device metadata.", "gauge", "{device_id="+quote(id)+",name="+quote(d.Name)+",model="+quote(d.Model)+",firmware="+quote(d.Firmware)+"}", 1)
		for _, f := range append(append([]field{}, fields...), extraFields...) {
			if v, ok := number(s[f.key]); ok {
				add(f.name, f.help, v*f.scale)
			}
		}
		for _, f := range binaryFields {
			if v, ok := number(s[f.key]); ok {
				value := 0.0
				if v != 0 {
					value = 1
				}
				if f.inverse {
					value = 1 - value
				}
				add(f.name, "Binary state matching HA; 1 means active.", value)
			}
		}
		for key, name := range map[string]string{"heater_errors": "heater_errors", "OBD_codes": "obd_errors"} {
			if values, ok := s[key].([]any); ok {
				active := 0.0
				if len(values) > 0 {
					active = 1
				}
				add(name, "Whether the API reports any diagnostic errors.", active)
			}
		}
		for key, name := range map[string]string{"command": "last_command_timestamp_seconds", "setting": "last_settings_timestamp_seconds"} {
			if v, ok := number(e.times[id][key]); ok {
				add(name, "Last operation Unix timestamp.", v)
			}
		}
		if v, ok := number(e.times[id]["online"]); ok {
			add("last_online_timestamp_seconds", "Last online Unix timestamp.", v)
		}
		if v, ok := number(s["dtime_rec"]); ok {
			age := math.Max(0, float64(time.Now().Unix())-v)
			add("data_age_seconds", "Age of API stats in seconds.", age)
			stale := 0.
			if age > e.maxAge.Seconds() {
				stale = 1
			}
			add("data_stale", "Whether stats exceed configured maximum age.", stale)
		}
		if fuel, ok := number(s["fuel"]); ok {
			if tank, ok := number(d.FuelTank); ok && tank > 0 {
				add("fuel_liters", "Estimated fuel volume from configured tank capacity.", fuel*tank/100)
			}
		}
		if e.coordinates {
			if lat, lon, valid := validCoordinates(s); valid {
				add("latitude_degrees", "GPS latitude in degrees.", lat)
				add("longitude_degrees", "GPS longitude in degrees.", lon)
			}
		}
		if raw, ok := s["bit_state_1"]; ok {
			bits, err := strconv.ParseUint(fmt.Sprint(raw), 10, 64)
			if err == nil {
				for bit, name := range flags {
					m.add("state", "Decoded bit_state_1 flag; 1 means active.", "gauge", "{device_id="+quote(id)+",state="+quote(name)+"}", float64((bits>>bit)&1))
				}
			}
		}
		for i, key := range []string{"balance", "balance1"} {
			if b, ok := s[key].(map[string]any); ok {
				if v, ok := number(b["value"]); ok {
					currency, _ := b["cur"].(string)
					m.add("sim_balance", "SIM balance in currency units.", "gauge", "{device_id="+quote(id)+",sim="+quote(strconv.Itoa(i))+",currency="+quote(currency)+"}", v)
				}
			}
		}
	}
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	for _, n := range names {
		v := m[n]
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s %s\n", n, v.help, n, v.kind)
		sort.Strings(v.samples)
		for _, s := range v.samples {
			fmt.Fprint(w, s)
		}
	}
}
