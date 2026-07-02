/**
 * Spotune Premium Javascript Logic
 */

// Auto-detect host on startup
function initDefaultHost() {
    const hostSelect = document.getElementById('apiHostSelect');
    const isLocal = window.location.hostname === 'localhost' || 
                    window.location.hostname === '127.0.0.1' || 
                    window.location.hostname.includes('wails') ||
                    window.location.protocol === 'file:';
    if (hostSelect) {
        hostSelect.value = isLocal ? 'local' : 'render';
    }
}

// Retrieve active API base URL based on host selector
const getApiBase = () => {
    const hostSelect = document.getElementById('apiHostSelect');
    if (hostSelect && hostSelect.value === 'local') {
        // If we are served by the local backend, use the same origin to be safe
        if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
            return window.location.origin + '/api/v1';
        }
        return 'http://localhost:16860/api/v1';
    }
    return 'https://spotune.onrender.com/api/v1';
};

// State variables
let activeJobId = null;
let pollingInterval = null;
let currentMetadata = null;

// Fallback vector icon SVG template (when Fetch Icons is off for fast loading)
const getFallbackIconSVG = (type) => {
    if (type === 'playlist') {
        return `
            <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M9 18V5l12-2v13"></path>
                <circle cx="6" cy="18" r="3"></circle>
                <circle cx="18" cy="16" r="3"></circle>
            </svg>
        `;
    }
    return `
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"></circle>
            <circle cx="12" cy="12" r="3"></circle>
        </svg>
    `;
};

// Check API health status
async function checkHealth() {
    const badge = document.getElementById('healthBadge');
    const label = document.getElementById('healthLabel');
    const apiBase = getApiBase();

    try {
        const res = await fetch(`${apiBase}/health`);
        const data = await res.json();
        if (data.status === 'online') {
            badge.className = 'health-badge status-online';
            label.innerText = 'Online';
        } else {
            badge.className = 'health-badge status-offline';
            label.innerText = 'Offline';
        }
    } catch (err) {
        badge.className = 'health-badge status-offline';
        label.innerText = 'Offline';
    }
}

// Format milliseconds to M:SS duration
function formatDuration(ms) {
    if (!ms) return '0:00';
    const totalSecs = Math.floor(ms / 1000);
    const mins = Math.floor(totalSecs / 60);
    const secs = totalSecs % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

// ── Event Handlers ───────────────────────────────────────────────────────────

// Toggle Format selectivity
document.getElementById('formatSelect').addEventListener('change', (e) => {
    const fmt = e.target.value;
    const qSelect = document.getElementById('qualitySelect');
    if (fmt === 'flac' || fmt === 'wav') {
        qSelect.disabled = true;
    } else {
        qSelect.disabled = false;
    }
});

// Fetch Metadata
document.getElementById('btnFetch').addEventListener('click', async () => {
    const url = document.getElementById('urlInput').value.trim();
    if (!url) return alert('Please enter a Spotify URL.');

    const btnFetch = document.getElementById('btnFetch');
    const emptyState = document.getElementById('emptyState');
    const loadingState = document.getElementById('loadingState');
    const metaLayout = document.getElementById('metadataLayout');
    const progressState = document.getElementById('progressState');
    const successState = document.getElementById('successState');

    // Reset layout visibility
    emptyState.style.display = 'none';
    metaLayout.style.display = 'none';
    progressState.style.display = 'none';
    successState.style.display = 'none';
    loadingState.style.display = 'flex';
    btnFetch.disabled = true;

    const apiBase = getApiBase();
    try {
        const res = await fetch(`${apiBase}/metadata`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        
        const body = await res.json();
        loadingState.style.display = 'none';

        if (!body.ok) {
            alert(body.message || 'Failed to retrieve metadata.');
            emptyState.style.display = 'flex';
            return;
        }

        const payload = body.data;
        currentMetadata = { url, type: payload.type, data: payload.data };

        // Render fields
        const typeBadge = document.getElementById('itemTypeBadge');
        const itemTitle = document.getElementById('itemTitle');
        const itemArtist = document.getElementById('itemArtist');
        const artContainer = document.getElementById('artContainer');
        const playlistBox = document.getElementById('playlistTracksBox');

        typeBadge.innerText = payload.type.toUpperCase();
        
        if (payload.type === 'track') {
            itemTitle.innerText = payload.data.title;
            itemArtist.innerText = payload.data.artists.join(', ');
            playlistBox.style.display = 'none';
        } else {
            itemTitle.innerText = payload.data.name;
            itemArtist.innerText = `Created by ${payload.data.owner || 'Spotify Creator'} • ${payload.data.total_tracks} tracks`;
            
            // Build and display track list if available
            playlistBox.innerHTML = '';
            if (payload.data.tracks && payload.data.tracks.length > 0) {
                payload.data.tracks.forEach((track, index) => {
                    const row = document.createElement('div');
                    row.className = 'track-row';
                    
                    row.innerHTML = `
                        <span class="track-index">${index + 1}</span>
                        <div class="track-info">
                            <span class="track-name">${track.name}</span>
                            <span class="track-artist-small">${track.artists}</span>
                        </div>
                        <span class="track-duration">${formatDuration(track.duration_ms)}</span>
                    `;
                    playlistBox.appendChild(row);
                });
                playlistBox.style.display = 'block';
            } else {
                playlistBox.style.display = 'none';
            }
        }

        // Handle cover art rendering depending on the "Fetch Icons" toggle state
        const fetchIconsOn = document.getElementById('iconFetchToggle').checked;
        if (fetchIconsOn && payload.data.cover_url) {
            artContainer.innerHTML = `<img src="${payload.data.cover_url}" alt="Cover art" style="width:100%; height:100%; border-radius:12px; object-fit:cover;">`;
        } else {
            // Render fast SVG fallback icon
            artContainer.innerHTML = getFallbackIconSVG(payload.type);
        }

        metaLayout.style.display = 'grid';

    } catch (err) {
        loadingState.style.display = 'none';
        emptyState.style.display = 'flex';
        alert('Error connecting to the Spotune metadata server.');
    } finally {
        btnFetch.disabled = false;
    }
});

// Submit Download Job
document.getElementById('btnDownload').addEventListener('click', async () => {
    if (!currentMetadata) return;

    const format = document.getElementById('formatSelect').value;
    const quality = document.getElementById('qualitySelect').value;
    const endpoint = currentMetadata.type === 'track' ? '/download/track' : '/playlist/download';
    const apiBase = getApiBase();

    document.getElementById('metadataLayout').style.display = 'none';
    document.getElementById('progressState').style.display = 'block';

    try {
        const res = await fetch(`${apiBase}${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: currentMetadata.url,
                format,
                quality
            })
        });
        const body = await res.json();
        
        if (body.ok) {
            startProgressPolling(body.data.job_id);
        } else {
            alert(body.message || 'Job submission rejected.');
            resetUI();
        }
    } catch (err) {
        alert('Failed to submit download job to the backend.');
        resetUI();
    }
});

// Start Dynamic Progress Polling
function startProgressPolling(jobId) {
    activeJobId = jobId;
    if (pollingInterval) clearInterval(pollingInterval);

    const titleEl = document.getElementById('progressTitle');
    const percentEl = document.getElementById('progressPercent');
    const fillEl = document.getElementById('progressBarFill');
    const statusEl = document.getElementById('progressStatus');
    const apiBase = getApiBase();

    pollingInterval = setInterval(async () => {
        try {
            const res = await fetch(`${apiBase}/queue/${activeJobId}`);
            const body = await res.json();
            if (!body.ok) return;

            const job = body.data;
            const pct = job.percent || 0;

            percentEl.innerText = `${Math.round(pct)}%`;
            fillEl.style.width = `${pct}%`;

            if (job.status === 'downloading') {
                titleEl.innerText = 'Downloading Tracks';
                statusEl.innerText = `Fetching streams... ${Math.round(pct)}%`;
            } else if (job.status === 'converting') {
                titleEl.innerText = 'Processing Transcode';
                statusEl.innerText = 'FFmpeg encoding audio layers...';
            } else if (job.status === 'zipping') {
                titleEl.innerText = 'Packaging Archive';
                statusEl.innerText = 'Zipping playlist contents...';
            } else if (job.status === 'complete') {
                clearInterval(pollingInterval);
                showSuccess(job);
            } else if (job.status === 'failed') {
                clearInterval(pollingInterval);
                alert(`Job failed: ${job.error || 'Unknown error'}`);
                resetUI();
            } else if (job.status === 'cancelled') {
                clearInterval(pollingInterval);
                resetUI();
            }
        } catch (err) {
            // Ignore temporary network connection glitches
        }
    }, 2000);
}

// Pause/Resume Downloader
document.getElementById('btnPauseResume').addEventListener('click', async () => {
    if (!activeJobId) return;
    const btn = document.getElementById('btnPauseResume');
    const apiBase = getApiBase();
    const isPlaylist = currentMetadata.type === 'playlist';

    if (btn.innerText.includes('Pause')) {
        const path = isPlaylist ? `/playlist/${activeJobId}/pause` : `/queue/${activeJobId}/pause`;
        await fetch(`${apiBase}${path}`, { method: 'POST' });
        btn.innerText = '▶ Resume';
        document.getElementById('progressStatus').innerText = 'Operation Paused';
        if (pollingInterval) clearInterval(pollingInterval);
    } else {
        const path = isPlaylist ? `/playlist/${activeJobId}/resume` : `/queue/${activeJobId}/resume`;
        await fetch(`${apiBase}${path}`, { method: 'POST' });
        btn.innerText = '⏸ Pause';
        startProgressPolling(activeJobId);
    }
});

// Cancel Downloader
document.getElementById('btnCancel').addEventListener('click', async () => {
    if (!activeJobId) return;
    const apiBase = getApiBase();
    const isPlaylist = currentMetadata.type === 'playlist';

    if (pollingInterval) clearInterval(pollingInterval);

    if (!isPlaylist) {
        // Track cancel
        await fetch(`${apiBase}/download/${activeJobId}/cancel`, { method: 'POST' });
    } else {
        // Playlist cancel
        await fetch(`${apiBase}/playlist/${activeJobId}/cancel`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ option: 'delete' })
        });
    }
    resetUI();
});

// Show Success screen
function showSuccess(job) {
    document.getElementById('progressState').style.display = 'none';
    const successState = document.getElementById('successState');
    const downloadLink = document.getElementById('downloadLink');
    const apiBase = getApiBase();

    // Map URL to direct file download
    downloadLink.href = job.job_type === 'playlist' 
        ? `${apiBase}/playlist/${job.job_id}/zip` 
        : `${apiBase}/download/${job.job_id}`;

    successState.style.display = 'block';
}

// Reset UI
function resetUI() {
    if (pollingInterval) clearInterval(pollingInterval);
    activeJobId = null;
    currentMetadata = null;
    document.getElementById('urlInput').value = '';
    document.getElementById('metadataLayout').style.display = 'none';
    document.getElementById('progressState').style.display = 'none';
    document.getElementById('successState').style.display = 'none';
    document.getElementById('emptyState').style.display = 'flex';
}

document.getElementById('btnReset').addEventListener('click', resetUI);

// ── Initializations ──────────────────────────────────────────────────────────

// Initialize default host based on location
initDefaultHost();

// Start health polling
checkHealth();
setInterval(checkHealth, 8000);
// Trigger check health when host selection changes
document.getElementById('apiHostSelect').addEventListener('change', checkHealth);
