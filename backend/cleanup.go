package backend

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func StartBackgroundCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		// Run immediate cleanup on startup
		runCleanupTask()

		for range ticker.C {
			runCleanupTask()
		}
	}()
}

func runCleanupTask() {
	// Avoid running cleanup during active downloads
	downloadingLock.RLock()
	downloading := isDownloading
	downloadingLock.RUnlock()

	if downloading {
		fmt.Println("[Cleanup] Active download in progress. Skipping cleanup task.")
		return
	}

	fmt.Println("[Cleanup] Running periodic cleanup task...")

	// 1. Clean temporary workspaces in OS temp dir
	tempDir := os.TempDir()
	files, err := os.ReadDir(tempDir)
	if err == nil {
		now := time.Now()
		for _, file := range files {
			if file.IsDir() && strings.HasPrefix(file.Name(), "spotune-") {
				dirPath := filepath.Join(tempDir, file.Name())
				if info, err := os.Stat(dirPath); err == nil {
					// Delete if older than 6 hours
					if now.Sub(info.ModTime()) > 6*time.Hour {
						fmt.Printf("[Cleanup] Removing expired workspace: %s\n", dirPath)
						os.RemoveAll(dirPath)
					}
				}
			}
		}
	}

	// 2. Clean cache entries in sync.Map
	now := time.Now()
	apiCache.Range(func(key, value interface{}) bool {
		cached, ok := value.(CacheItem)
		if ok && now.After(cached.Expiration) {
			apiCache.Delete(key)
		}
		return true
	})

	// 3. Clean old partial download artifacts in music directory
	musicDir := GetDefaultMusicPath()
	if items, err := os.ReadDir(musicDir); err == nil {
		for _, item := range items {
			if !item.IsDir() && (strings.HasSuffix(item.Name(), ".tmp") || strings.HasSuffix(item.Name(), ".bak")) {
				filePath := filepath.Join(musicDir, item.Name())
				if info, err := os.Stat(filePath); err == nil {
					// Delete if older than 6 hours
					if now.Sub(info.ModTime()) > 6*time.Hour {
						fmt.Printf("[Cleanup] Removing leftover temp file: %s\n", filePath)
						os.Remove(filePath)
					}
				}
			}
		}
	}

	fmt.Println("[Cleanup] Periodic cleanup completed.")
}
