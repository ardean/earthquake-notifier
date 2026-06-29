package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/ardean/earthquake-notifier/config"
)

const usgsQueryURL = "https://earthquake.usgs.gov/fdsnws/event/1/query"

type Event struct {
	ID        string
	Magnitude float64
	Place     string
	Time      time.Time
	Updated   time.Time
	DepthKm   float64
	Latitude  float64
	Longitude float64
	URL       string
	MagType   string
	Status    string
	Alert     string
}

type Client struct {
	httpClient *http.Client
	watch      config.WatchArea
	minMag     float64
}

func NewClient(watch config.WatchArea, minMag float64) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		watch:      watch,
		minMag:     minMag,
	}
}

type geoJSONResponse struct {
	Features []geoJSONFeature `json:"features"`
}

type geoJSONFeature struct {
	ID         string             `json:"id"`
	Properties geoJSONProperties  `json:"properties"`
	Geometry   geoJSONGeometry    `json:"geometry"`
}

type geoJSONProperties struct {
	Mag     *float64 `json:"mag"`
	Place   string   `json:"place"`
	Time    int64    `json:"time"`
	Updated int64    `json:"updated"`
	URL     string   `json:"url"`
	MagType string   `json:"magType"`
	Status  string   `json:"status"`
	Alert   string   `json:"alert"`
}

type geoJSONGeometry struct {
	Coordinates []float64 `json:"coordinates"`
}

func (c *Client) FetchSince(since time.Time) ([]Event, error) {
	params := url.Values{}
	params.Set("format", "geojson")
	params.Set("orderby", "time")
	params.Set("latitude", strconv.FormatFloat(c.watch.Latitude, 'f', -1, 64))
	params.Set("longitude", strconv.FormatFloat(c.watch.Longitude, 'f', -1, 64))
	params.Set("maxradiuskm", strconv.FormatFloat(c.watch.RadiusKm, 'f', -1, 64))
	params.Set("minmagnitude", strconv.FormatFloat(c.minMag, 'f', -1, 64))

	if since.IsZero() {
		params.Set("starttime", time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339))
	} else {
		params.Set("updatedafter", since.UTC().Format(time.RFC3339))
	}

	reqURL := usgsQueryURL + "?" + params.Encode()
	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch earthquakes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("USGS API returned %d: %s", resp.StatusCode, string(body))
	}

	var payload geoJSONResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	events := make([]Event, 0, len(payload.Features))
	for _, feature := range payload.Features {
		event, ok := parseFeature(feature)
		if !ok {
			continue
		}
		events = append(events, event)
	}

	return events, nil
}

func parseFeature(feature geoJSONFeature) (Event, bool) {
	if feature.ID == "" {
		return Event{}, false
	}
	if feature.Properties.Mag == nil {
		return Event{}, false
	}
	if len(feature.Geometry.Coordinates) < 2 {
		return Event{}, false
	}

	lon := feature.Geometry.Coordinates[0]
	lat := feature.Geometry.Coordinates[1]
	depth := 0.0
	if len(feature.Geometry.Coordinates) > 2 {
		depth = feature.Geometry.Coordinates[2]
	}

	return Event{
		ID:        feature.ID,
		Magnitude: *feature.Properties.Mag,
		Place:     feature.Properties.Place,
		Time:      time.UnixMilli(feature.Properties.Time).UTC(),
		Updated:   time.UnixMilli(feature.Properties.Updated).UTC(),
		DepthKm:   depth,
		Latitude:  lat,
		Longitude: lon,
		URL:       feature.Properties.URL,
		MagType:   feature.Properties.MagType,
		Status:    feature.Properties.Status,
		Alert:     feature.Properties.Alert,
	}, true
}
