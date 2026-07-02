package backend

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var AppVersion = "7.1.9"

// JobState tracks download jobs for polling and streaming.
type JobState struct {
	JobID        string  `json:"job_id"`
	JobType      string  `json:"job_type"` // track, playlist
	Status       string  `json:"status"`   // queued, downloading, converting, zipping, complete, failed, cancelled
	Percent      float64 `json:"percent"`
	Error        string  `json:"error,omitempty"`
	Format       string  `json:"format"`
	Quality      string  `json:"quality"`
	ZipSizeBytes int64   `json:"zip_size_bytes,omitempty"`
	FilePath     string  `json:"file_path,omitempty"` // path on server for streaming
	Completed    int     `json:"completed_tracks"`
	Failed       int     `json:"failed_tracks"`
	Cancelled    int     `json:"cancelled_tracks"`
}

var (
	jobsMap         sync.Map // map[string]*JobState
	metadataCache   sync.Map // cache for Spotify metadata
	cacheDuration   = 5 * time.Minute
	metadataLimiter = NewRateLimiter(10, 0.5) // Max 10 requests, refills 1 token every 2 seconds
	downloadLimiter = NewRateLimiter(5, 0.2)  // Max 5 requests, refills 1 token every 5 seconds
	activeWorkspaces = sync.Map{} // map[string]string (taskId -> tempDir)
)

type RateLimiter struct {
	mu           sync.Mutex
	tokens       float64
	maxTokens    float64
	refillRate   float64
	lastRefilled time.Time
}

func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:       maxTokens,
		maxTokens:    maxTokens,
		refillRate:   refillRate,
		lastRefilled: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefilled).Seconds()
	rl.lastRefilled = now

	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}

	if rl.tokens >= 1.0 {
		rl.tokens -= 1.0
		return true
	}
	return false
}

type CacheItem struct {
	Value      interface{}
	Expiration time.Time
}

func setCache(key string, val interface{}) {
	metadataCache.Store(key, CacheItem{
		Value:      val,
		Expiration: time.Now().Add(cacheDuration),
	})
}

func getCache(key string) (interface{}, bool) {
	item, ok := metadataCache.Load(key)
	if !ok {
		return nil, false
	}
	cached := item.(CacheItem)
	if time.Now().After(cached.Expiration) {
		metadataCache.Delete(key)
		return nil, false
	}
	return cached.Value, true
}

func StartRESTServer() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "16860"
	}

	mux := http.NewServeMux()

	// Connect frontend routes
	mux.HandleFunc("/api/v1/health", handleHealth)
	mux.HandleFunc("/api/v1/metadata", handleMetadata)
	mux.HandleFunc("/api/v1/download/track", handleDownloadTrack)
	mux.HandleFunc("/api/v1/playlist/download", handleDownloadPlaylist)
	
	// Dynamic wildcards mapping
	mux.HandleFunc("/api/v1/queue/", handleQueueRoute)
	mux.HandleFunc("/api/v1/download/", handleDownloadRoute)
	mux.HandleFunc("/api/v1/playlist/", handlePlaylistRoute)

	// Admin / documentation endpoints
	mux.HandleFunc("/api/docs", handleSwaggerUI)
	mux.HandleFunc("/swagger.json", handleSwaggerJSON)

	handler := recoveryMiddleware(corsMiddleware(mux))

	go func() {
		fmt.Printf("[RESTServer] Listening on port %s...\n", port)
		if err := http.ListenAndServe(":"+port, handler); err != nil {
			fmt.Printf("[RESTServer] Error: %v\n", err)
		}
	}()
}

// Middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("[RESTServer] Panic recovered: %v\n", err)
				writeJSONError(w, http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func writeJSONResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeJSONSuccess(w http.ResponseWriter, data interface{}) {
	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"ok":   true,
		"data": data,
	})
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSONResponse(w, status, map[string]interface{}{
		"ok":      false,
		"message": message,
	})
}

// Handlers

func handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "online"
	registryLoaded := false
	providersInitialized := false

	reg, err := GetRegistryInfo()
	if err == nil && reg != nil && len(reg.Extensions) > 0 {
		registryLoaded = true
	}

	if IsProviderSetupCompleted() {
		providersInitialized = true
	}

	if !registryLoaded {
		status = "offline"
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"status":                status,
		"version":               AppVersion,
		"registry_loaded":       registryLoaded,
		"providers_initialized": providersInitialized,
	})
}

type MetadataRequest struct {
	URL string `json:"url"`
}

func handleMetadata(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !metadataLimiter.Allow() {
		writeJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}

	var req MetadataRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if !validateSpotifyURL(req.URL) {
		writeJSONError(w, http.StatusBadRequest, "Invalid Spotify URL")
		return
	}

	if cachedVal, found := getCache(req.URL); found {
		writeJSONSuccess(w, cachedVal)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	data, err := GetFilteredSpotifyData(ctx, req.URL, false, 0, ", ", nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to fetch metadata: %v", err))
		return
	}

	// Format metadata into format frontend expects
	var result interface{}
	if strings.Contains(req.URL, "/track/") {
		details := data.(map[string]interface{})
		artistsSlice := []string{}
		if rawArtists, ok := details["artists"].(string); ok {
			artistsSlice = strings.Split(rawArtists, ", ")
		}
		
		result = map[string]interface{}{
			"type": "track",
			"data": map[string]interface{}{
				"title":       getString(details, "name"),
				"artists":     artistsSlice,
				"cover_url":   getString(details, "cover_url"),
				"duration_ms": details["duration_ms"],
				"album":       getString(details, "album_name"),
			},
		}
	} else {
		// Playlist or album
		playlistDetails := data.(*PlaylistResponsePayload)
		result = map[string]interface{}{
			"type": "playlist",
			"data": map[string]interface{}{
				"name":         playlistDetails.PlaylistInfo.Owner.DisplayName,
				"owner":        playlistDetails.PlaylistInfo.Owner.Name,
				"cover_url":    playlistDetails.PlaylistInfo.Cover,
				"total_tracks": playlistDetails.PlaylistInfo.Tracks.Total,
			},
		}
	}

	setCache(req.URL, result)
	writeJSONSuccess(w, result)
}

type DownloadSubmitRequest struct {
	URL     string `json:"url"`
	Format  string `json:"format"`
	Quality string `json:"quality"`
}

func handleDownloadTrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !downloadLimiter.Allow() {
		writeJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}

	var req DownloadSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	spotifyID := extractSpotifyID(req.URL)
	if spotifyID == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid Spotify Track URL")
		return
	}

	if err := ValidateFormatAndQuality(req.Format, req.Quality); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	taskID := fmt.Sprintf("track-%s-%d", spotifyID, time.Now().UnixNano())
	job := &JobState{
		JobID:   taskID,
		JobType: "track",
		Status:  "queued",
		Format:  req.Format,
		Quality: req.Quality,
	}
	jobsMap.Store(taskID, job)

	appDir, _ := GetAppDir()
	outDir := filepath.Join(appDir, "downloads")
	os.MkdirAll(outDir, 0755)

	go func() {
		err := executeTrackDownloadTask(taskID, spotifyID, req.Format, req.Quality, outDir)
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
		}
	}()

	writeJSONSuccess(w, job)
}

func handleDownloadPlaylist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if !downloadLimiter.Allow() {
		writeJSONError(w, http.StatusTooManyRequests, "Rate limit exceeded")
		return
	}

	var req DownloadSubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.URL == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := ValidateFormatAndQuality(req.Format, req.Quality); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	taskID := fmt.Sprintf("playlist-%d", time.Now().UnixNano())
	job := &JobState{
		JobID:   taskID,
		JobType: "playlist",
		Status:  "queued",
		Format:  req.Format,
		Quality: req.Quality,
	}
	jobsMap.Store(taskID, job)

	appDir, _ := GetAppDir()
	outDir := filepath.Join(appDir, "downloads")
	os.MkdirAll(outDir, 0755)

	go func() {
		err := executePlaylistDownloadTask(taskID, req.URL, nil, req.Format, req.Quality, outDir)
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
		}
	}()

	writeJSONSuccess(w, job)
}

func handleQueueRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/queue/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing Job ID")
		return
	}

	jobID := parts[0]
	val, ok := jobsMap.Load(jobID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}
	job := val.(*JobState)

	if len(parts) > 1 && parts[1] == "pause" {
		PauseDownloads()
		job.Status = "paused"
		writeJSONSuccess(w, job)
		return
	}

	if len(parts) > 1 && parts[1] == "resume" {
		ResumeDownloads()
		job.Status = "downloading"
		writeJSONSuccess(w, job)
		return
	}

	// Update percentage from downloader queue
	if job.Status == "downloading" {
		job.Percent = getJobPercent(jobID)
	}

	writeJSONSuccess(w, job)
}

func handleDownloadRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/download/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing Job ID")
		return
	}

	jobID := parts[0]
	val, ok := jobsMap.Load(jobID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}
	job := val.(*JobState)

	if len(parts) > 1 && parts[1] == "cancel" {
		ForceStopActiveDownloads()
		job.Status = "cancelled"
		if job.FilePath != "" {
			os.Remove(job.FilePath)
		}
		writeJSONSuccess(w, job)
		return
	}

	// GET - Serve the completed file
	if r.Method == http.MethodGet {
		if job.Status != "complete" || job.FilePath == "" {
			writeJSONError(w, http.StatusBadRequest, "File not ready for download")
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(job.FilePath)))
		http.ServeFile(w, r, job.FilePath)
		return
	}
}

type PlaylistCancelRequest struct {
	Option string `json:"option"`
}

func handlePlaylistRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/playlist/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeJSONError(w, http.StatusBadRequest, "Missing Job ID")
		return
	}

	jobID := parts[0]
	val, ok := jobsMap.Load(jobID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Job not found")
		return
	}
	job := val.(*JobState)

	if len(parts) > 1 && parts[1] == "pause" {
		PauseDownloads()
		job.Status = "paused"
		writeJSONSuccess(w, job)
		return
	}

	if len(parts) > 1 && parts[1] == "resume" {
		ResumeDownloads()
		job.Status = "downloading"
		writeJSONSuccess(w, job)
		return
	}

	if len(parts) > 1 && parts[1] == "cancel" {
		var req PlaylistCancelRequest
		json.NewDecoder(r.Body).Decode(&req)

		ForceStopActiveDownloads()
		job.Status = "cancelled"

		if req.Option == "delete" {
			cleanupAllTaskWorkspaces()
			writeJSONSuccess(w, map[string]interface{}{
				"option":           "delete",
				"completed_tracks": job.Completed,
				"failed_tracks":    job.Failed,
				"cancelled_tracks": job.Cancelled,
			})
			return
		}

		if req.Option == "zip" {
			zipCompletedSessionFiles()
			cleanupAllTaskWorkspaces()
			writeJSONSuccess(w, map[string]interface{}{
				"option":           "zip",
				"completed_tracks": job.Completed,
				"failed_tracks":    job.Failed,
				"cancelled_tracks": job.Cancelled,
				"download_url":     fmt.Sprintf("/api/v1/playlist/%s/zip", jobID),
			})
			return
		}

		writeJSONSuccess(w, map[string]interface{}{
			"option":           "keep",
			"completed_tracks": job.Completed,
			"failed_tracks":    job.Failed,
			"cancelled_tracks": job.Cancelled,
		})
		return
	}

	if len(parts) > 1 && parts[1] == "zip" {
		if job.Status != "complete" || job.FilePath == "" {
			writeJSONError(w, http.StatusBadRequest, "ZIP not ready for download")
			return
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(job.FilePath)))
		http.ServeFile(w, r, job.FilePath)
		return
	}
}

// Internal Workers & Utilities

func executeTrackDownloadTask(taskID, spotifyID, format, quality, outputDir string) error {
	jobVal, _ := jobsMap.Load(taskID)
	job := jobVal.(*JobState)
	job.Status = "downloading"

	startTime := time.Now()
	metric := DownloadLogEntry{
		TaskID:  taskID,
		Format:  format,
		Quality: quality,
	}

	tempWorkspace, err := os.MkdirTemp("", "spotune-"+taskID)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		metric.Success = false
		metric.FailureReason = err.Error()
		logDownloadMetrics(metric)
		return err
	}
	defer os.RemoveAll(tempWorkspace)
	activeWorkspaces.Store(taskID, tempWorkspace)
	defer activeWorkspaces.Delete(taskID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	trackURL := "https://open.spotify.com/track/" + spotifyID
	metaData, err := GetFilteredSpotifyData(ctx, trackURL, false, 0, GetSeparator(), nil)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		metric.Success = false
		metric.FailureReason = err.Error()
		logDownloadMetrics(metric)
		return err
	}

	trackDetails, ok := metaData.(map[string]interface{})
	if !ok {
		job.Status = "failed"
		job.Error = "invalid metadata format"
		metric.Success = false
		metric.FailureReason = "invalid metadata"
		logDownloadMetrics(metric)
		return fmt.Errorf("invalid metadata format")
	}

	trackName := getString(trackDetails, "name")
	artistName := getString(trackDetails, "artists")
	albumName := ""
	if albumMap, ok := trackDetails["album"].(map[string]interface{}); ok {
		albumName = getString(albumMap, "name")
	}

	metric.TrackName = trackName
	metric.ArtistName = artistName

	service, err := SelectBestProvider()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		metric.Success = false
		metric.FailureReason = err.Error()
		logDownloadMetrics(metric)
		return err
	}
	metric.Provider = service

	downloadFormat := "LOSSLESS"
	downloader := NewTidalDownloader(GetCustomTidalAPISetting())
	if service == "qobuz" {
		downloader.SetCustomAPIURL(GetQobuzCommunityHealthURL())
	}

	AddToQueue(taskID, trackName, artistName, albumName, spotifyID)
	StartDownloadItem(taskID)

	var downloadPath string
	dlStart := time.Now()
	retries := 0

	for attempt := 0; attempt < 3; attempt++ {
		retries = attempt
		if service == "amazon" {
			amazonDownloader := NewAmazonDownloader()
			downloadPath, err = amazonDownloader.DownloadBySpotifyID(spotifyID, tempWorkspace, downloadFormat, "raw", "", "", false, 0, trackName, artistName, albumName, "", "", "", 0, 0, 0, true, 0, "", "", "", GetSeparator(), "", trackURL, false, false, false)
		} else if service == "qobuz" {
			qobuzDownloader := NewQobuzDownloader()
			isrc := ResolveTrackISRC(spotifyID)
			downloadPath, err = qobuzDownloader.DownloadTrackWithISRC(isrc, tempWorkspace, "27", "raw", false, 0, trackName, artistName, albumName, "", "", false, "", true, 0, 0, 0, 0, "", "", "", GetSeparator(), trackURL, true, false, false, false)
		} else {
			downloadPath, err = downloader.Download(spotifyID, tempWorkspace, downloadFormat, "raw", false, 0, trackName, artistName, albumName, "", "", false, "", true, 0, 0, 0, 0, "", "", "", GetSeparator(), "", trackURL, true, false, false, false)
		}
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}

	metric.Retries = retries
	metric.DownloadDurationMs = time.Since(dlStart).Milliseconds()

	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		FailDownloadItem(taskID, err.Error())
		RecordProviderResult(service, false)
		metric.Success = false
		metric.FailureReason = err.Error()
		logDownloadMetrics(metric)
		return err
	}
	RecordProviderResult(service, true)

	if _, err := os.Stat(downloadPath); err != nil {
		job.Status = "failed"
		job.Error = "Downloaded file not found"
		FailDownloadItem(taskID, "Downloaded file not found")
		metric.Success = false
		metric.FailureReason = "downloaded file not found"
		logDownloadMetrics(metric)
		return fmt.Errorf("downloaded file not found: %s", downloadPath)
	}

	job.Status = "converting"
	job.Percent = 95

	convStart := time.Now()
	finalExt := "." + strings.ToLower(format)
	if format == "aac" {
		finalExt = ".m4a"
	}
	qualityLabel := quality
	if format == "flac" || format == "wav" {
		qualityLabel = "FLAC"
		if format == "wav" {
			qualityLabel = "WAV"
		}
	}

	finalFilename := BuildExpectedFilename(trackName, artistName, albumName, "", "", "", "", "", false, 0, 0, false)
	finalFilename = strings.ReplaceAll(finalFilename, "[FLAC]", "["+qualityLabel+"]")
	finalFilename = strings.TrimSuffix(finalFilename, ".flac") + finalExt
	finalPath := filepath.Join(outputDir, finalFilename)

	if strings.ToLower(format) == "flac" {
		err = copyFile(downloadPath, finalPath)
	} else {
		convReq := ConvertAudioRequest{
			InputFiles:   []string{downloadPath},
			OutputFormat: format,
			Bitrate:      quality,
		}
		convResults, convErr := ConvertAudio(convReq)
		if convErr != nil || len(convResults) == 0 || !convResults[0].Success {
			errMsg := "Conversion failed"
			if len(convResults) > 0 {
				errMsg = convResults[0].Error
			}
			job.Status = "failed"
			job.Error = errMsg
			FailDownloadItem(taskID, errMsg)
			metric.Success = false
			metric.FailureReason = errMsg
			logDownloadMetrics(metric)
			return fmt.Errorf("conversion failed: %s", errMsg)
		}
		err = copyFile(convResults[0].OutputFile, finalPath)
	}

	metric.ConvertDurationMs = time.Since(convStart).Milliseconds()

	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		FailDownloadItem(taskID, err.Error())
		metric.Success = false
		metric.FailureReason = err.Error()
		logDownloadMetrics(metric)
		return err
	}

	CompleteDownloadItem(taskID, finalPath, getFileSizeMB(finalPath))
	
	job.Status = "complete"
	job.Percent = 100
	job.FilePath = finalPath

	metric.Success = true
	logDownloadMetrics(metric)
	return nil
}

func executePlaylistDownloadTask(taskID, playlistURL string, selectedTracks []string, format, quality, outputDir string) error {
	jobVal, _ := jobsMap.Load(taskID)
	job := jobVal.(*JobState)
	job.Status = "downloading"

	tempWorkspace, err := os.MkdirTemp("", "spotune-"+taskID)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return err
	}
	defer os.RemoveAll(tempWorkspace)
	activeWorkspaces.Store(taskID, tempWorkspace)
	defer activeWorkspaces.Delete(taskID)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	metaData, err := GetFilteredSpotifyData(ctx, playlistURL, false, 0, GetSeparator(), nil)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return err
	}

	playlistDetails, ok := metaData.(*PlaylistResponsePayload)
	if !ok {
		job.Status = "failed"
		job.Error = "invalid playlist metadata"
		return fmt.Errorf("invalid playlist metadata format")
	}

	playlistName := playlistDetails.PlaylistInfo.Owner.DisplayName
	if playlistName == "" {
		playlistName = "Playlist"
	}

	var completedPaths []string
	var completedPathsMu sync.Mutex

	// Download playlist cover image (cover.jpg) if available
	if playlistDetails.PlaylistInfo.Cover != "" {
		coverDestTemp := filepath.Join(tempWorkspace, "cover.jpg")
		coverDestOut := filepath.Join(outputDir, "cover.jpg")

		client := &http.Client{Timeout: 10 * time.Second}
		if resp, err := client.Get(playlistDetails.PlaylistInfo.Cover); err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			out, createErr := os.Create(coverDestTemp)
			if createErr == nil {
				io.Copy(out, resp.Body)
				out.Close()
				copyFile(coverDestTemp, coverDestOut)

				completedPathsMu.Lock()
				completedPaths = append(completedPaths, coverDestTemp)
				completedPathsMu.Unlock()
			}
		}
	}

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	totalTracksToDownload := len(playlistDetails.TrackList)
	for _, track := range playlistDetails.TrackList {
		wg.Add(1)
		go func(trackItem AlbumTrackMetadata) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			trackTaskID := fmt.Sprintf("playlist-track-%s-%d", trackItem.SpotifyID, time.Now().UnixNano())
			trackTempDir := filepath.Join(tempWorkspace, trackItem.SpotifyID)
			os.MkdirAll(trackTempDir, 0755)

			service, selErr := SelectBestProvider()
			if selErr != nil {
				job.Failed++
				return
			}

			AddToQueue(trackTaskID, trackItem.Name, trackItem.Artists, trackItem.AlbumName, trackItem.SpotifyID)
			StartDownloadItem(trackTaskID)

			var downloadPath string
			downloadFormat := "LOSSLESS"
			var dlErr error

			for attempt := 0; attempt < 3; attempt++ {
				if service == "amazon" {
					amazonDownloader := NewAmazonDownloader()
					downloadPath, dlErr = amazonDownloader.DownloadBySpotifyID(trackItem.SpotifyID, trackTempDir, downloadFormat, "raw", "", "", false, 0, trackItem.Name, trackItem.Artists, trackItem.AlbumName, "", "", "", 0, 0, 0, true, 0, "", "", "", GetSeparator(), "", "", false, false, false)
				} else if service == "qobuz" {
					qobuzDownloader := NewQobuzDownloader()
					isrc := ResolveTrackISRC(trackItem.SpotifyID)
					downloadPath, dlErr = qobuzDownloader.DownloadTrackWithISRC(isrc, trackTempDir, "27", "raw", false, 0, trackItem.Name, trackItem.Artists, trackItem.AlbumName, "", "", false, "", true, 0, 0, 0, 0, "", "", "", GetSeparator(), "", true, false, false, false)
				} else {
					downloader := NewTidalDownloader(GetCustomTidalAPISetting())
					downloadPath, dlErr = downloader.Download(trackItem.SpotifyID, trackTempDir, downloadFormat, "raw", false, 0, trackItem.Name, trackItem.Artists, trackItem.AlbumName, "", "", false, "", true, 0, 0, 0, 0, "", "", "", GetSeparator(), "", "", true, false, false, false)
				}
				if dlErr == nil {
					break
				}
				time.Sleep(1 * time.Second)
			}

			if dlErr != nil {
				FailDownloadItem(trackTaskID, dlErr.Error())
				RecordProviderResult(service, false)
				job.Failed++
				return
			}
			RecordProviderResult(service, true)

			finalExt := "." + strings.ToLower(format)
			if format == "aac" {
				finalExt = ".m4a"
			}
			qualityLabel := quality
			if format == "flac" || format == "wav" {
				qualityLabel = "FLAC"
				if format == "wav" {
					qualityLabel = "WAV"
				}
			}

			finalFilename := BuildExpectedFilename(trackItem.Name, trackItem.Artists, trackItem.AlbumName, "", "", "", "", "", false, 0, 0, false)
			finalFilename = strings.ReplaceAll(finalFilename, "[FLAC]", "["+qualityLabel+"]")
			finalFilename = strings.TrimSuffix(finalFilename, ".flac") + finalExt
			finalTrackPath := filepath.Join(trackTempDir, finalFilename)

			if strings.ToLower(format) == "flac" {
				dlErr = copyFile(downloadPath, finalTrackPath)
			} else {
				convReq := ConvertAudioRequest{
					InputFiles:   []string{downloadPath},
					OutputFormat: format,
					Bitrate:      quality,
				}
				convResults, convErr := ConvertAudio(convReq)
				if convErr == nil && len(convResults) > 0 && convResults[0].Success {
					dlErr = copyFile(convResults[0].OutputFile, finalTrackPath)
				} else {
					dlErr = fmt.Errorf("conversion failed")
				}
			}

			if dlErr != nil {
				FailDownloadItem(trackTaskID, dlErr.Error())
				job.Failed++
				return
			}

			CompleteDownloadItem(trackTaskID, finalTrackPath, getFileSizeMB(finalTrackPath))
			job.Completed++

			completedPathsMu.Lock()
			completedPaths = append(completedPaths, finalTrackPath)
			completedPathsMu.Unlock()

			// Update total percentage dynamically
			job.Percent = float64(job.Completed) / float64(totalTracksToDownload) * 100.0
		}(track)
	}

	wg.Wait()

	if IsDownloadForceStopRequested() {
		job.Status = "cancelled"
		job.Cancelled = totalTracksToDownload - job.Completed - job.Failed
		return fmt.Errorf("playlist download cancelled")
	}

	if len(completedPaths) > 0 {
		job.Status = "zipping"
		zipFilename := fmt.Sprintf("%s - Spotune.zip", SanitizeFilename(playlistName))
		zipPath := filepath.Join(outputDir, zipFilename)

		err = createZipFromFiles(zipPath, completedPaths)
		if err != nil {
			job.Status = "failed"
			job.Error = err.Error()
			return err
		}

		job.Status = "complete"
		job.Percent = 100
		job.FilePath = zipPath
		if info, err := os.Stat(zipPath); err == nil {
			job.ZipSizeBytes = info.Size()
		}
	} else {
		job.Status = "failed"
		job.Error = "no tracks completed"
	}

	return nil
}

func cleanupAllTaskWorkspaces() {
	activeWorkspaces.Range(func(key, value interface{}) bool {
		dir := value.(string)
		os.RemoveAll(dir)
		activeWorkspaces.Delete(key)
		return true
	})
}

func zipCompletedSessionFiles() {
	queue := GetDownloadQueue()
	var completed []string
	for _, item := range queue.Queue {
		if item.Status == StatusCompleted && item.FilePath != "" {
			completed = append(completed, item.FilePath)
		}
	}

	if len(completed) > 0 {
		appDir, _ := GetAppDir()
		zipPath := filepath.Join(appDir, "downloads", "Cancelled Session - Spotune.zip")
		createZipFromFiles(zipPath, completed)
	}
}

func validateSpotifyURL(urlStr string) bool {
	re := regexp.MustCompile(`https?://open\.spotify\.com/(track|playlist|album)/[a-zA-Z0-9]+`)
	return re.MatchString(urlStr)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func createZipFromFiles(zipPath string, files []string) error {
	archive, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}

		w, err := zipWriter.Create(filepath.Base(file))
		if err != nil {
			f.Close()
			return err
		}

		_, err = io.Copy(w, f)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func getFileSizeMB(path string) float64 {
	if info, err := os.Stat(path); err == nil {
		return float64(info.Size()) / (1024 * 1024)
	}
	return 0
}

func ValidateFormatAndQuality(format string, quality string) error {
	format = strings.ToUpper(strings.TrimSpace(format))
	quality = strings.TrimSpace(quality)

	validFormats := map[string]bool{
		"MP3":  true,
		"FLAC": true,
		"WAV":  true,
		"M4A":  true,
		"AAC":  true,
		"OGG":  true,
		"OPUS": true,
	}

	if !validFormats[format] {
		return fmt.Errorf("invalid format: %s", format)
	}

	isLossless := format == "FLAC" || format == "WAV"
	if isLossless {
		if quality != "" && !strings.EqualFold(quality, "lossless") && !strings.EqualFold(quality, "hi_res") {
			return fmt.Errorf("lossless formats do not support custom bitrates")
		}
	} else {
		validBitrates := map[string]bool{
			"128k":    true, "128kbps": true, "128": true,
			"192k":    true, "192kbps": true, "192": true,
			"256k":    true, "256kbps": true, "256": true,
			"320k":    true, "320kbps": true, "320": true,
		}
		if quality == "" {
			return nil
		}
		if !validBitrates[strings.ToLower(quality)] {
			return fmt.Errorf("invalid bitrate for lossy format: %s", quality)
		}
	}
	return nil
}

func GetSeparator() string {
	return ", "
}

type DownloadLogEntry struct {
	Timestamp          string `json:"timestamp"`
	TaskID             string `json:"task_id"`
	TrackName          string `json:"track_name"`
	ArtistName         string `json:"artist_name"`
	Format             string `json:"format"`
	Quality            string `json:"quality"`
	Provider           string `json:"provider"`
	DownloadDurationMs int64  `json:"download_duration_ms"`
	ConvertDurationMs  int64  `json:"convert_duration_ms"`
	Retries            int    `json:"retries"`
	Success            bool   `json:"success"`
	FailureReason      string `json:"failure_reason,omitempty"`
}

func logDownloadMetrics(entry DownloadLogEntry) {
	entry.Timestamp = time.Now().Format(time.RFC3339)
	data, err := json.Marshal(entry)
	if err == nil {
		fmt.Printf("[METRICS] %s\n", string(data))
		appDir, err := EnsureAppDir()
		if err == nil {
			logPath := filepath.Join(appDir, "metrics.log")
			f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err == nil {
				f.Write(append(data, '\n'))
				f.Close()
			}
		}
	}
}

// Swagger & OpenAPI handlers

func handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	swagger := `{
  "openapi": "3.0.0",
  "info": {
    "title": "Spotune REST API",
    "description": "API endpoints for Spotune Spotify Downloader backend service.",
    "version": "` + AppVersion + `"
  },
  "paths": {
    "/api/v1/health": {
      "get": {
        "responses": {
          "200": { "description": "API status details" }
        }
      }
    }
  }
}`
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(swagger))
}

func handleSwaggerUI(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
  <title>Spotune API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/swagger.json',
        dom_id: '#swagger-ui'
      });
    };
  </script>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(html))
}
