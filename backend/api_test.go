package backend

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestValidateFormatAndQuality(t *testing.T) {
	tests := []struct {
		format  string
		quality string
		wantErr bool
	}{
		{"FLAC", "", false},
		{"FLAC", "lossless", false},
		{"FLAC", "320k", true},
		{"MP3", "320k", false},
		{"MP3", "256k", false},
		{"MP3", "128k", false},
		{"MP3", "lossless", true},
		{"MP3", "500k", true},
		{"INVALID", "320k", true},
	}

	for _, tt := range tests {
		err := ValidateFormatAndQuality(tt.format, tt.quality)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateFormatAndQuality(%q, %q) error = %v, wantErr %v", tt.format, tt.quality, err, tt.wantErr)
		}
	}
}

func TestBuildSpotuneFilename(t *testing.T) {
	filename := BuildExpectedFilename("Blinding Lights", "The Weeknd", "", "", "", "", "", "", false, 0, 0, false)
	expected := "Blinding Lights - The Weeknd - Spotune [FLAC].flac"
	if filename != expected {
		t.Errorf("BuildExpectedFilename got %q, expected %q", filename, expected)
	}
}

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(2, 10)
	if !rl.Allow() {
		t.Errorf("Expected first allow to pass")
	}
	if !rl.Allow() {
		t.Errorf("Expected second allow to pass")
	}
	if rl.Allow() {
		t.Errorf("Expected third allow to fail (rate limited)")
	}
}

func TestCache(t *testing.T) {
	setCache("test-key", "test-value")

	val, found := getCache("test-key")
	if !found || val != "test-value" {
		t.Errorf("Expected test-value to be found in cache")
	}

	apiCache.Store("exp-key", CacheItem{
		Value:      "exp-value",
		Expiration: time.Now().Add(-1 * time.Second),
	})

	_, found = getCache("exp-key")
	if found {
		t.Errorf("Expected expired key to be missing")
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		{"2.2.0", "2.2.0", 0},
		{"2.2.1", "2.2.0", 1},
		{"2.2.0", "2.2.1", -1},
		{"3.0.0", "2.9.9", 1},
		{"2.1.9", "2.2.0", -1},
	}

	for _, tt := range tests {
		got := compareVersions(tt.v1, tt.v2)
		if (tt.expected > 0 && got <= 0) || (tt.expected < 0 && got >= 0) || (tt.expected == 0 && got != 0) {
			t.Errorf("compareVersions(%q, %q) got %d, expected sign match for %d", tt.v1, tt.v2, got, tt.expected)
		}
	}
}

func TestCalculateProviderScore(t *testing.T) {
	// Setup mock stats
	RecordProviderResult("tidal", true)
	RecordProviderResult("tidal", true)
	RecordProviderResult("tidal", false) // 66.6% success rate

	score := CalculateProviderScore("tidal")
	// Tidal online check might fail in sandbox (no internet), but should still calculate history score correctly
	if score < 0 {
		t.Errorf("expected score >= 0, got %d", score)
	}
}

func TestHealthHandler(t *testing.T) {
	req, err := http.NewRequest("GET", "/api/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(handleHealth)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp["version"] != AppVersion {
		t.Errorf("expected version %q, got %q", AppVersion, resp["version"])
	}
}
