package feeds

import (
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// METARStation represents a weather station
type METARStation struct {
	Code     string
	City     string
	Timezone string
}

// METARData represents temperature data for a station
type METARData struct {
	Station    string
	MaxTemp    int       // Running max temperature (°F)
	LastTemp   int       // Last observed temperature (°F)
	Updated    time.Time
	Readings   int       // Number of readings today
}

// METARFeed provides temperature data from METAR observations
type METARFeed struct {
	httpClient *http.Client
	stations   []METARStation

	mu   sync.RWMutex
	data map[string]*METARData // Station code -> data

	pollInterval time.Duration
	stopChan     chan struct{}
}

// NewMETARFeed creates a new METAR feed
func NewMETARFeed(stations []METARStation, pollInterval time.Duration) *METARFeed {
	return &METARFeed{
		httpClient:   &http.Client{Timeout: 15 * time.Second},
		stations:     stations,
		data:         make(map[string]*METARData),
		pollInterval: pollInterval,
		stopChan:     make(chan struct{}),
	}
}

// Start begins polling for METAR data
func (f *METARFeed) Start(ctx context.Context) {
	log.Printf("[METAR] Starting feed with %d stations, poll interval %v",
		len(f.stations), f.pollInterval)

	// Initial fetch
	f.fetchAll()

	ticker := time.NewTicker(f.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopChan:
			return
		case <-ticker.C:
			f.fetchAll()
		}
	}
}

// Stop stops the METAR feed
func (f *METARFeed) Stop() {
	close(f.stopChan)
}

// GetData returns the current METAR data for a station
func (f *METARFeed) GetData(stationCode string) *METARData {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.data[stationCode]
}

// GetMaxTemp returns the running max temperature for a station
func (f *METARFeed) GetMaxTemp(stationCode string) int {
	data := f.GetData(stationCode)
	if data == nil {
		return 0
	}
	return data.MaxTemp
}

func (f *METARFeed) fetchAll() {
	for _, station := range f.stations {
		if err := f.fetchStation(station); err != nil {
			log.Printf("[METAR] Error fetching %s: %v", station.Code, err)
		}
	}
}

func (f *METARFeed) fetchStation(station METARStation) error {
	loc, err := time.LoadLocation(station.Timezone)
	if err != nil {
		return err
	}

	now := time.Now().In(loc)

	url := fmt.Sprintf(
		"https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py?station=%s&data=tmpf&year1=%d&month1=%d&day1=%d&year2=%d&month2=%d&day2=%d&tz=%s&format=onlycomma&latlon=no&elev=no&missing=M&trace=T&direct=no&report_type=3",
		station.Code,
		now.Year(), int(now.Month()), now.Day(),
		now.Year(), int(now.Month()), now.Day()+1,
		station.Timezone,
	)

	resp, err := f.httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	lines := strings.Split(string(body), "\n")
	maxTemp := -999.0
	lastTemp := -999.0
	readings := 0

	for _, line := range lines {
		if strings.HasPrefix(line, station.Code+",") {
			parts := strings.Split(line, ",")
			if len(parts) >= 3 && parts[2] != "M" {
				var temp float64
				fmt.Sscanf(parts[2], "%f", &temp)
				if temp > -100 && temp < 150 {
					lastTemp = temp
					readings++
					if temp > maxTemp {
						maxTemp = temp
					}
				}
			}
		}
	}

	if maxTemp == -999.0 {
		return fmt.Errorf("no valid readings")
	}

	f.mu.Lock()
	f.data[station.Code] = &METARData{
		Station:  station.Code,
		MaxTemp:  int(math.Round(maxTemp)),
		LastTemp: int(math.Round(lastTemp)),
		Updated:  time.Now(),
		Readings: readings,
	}
	f.mu.Unlock()

	log.Printf("[METAR] %s: Max=%d°F, Last=%d°F, Readings=%d",
		station.Code, int(math.Round(maxTemp)), int(math.Round(lastTemp)), readings)

	return nil
}

