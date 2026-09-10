package pandora

import "math"

var stickyStateFields = []string{"motohours", "motohours_CAN", "fuel"}

// Flatten each update before merging the cache, as CurrentState does for HTTP.
// A new root value must not be shadowed by an older cached CAN value.
func normalizeState(s Object) Object {
	result := Object{}
	for key, value := range s {
		if key != "can" && key != "heater" {
			result[key] = value
		}
	}
	for _, group := range []string{"can", "heater"} {
		if nested, ok := s[group].(map[string]any); ok {
			for key, value := range nested {
				result[key] = value
			}
		}
	}
	// HTTP connectivity comes from stats.online, not online_mode or a nested value.
	if value, ok := s["online"]; ok {
		result["online"] = value
	}
	return result
}

func validCoordinates(s Object) (float64, float64, bool) {
	lat, latOK := number(s["x"])
	if !latOK {
		lat, latOK = number(s["latitude"])
	}
	lon, lonOK := number(s["y"])
	if !lonOK {
		lon, lonOK = number(s["longitude"])
	}
	return lat, lon, latOK && lonOK && lat != 0 && lon != 0 && math.Abs(lat) <= 90 && math.Abs(lon) <= 180
}

func coordinateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const radians = math.Pi / 180
	dLat, dLon := (lat2-lat1)*radians, (lon2-lon1)*radians
	a := math.Pow(math.Sin(dLat/2), 2) + math.Cos(lat1*radians)*math.Cos(lat2*radians)*math.Pow(math.Sin(dLon/2), 2)
	return 6371008.8 * 2 * math.Asin(math.Sqrt(math.Min(1, a)))
}

// Match the HA tracker's default 10-meter coordinate debounce.
func mergeState(previous, update Object) Object {
	result := normalizeState(update)
	lat, lon, valid := validCoordinates(result)
	oldLat, oldLon, oldValid := validCoordinates(previous)
	if !valid || (oldValid && coordinateDistance(oldLat, oldLon, lat, lon) < 10) {
		delete(result, "x")
		delete(result, "y")
	}
	for key, value := range result {
		// Pandora sometimes sends null/omits telemetry between engine starts.
		// Never erase a previously valid value on such an update.
		if value == nil {
			delete(result, key)
			continue
		}
		if stickyTelemetry(key) {
			if current, ok := number(previous[key]); ok {
				if incoming, ok := number(value); ok && current > 0 && incoming == 0 {
					delete(result, key)
					continue
				}
			}
		}
		previous[key] = value
	}
	return previous
}

func stickyTelemetry(key string) bool {
	for _, candidate := range stickyStateFields {
		if key == candidate {
			return true
		}
	}
	return false
}

var extraFields = []field{
	{"motohours", "engine_hours", "Accumulated engine runtime in hours.", 1},
	{"motohours_CAN", "can_engine_hours", "CAN accumulated engine runtime in hours.", 1},
	{"CAN_mileage_to_empty", "can_range_meters", "CAN range to empty in meters; native miles per HA.", 1609.344},
	{"CAN_mileage_by_battery", "can_battery_range_meters", "CAN battery range in meters.", 1000},
	{"CAN_consumption", "fuel_consumption", "CAN fuel consumption in API native units.", 1},
	{"SOC", "battery_charge_ratio", "EV battery state of charge ratio.", .01},
	{"battery_temperature", "battery_temperature_celsius", "Battery temperature in Celsius.", 1},
	{"heater_temperature", "heater_temperature_celsius", "Heater temperature in Celsius.", 1},
	{"internal_power", "internal_voltage_volts", "Internal power supply voltage.", 1},
	{"heater_voltage", "heater_voltage_volts", "Heater voltage.", 1},
	// These counterintuitive axle mappings are present in pandora-cas 0.0.16.
	{"CAN_TMPS_back_left", "front_left_tire_pressure_pascals", "Front left tire pressure; HA 0.0.16 mapping.", 100000},
	{"CAN_TMPS_back_right", "front_right_tire_pressure_pascals", "Front right tire pressure; HA 0.0.16 mapping.", 100000},
	{"CAN_TMPS_forvard_left", "rear_left_tire_pressure_pascals", "Rear left tire pressure; HA 0.0.16 mapping.", 100000},
	{"CAN_TMPS_forvard_right", "rear_right_tire_pressure_pascals", "Rear right tire pressure; HA 0.0.16 mapping.", 100000},
	{"CAN_TMPS_reserve", "reserve_tire_pressure_pascals", "Reserve tire pressure.", 100000},
	{"brelok", "key_number", "Key number reported by API.", 1},
	{"metka", "tag_number", "Tag number reported by API.", 1},
	{"CAN_days_to_maintenance", "maintenance_remaining_seconds", "Time remaining to maintenance in seconds.", 86400},
	{"engine_remains", "engine_remaining_runtime_seconds", "Remaining engine runtime in seconds.", 60},
	{"state_utc", "last_state_update_timestamp_seconds", "Last state update Unix timestamp from HA model.", 1},
}

type binaryField struct {
	key, name string
	inverse   bool
}

var binaryFields = []binaryField{
	{"online", "online", false}, {"move", "moving", false},
	{"CAN_driver_glass", "driver_window_open", false},
	{"CAN_passenger_glass", "passenger_window_open", false},
	{"CAN_back_left_glass", "rear_left_window_open", false},
	{"CAN_back_right_glass", "rear_right_window_open", false},
	{"CAN_driver_belt", "driver_seatbelt_unfastened", true},
	{"CAN_passenger_belt", "passenger_seatbelt_unfastened", true},
	{"CAN_back_left_belt", "rear_left_seatbelt_unfastened", true},
	{"CAN_back_right_belt", "rear_right_seatbelt_unfastened", true},
	{"CAN_back_center_belt", "rear_center_seatbelt_unfastened", true},
	{"CAN_seat_taken", "seat_occupied", false},
	{"charging_connect", "ev_charging_connected", false},
	{"CAN_low_liquid", "low_liquid", false},
}
