// Package weather provides weather data abstractions for multiple stations
package weather

import "time"

// MarketType represents the type of temperature market
type MarketType string

const (
	MarketTypeHigh MarketType = "HIGH"
	MarketTypeLow  MarketType = "LOW"
)

// Station represents a weather observation station with Kalshi market integration
type Station struct {
	// Identification
	ID     string // METAR station ID (e.g., "KLAX")
	Name   string // Full name (e.g., "Los Angeles International Airport")
	City   string // City name for display
	State  string // State abbreviation

	// Location
	Timezone string  // IANA timezone (e.g., "America/Los_Angeles")
	Lat      float64 // Latitude
	Lon      float64 // Longitude

	// Kalshi Integration - prefix for high temp markets (e.g., "KXHIGHLAX")
	// Low temp markets use different prefix (e.g., "KXLOWTLAX")
	EventPrefix string

	// NWS Integration
	NWSOffice string // NWS office code (e.g., "LOX")
	NWSGridX  int    // NWS grid X coordinate
	NWSGridY  int    // NWS grid Y coordinate

	// Climatology (monthly average temperatures in °F)
	MonthlyAvgHigh map[time.Month]float64
	MonthlyAvgLow  map[time.Month]float64
}

// Stations is the registry of all supported weather stations
// Based on Kalshi daily temperature markets:
// HIGH: https://kalshi.com/markets/kxhigh{city}/
// LOW:  https://kalshi.com/markets/kxlowt{city}/
var Stations = map[string]*Station{
	"LAX": {
		ID:          "KLAX",
		Name:        "Los Angeles International Airport",
		City:        "Los Angeles",
		State:       "CA",
		Timezone:    "America/Los_Angeles",
		Lat:         33.9425,
		Lon:         -118.4081,
		EventPrefix: "KXHIGHLAX", // Also KXLOWTLAX for low temps
		NWSOffice:   "LOX",
		NWSGridX:    154,
		NWSGridY:    44,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   68, time.February: 69, time.March: 70,
			time.April:     72, time.May: 74, time.June: 78,
			time.July:      83, time.August: 84, time.September: 83,
			time.October:   79, time.November: 73, time.December: 68,
		},
	},
	"NYC": {
		ID:          "KJFK",
		Name:        "John F. Kennedy International Airport",
		City:        "New York City",
		State:       "NY",
		Timezone:    "America/New_York",
		Lat:         40.6413,
		Lon:         -73.7781,
		EventPrefix: "KXHIGHNY", // NYC uses "NY" not "NYC"
		NWSOffice:   "OKX",
		NWSGridX:    33,
		NWSGridY:    37,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   39, time.February: 42, time.March: 50,
			time.April:     61, time.May: 71, time.June: 79,
			time.July:      84, time.August: 83, time.September: 76,
			time.October:   65, time.November: 54, time.December: 44,
		},
	},
	"CHI": {
		ID:          "KORD",
		Name:        "Chicago O'Hare International Airport",
		City:        "Chicago",
		State:       "IL",
		Timezone:    "America/Chicago",
		Lat:         41.9742,
		Lon:         -87.9073,
		EventPrefix: "KXHIGHCHI", // Also KXLOWTCHI for low temps
		NWSOffice:   "LOT",
		NWSGridX:    65,
		NWSGridY:    76,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   32, time.February: 36, time.March: 47,
			time.April:     59, time.May: 70, time.June: 80,
			time.July:      84, time.August: 82, time.September: 75,
			time.October:   62, time.November: 48, time.December: 35,
		},
	},
	"MIA": {
		ID:          "KMIA",
		Name:        "Miami International Airport",
		City:        "Miami",
		State:       "FL",
		Timezone:    "America/New_York",
		Lat:         25.7959,
		Lon:         -80.2870,
		EventPrefix: "KXHIGHMIA", // Also KXLOWTMIA for low temps
		NWSOffice:   "MFL",
		NWSGridX:    109,
		NWSGridY:    50,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   76, time.February: 78, time.March: 80,
			time.April:     83, time.May: 87, time.June: 89,
			time.July:      91, time.August: 91, time.September: 89,
			time.October:   86, time.November: 81, time.December: 77,
		},
	},
	"AUS": {
		ID:          "KAUS",
		Name:        "Austin-Bergstrom International Airport",
		City:        "Austin",
		State:       "TX",
		Timezone:    "America/Chicago",
		Lat:         30.1975,
		Lon:         -97.6664,
		EventPrefix: "KXHIGHAUS", // Also KXLOWTAUS for low temps
		NWSOffice:   "EWX",
		NWSGridX:    156,
		NWSGridY:    91,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   62, time.February: 66, time.March: 73,
			time.April:     80, time.May: 86, time.June: 93,
			time.July:      97, time.August: 98, time.September: 92,
			time.October:   82, time.November: 71, time.December: 63,
		},
	},
	"PHIL": {
		ID:          "KPHL",
		Name:        "Philadelphia International Airport",
		City:        "Philadelphia",
		State:       "PA",
		Timezone:    "America/New_York",
		Lat:         39.8721,
		Lon:         -75.2411,
		EventPrefix: "KXHIGHPHIL", // Also KXLOWTPHIL for low temps
		NWSOffice:   "PHI",
		NWSGridX:    49,
		NWSGridY:    75,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   40, time.February: 44, time.March: 53,
			time.April:     64, time.May: 74, time.June: 83,
			time.July:      87, time.August: 85, time.September: 78,
			time.October:   67, time.November: 55, time.December: 45,
		},
	},
	"DEN": {
		ID:          "KDEN",
		Name:        "Denver International Airport",
		City:        "Denver",
		State:       "CO",
		Timezone:    "America/Denver",
		Lat:         39.8561,
		Lon:         -104.6737,
		EventPrefix: "KXHIGHDEN", // Also KXLOWTDEN for low temps
		NWSOffice:   "BOU",
		NWSGridX:    62,
		NWSGridY:    60,
		MonthlyAvgHigh: map[time.Month]float64{
			time.January:   45, time.February: 48, time.March: 55,
			time.April:     62, time.May: 71, time.June: 82,
			time.July:      88, time.August: 86, time.September: 78,
			time.October:   65, time.November: 52, time.December: 45,
		},
	},
}

// GetStation returns a station by its short code (LAX, MIA, DEN, CHI)
func GetStation(code string) *Station {
	return Stations[code]
}

// GetStationByMETAR returns a station by its METAR ID (KLAX, KMIA, etc.)
func GetStationByMETAR(metarID string) *Station {
	for _, s := range Stations {
		if s.ID == metarID {
			return s
		}
	}
	return nil
}

// GetStationByEventPrefix returns a station by its Kalshi event prefix
func GetStationByEventPrefix(prefix string) *Station {
	for _, s := range Stations {
		if s.EventPrefix == prefix {
			return s
		}
	}
	return nil
}

// AllStations returns all registered stations
func AllStations() []*Station {
	result := make([]*Station, 0, len(Stations))
	for _, s := range Stations {
		result = append(result, s)
	}
	return result
}

// Location returns the timezone-aware location for the station
func (s *Station) Location() *time.Location {
	loc, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

// GetClimatologyHigh returns the average high temperature for a given month
func (s *Station) GetClimatologyHigh(month time.Month) float64 {
	if avg, ok := s.MonthlyAvgHigh[month]; ok {
		return avg
	}
	return 65 // Default fallback
}

// GetClimatologyLow returns the average low temperature for a given month
func (s *Station) GetClimatologyLow(month time.Month) float64 {
	if avg, ok := s.MonthlyAvgLow[month]; ok {
		return avg
	}
	return 45 // Default fallback
}

// EventTicker generates the Kalshi event ticker for a given date and market type
func (s *Station) EventTicker(date time.Time) string {
	return s.HighEventTicker(date)
}

// HighEventTicker generates the Kalshi event ticker for HIGH temp markets
func (s *Station) HighEventTicker(date time.Time) string {
	return s.EventPrefix + "-" + date.Format("06Jan02")
}

// LowEventTicker generates the Kalshi event ticker for LOW temp markets
func (s *Station) LowEventTicker(date time.Time) string {
	// Convert KXHIGHLAX -> KXLOWTLAX, KXHIGHNY -> KXLOWTNY, etc.
	prefix := s.EventPrefix
	if len(prefix) > 6 && prefix[:6] == "KXHIGH" {
		prefix = "KXLOWT" + prefix[6:]
	}
	return prefix + "-" + date.Format("06Jan02")
}

// EventTickerForType generates the Kalshi event ticker for a given market type
func (s *Station) EventTickerForType(date time.Time, marketType MarketType) string {
	if marketType == MarketTypeLow {
		return s.LowEventTicker(date)
	}
	return s.HighEventTicker(date)
}

// NWSForecastURL returns the NWS API forecast URL for this station
func (s *Station) NWSForecastURL() string {
	return "https://api.weather.gov/gridpoints/" + s.NWSOffice + "/" +
		itoa(s.NWSGridX) + "," + itoa(s.NWSGridY) + "/forecast"
}

// METARHistoryURL returns the Iowa State ASOS URL for historical METAR data
func (s *Station) METARHistoryURL(date time.Time) string {
	// Remove the 'K' prefix for Iowa State
	stationID := s.ID
	if len(stationID) > 1 && stationID[0] == 'K' {
		stationID = stationID[1:]
	}

	return "https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?" +
		"station=" + stationID +
		"&data=tmpf" +
		"&year1=" + itoa(date.Year()) +
		"&month1=" + itoa(int(date.Month())) +
		"&day1=" + itoa(date.Day()) +
		"&year2=" + itoa(date.Year()) +
		"&month2=" + itoa(int(date.Month())) +
		"&day2=" + itoa(date.Day()+1) +
		"&tz=" + s.Timezone +
		"&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3"
}

func itoa(i int) string {
	if i < 0 {
		return "-" + itoa(-i)
	}
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa(i/10) + string(rune('0'+i%10))
}

