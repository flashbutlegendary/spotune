/**
 * Spotune Production Frontend Logic
 */

const getApiBase = () => {
    if (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1') {
        return 'http://localhost:16860/api/v1';
    }
    if (window.location.origin.includes('github.io') || window.location.origin.includes('pages.dev')) {
        // Expose helper to override api base URL from query parameters or localStorage if needed
        const urlParams = new URLSearchParams(window.location.search);
        if (urlParams.has('api')) {
            return urlParams.get('api');
        }
        const stored = localStorage.getItem('SPOTUNE_API_BASE');
        if (stored) return stored;
    }
    return window.location.origin.endsWith(':16860') ? window.location.origin + '/api/v1' : 'https://spotune.onrender.com/api/v1';
};

let activeCancelJobId = null;
let activePollingJobId = null;
let activePollingInterval = null;
let currentMetadata = null;
let metadataStartTime = 0;

// ── 1. 10 Premium Hero Combinations ──────────────────────────────────────────
const HERO_COMBINATIONS = [
    {
        title: "Your Music, <span class='gradient-text'>Unchained</span>",
        subtitle: "Convert Spotify track and playlist links to pristine, high-fidelity audio files instantly."
    },
    {
        title: "Pristine Sound. <span class='gradient-text'>Zero Constraints</span>",
        subtitle: "Lossless conversion from link to local. Download tracks and playlists with cover art intact."
    },
    {
        title: "Music Belongs to <span class='gradient-text'>Your Ears</span>",
        subtitle: "A premium offline listening experience without compromise. Paste Spotify links, get FLAC & MP3."
    },
    {
        title: "Audiophile Quality. <span class='gradient-text'>Effortless Delivery</span>",
        subtitle: "Direct Spotify metadata extraction. Download tracks with original tags, label credits, and lyrics."
    },
    {
        title: "Free Your <span class='gradient-text'>Playlists</span>",
        subtitle: "Bulk playlist downloads compressed instantly. Pause, resume, and cancel tracks intelligently."
    },
    {
        title: "Lossless Audio. <span class='gradient-text'>Unbounded Freedom</span>",
        subtitle: "Experience high-fidelity FLAC audio files packed with original metadata, ready for offline playback."
    },
    {
        title: "The Sound You <span class='gradient-text'>Deserve</span>",
        subtitle: "Extract pure album metadata, composers, and high-res cover art. Clean downloading on demand."
    },
    {
        title: "Offline Sync, <span class='gradient-text'>Perfected</span>",
        subtitle: "Clean, responsive, smart downloader providing seamless formatting without any technical bloat."
    },
    {
        title: "Uncompromising <span class='gradient-text'>Fidelity</span>",
        subtitle: "Download entire music collections in high-speed, fully tagged WAV, FLAC, and MP3 structures."
    },
    {
        title: "Playlists Without <span class='gradient-text'>Boundaries</span>",
        subtitle: "Convert track listings to audio formats effortlessly. Download individually or batch in ZIP containers."
    }
];

// Apply random combination on page load
function applyRandomHero() {
    const idx = Math.floor(Math.random() * HERO_COMBINATIONS.length);
    const hero = HERO_COMBINATIONS[idx];
    document.getElementById('heroTitle').innerHTML = hero.title;
    document.getElementById('heroSubtitle').innerText = hero.subtitle;
}

// ── 2. 100 Unique Loading Messages ───────────────────────────────────────────
const LOADING_MESSAGES = [
    "Resolving tracks...", "Fetching artwork...", "Writing metadata...", "Transcoding formats...",
    "Querying Deezer database...", "Syncing Tidal streams...", "Initializing spotDL...", "Extracting ISRC...",
    "Embedding ID3 tags...", "Compressing layout...", "Caching provider entries...", "Filtering bad bitrates...",
    "Fetching composers...", "Scraping embed page...", "Searching YouTube Music...", "Injecting artwork blocks...",
    "Allocating workspace...", "Pruning stale paths...", "Mapping playlist collection...", "Parsing tracks metadata...",
    "Checking provider health...", "Tuning conversion pipeline...", "Verifying track availability...", "Resolving artists list...",
    "Packaging FLAC container...", "Aligning stereo channels...", "Buffering audio streams...", "Writing Vorbis comments...",
    "Validating output qualities...", "Connecting health monitor...", "Calculating health index...", "Updating scoring priorities...",
    "Extracting release dates...", "Matching record labels...", "Indexing playlist contents...", "Pinging Tidal API...",
    "Checking Deezer ARL token...", "Extracting SoundCloud streams...", "Re-routing network requests...", "Optimizing worker threads...",
    "Acquiring worker semaphore...", "Transcribing album titles...", "Extracting track indexes...", "Loading lyrics template...",
    "Parsing copyright details...", "Processing batch queue...", "Checking storage boundaries...", "Cleaning temporary folders...",
    "Sanitizing final filenames...", "Verifying file existence...", "Streaming archive chunks...", "Compressing zip targets...",
    "Initializing smart failover...", "Re-routing failed requests...", "Checking network thresholds...", "Updating job statistics...",
    "Initializing database session...", "Writing JSON response...", "Validating Spotify patterns...", "Extracting resource IDs...",
    "Extracting metadata fields...", "Injecting composers credits...", "Parsing track genres...", "Embedding genre attributes...",
    "Validating format combination...", "Filtering lossless bitrates...", "Analyzing track duration...", "Fetching original album art...",
    "Downloading high-res thumbnails...", "Setting mutagen parameters...", "Saving EasyID3 frames...", "Saving FLAC pictures...",
    "Converting audio samples...", "Invoking FFmpeg transcoder...", "Aligning audio bitrates...", "Checking FFmpeg binaries...",
    "Tuning worker allocations...", "Acquiring lock tokens...", "Applying rate limits...", "Tracking consumer requests...",
    "Configuring JSON formats...", "Configuring console outputs...", "Correlating request IDs...", "Injecting trace headers...",
    "Refreshing provider registry...", "Pinging registry URL...", "Updating extension status...", "Parsing registry structure...",
    "Sorting degraded providers...", "Calculating latency index...", "Weighting recency parameters...", "Matching ISRC hashes...",
    "Fetching Tidal session...", "Streaming audio file...", "Resolving direct streams...", "Resolving track details..."
];

function getRandomLoadingMessage() {
    const idx = Math.floor(Math.random() * LOADING_MESSAGES.length);
    return LOADING_MESSAGES[idx];
}

// ── 3. 50+ Unique Dismiss Button Texts ────────────────────────────────────────
const DISMISS_BUTTON_TEXTS = [
    "Okay, cool", "Awesome", "Got it", "Maybe later", "Close",
    "Sure thing", "Understood", "No thanks", "Will do!", "Perfect",
    "Sweet", "Nice!", "Sounds good", "I'm good", "Dismiss",
    "Done", "Alright", "Yep", "Roger that", "Cool",
    "Go back", "Continue", "Skip", "Next time", "Not now",
    "Great", "Thanks!", "Gotcha", "Fine", "Okay",
    "Received", "Noted", "Cheers!", "Okay, thanks", "No worries",
    "Keep going", "Close window", "Hide", "Back to app", "Let's go",
    "I'm done", "All set", "Superb", "Fabulous", "Got it, thanks",
    "Okay, bye", "Exit", "Close popup", "Return", "Proceed"
];

function applyRandomDismissText() {
    const idx = Math.floor(Math.random() * DISMISS_BUTTON_TEXTS.length);
    document.getElementById('dismissText').innerText = DISMISS_BUTTON_TEXTS[idx];
}

// ── 4. 30+ Unique Rotating Quick Tips ────────────────────────────────────────
const QUICK_TIPS = [
    "Bookmark Spotune with Ctrl + D so it's one click away next time.",
    "FLAC preserves the highest audio quality.",
    "MP3 320 kbps is the best balance between quality and file size.",
    "Playlist downloads may take longer depending on the number of songs.",
    "Use playlists to download multiple songs in one go.",
    "Lossless formats like FLAC and WAV ignore quality bitrate selections.",
    "Spotune works without Spotify credentials using public page scraping.",
    "Providing Spotify API credentials in the .env enables fast official lookups.",
    "You can pause and resume active downloads dynamically.",
    "If a download fails, Spotune automatically fails over to other providers.",
    "Smart Cancellation lets you download completed tracks as a ZIP instantly.",
    "Mutagen writes full ID3 metadata tags directly to output files.",
    "Spotune is 100% free and funded entirely by community donations.",
    "Audio quality below 192 kbps is perfect for saving mobile data.",
    "SoundCloud and Tidal can serve as fallback engines on demand.",
    "Isolated job folders are auto-deleted 1 hour after completion.",
    "AAC (M4A) format works beautifully on Apple Music and iOS devices.",
    "Use the Feedback section to request new features directly.",
    "Opus is highly optimized for modern speech and streaming compression.",
    "WAV format stores completely uncompressed studio raw audio data.",
    "Spotune sanitizes all track filenames to work on Windows, Linux, and macOS.",
    "You can customize backend concurrency worker counts in your .env file.",
    "The API Online status badge polls health status every 8 seconds.",
    "Spotune preserves original track/disc numbers for complete albums.",
    "We embed composers and lyrics tags directly inside files when available.",
    "Deezer downloads require configuring a valid ARL token in the backend.",
    "The scorer ranks download providers by success rates and latencies.",
    "Spotune isolates temporary job files to prevent workspace cross-talk.",
    "Rate limiting is applied to protect the backend from query spam.",
    "The Docker setup bundles FFmpeg automatically for easy deployment."
];

let currentTipIdx = 0;
function rotateQuickTip() {
    currentTipIdx = (currentTipIdx + 1) % QUICK_TIPS.length;
    const tipsText = document.getElementById('tipsText');
    tipsText.style.opacity = 0;
    setTimeout(() => {
        tipsText.innerText = QUICK_TIPS[currentTipIdx];
        tipsText.style.opacity = 1;
    }, 300);
}

// ── 5. Health Status Monitor ─────────────────────────────────────────────────
async function checkAPIHealth() {
    const badge = document.getElementById('apiHealthBadge');
    const label = document.getElementById('healthLabel');
    const apiBase = getApiBase();

    try {
        const resp = await fetch(`${apiBase}/health`);
        const data = await resp.json();
        if (data.status === 'online') {
            badge.className = 'health-badge status-online';
            label.innerText = '🟢 API Online';
        } else {
            badge.className = 'health-badge status-offline';
            label.innerText = '🔴 API Offline';
        }
    } catch {
        badge.className = 'health-badge status-offline';
        label.innerText = '🔴 API Offline';
    }
}

// ── 6. Sticky Navigation Link Highlighting ───────────────────────────────────
function initNavigation() {
    const links = document.querySelectorAll('.nav-link');
    const sections = document.querySelectorAll('section');

    window.addEventListener('scroll', () => {
        let current = '';
        sections.forEach(section => {
            const sectionTop = section.offsetTop;
            const sectionHeight = section.clientHeight;
            if (pageYOffset >= (sectionTop - 120)) {
                current = section.getAttribute('id');
            }
        });

        links.forEach(link => {
            link.classList.remove('active');
            if (link.getAttribute('href') === `#${current}`) {
                link.classList.add('active');
            }
        });
    });

    // Smooth Scroll
    links.forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const targetId = link.getAttribute('href');
            const targetSec = document.querySelector(targetId);
            window.scrollTo({
                top: targetSec.offsetTop - 80,
                behavior: 'smooth'
            });
            // Close mobile menu
            document.getElementById('navMenu').classList.remove('active');
            document.getElementById('navToggle').classList.remove('active');
        });
    });

    // Mobile Hamburger Toggle
    const navToggle = document.getElementById('navToggle');
    const navMenu = document.getElementById('navMenu');
    navToggle.addEventListener('click', () => {
        navToggle.classList.toggle('active');
        navMenu.classList.toggle('active');
    });
}

// ── 7. Toggle Quality Dropdown (Lossless has no bitrate) ──────────────────────
document.getElementById('formatSelect').addEventListener('change', (e) => {
    const fmt = e.target.value;
    const qualityWrapper = document.getElementById('qualityWrapper');
    if (fmt === 'flac' || fmt === 'wav') {
        qualityWrapper.style.display = 'none';
    } else {
        qualityWrapper.style.display = 'block';
    }
});

// ── 8. Resolve Metadata Workflow ─────────────────────────────────────────────
document.getElementById('btnSearch').addEventListener('click', async () => {
    const url = document.getElementById('urlInput').value.trim();
    if (!url) return alert('Please enter a Spotify URL.');

    const btnSearch = document.getElementById('btnSearch');
    const loaderFrame = document.getElementById('loadingFrame');
    const emptyState = document.getElementById('metaEmptyState');
    const configSection = document.getElementById('configSection');

    btnSearch.disabled = true;
    emptyState.style.display = 'none';
    configSection.style.display = 'none';
    loaderFrame.style.display = 'flex';
    
    // Rotate messages during lookup
    const msgInterval = setInterval(() => {
        document.getElementById('loadingMessage').innerText = getRandomLoadingMessage();
    }, 1500);

    metadataStartTime = performance.now();
    const apiBase = getApiBase();

    try {
        const resp = await fetch(`${apiBase}/metadata`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ url })
        });
        const body = await resp.json();
        
        clearInterval(msgInterval);
        loaderFrame.style.display = 'none';

        if (!body.ok) {
            alert(body.message || 'Failed to extract metadata.');
            emptyState.style.display = 'flex';
            return;
        }

        const data = body.data.data;
        const type = body.data.type;
        currentMetadata = { url, type };

        // Populate fields
        const art = document.getElementById('trackArt');
        const title = document.getElementById('trackTitle');
        const artist = document.getElementById('trackArtist');
        const tagType = document.getElementById('tagType');
        const tagDur = document.getElementById('tagDuration');
        const tagExtra = document.getElementById('tagExtra');

        if (type === 'track') {
            title.innerText = data.title;
            artist.innerText = data.artists.join(', ');
            art.src = data.cover_url || 'logo.png';
            tagType.innerText = 'Track';
            tagDur.innerText = formatDuration(data.duration_ms);
            tagExtra.innerText = data.album || 'Single';
            tagExtra.style.display = 'inline-block';
        } else {
            title.innerText = data.name;
            artist.innerText = data.owner || 'Spotify Creator';
            art.src = data.cover_url || 'logo.png';
            tagType.innerText = 'Playlist';
            tagDur.innerText = `${data.total_tracks} tracks`;
            tagExtra.style.display = 'none';
        }

        // Show config section
        configSection.style.display = 'grid';

        // Calculate and show lookup duration
        const durationMs = Math.round(performance.now() - metadataStartTime);
        document.getElementById('lookupTimer').innerText = `Resolved in ${(durationMs / 1000).toFixed(2)}s`;

    } catch (err) {
        clearInterval(msgInterval);
        loaderFrame.style.display = 'none';
        emptyState.style.display = 'flex';
        alert('Could not establish connection to the metadata server.');
    } finally {
        btnSearch.disabled = false;
    }
});

function formatDuration(ms) {
    if (!ms) return '0:00';
    const totalSecs = Math.floor(ms / 1000);
    const mins = Math.floor(totalSecs / 60);
    const secs = totalSecs % 60;
    return `${mins}:${secs.toString().padStart(2, '0')}`;
}

// ── 9. Submit to Queue & Dynamic Progress Polling ────────────────────────────
document.getElementById('btnQueue').addEventListener('click', async () => {
    if (!currentMetadata) return;

    const format = document.getElementById('formatSelect').value;
    const quality = document.getElementById('qualitySelect').value;
    const endpoint = currentMetadata.type === 'track' ? '/download/track' : '/playlist/download';
    const apiBase = getApiBase();

    const configSection = document.getElementById('configSection');
    const progressFrame = document.getElementById('progressFrame');
    const successFrame = document.getElementById('successFrame');

    configSection.style.display = 'none';
    successFrame.style.display = 'none';
    progressFrame.style.display = 'block';

    try {
        const resp = await fetch(`${apiBase}${endpoint}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                url: currentMetadata.url,
                format,
                quality
            })
        });
        const body = await resp.json();
        
        if (body.ok) {
            const job = body.data;
            // Begin tracking progress
            startPollingProgress(job.job_id);
        } else {
            alert(body.message);
            resetWorkflow();
        }
    } catch {
        alert('Failed to submit job to the download manager.');
        resetWorkflow();
    }
});

function resetWorkflow() {
    document.getElementById('configSection').style.display = 'none';
    document.getElementById('progressFrame').style.display = 'none';
    document.getElementById('successFrame').style.display = 'none';
    document.getElementById('metaEmptyState').style.display = 'flex';
    document.getElementById('urlInput').value = '';
    currentMetadata = null;
}

// ── 10. Progress Polling Lifecycle ───────────────────────────────────────────
function startPollingProgress(jobId) {
    activePollingJobId = jobId;
    if (activePollingInterval) clearInterval(activePollingInterval);

    const titleEl = document.getElementById('progressTitle');
    const percentEl = document.getElementById('progressPercent');
    const fillEl = document.getElementById('progressBarFill');
    const stageEl = document.getElementById('progressStage');
    const apiBase = getApiBase();

    activePollingInterval = setInterval(async () => {
        try {
            const resp = await fetch(`${apiBase}/queue/${activePollingJobId}`);
            const body = await resp.json();
            if (!body.ok) return;

            const job = body.data;
            const pct = job.percent || 0;
            
            percentEl.innerText = `${Math.round(pct)}%`;
            fillEl.style.width = `${pct}%`;
            
            // Map stages cleanly
            if (job.status === 'downloading') {
                titleEl.innerText = 'Downloading Audio Source';
                stageEl.innerText = `Fetching streams... ${Math.round(pct)}%`;
            } else if (job.status === 'converting') {
                titleEl.innerText = 'Transcoding Audio Format';
                stageEl.innerText = 'FFmpeg processing...';
            } else if (job.status === 'zipping') {
                titleEl.innerText = 'Creating ZIP Archive';
                stageEl.innerText = 'Packaging playlist...';
            } else if (job.status === 'complete') {
                clearInterval(activePollingInterval);
                showSuccessState(job);
            } else if (job.status === 'failed') {
                clearInterval(activePollingInterval);
                alert(`Download failed: ${job.error || 'Unknown error'}`);
                resetWorkflow();
            } else if (job.status === 'cancelled') {
                clearInterval(activePollingInterval);
                resetWorkflow();
            }
        } catch {
            // Wait silently on network glitch
        }
    }, 2000);
}

// Controls: Pause/Resume inside progress
document.getElementById('btnPauseResume').addEventListener('click', async () => {
    if (!activePollingJobId) return;
    const btn = document.getElementById('btnPauseResume');
    const apiBase = getApiBase();
    const isPlaylist = currentMetadata.type === 'playlist';

    if (btn.innerText.includes('Pause')) {
        const path = isPlaylist ? `/playlist/${activePollingJobId}/pause` : `/queue/${activePollingJobId}/pause`;
        await fetch(`${apiBase}${path}`, { method: 'POST' });
        btn.innerText = '▶ Resume';
        document.getElementById('progressStage').innerText = 'Paused';
        if (activePollingInterval) clearInterval(activePollingInterval);
    } else {
        const path = isPlaylist ? `/playlist/${activePollingJobId}/resume` : `/queue/${activePollingJobId}/resume`;
        await fetch(`${apiBase}${path}`, { method: 'POST' });
        btn.innerText = '⏸ Pause';
        startPollingProgress(activePollingJobId);
    }
});

// Controls: Cancel inside progress
document.getElementById('btnCancelDownload').addEventListener('click', () => {
    if (!activePollingJobId) return;
    const isPlaylist = currentMetadata.type === 'playlist';
    const apiBase = getApiBase();

    if (!isPlaylist) {
        // Track cancel
        fetch(`${apiBase}/download/${activePollingJobId}/cancel`, { method: 'POST' }).then(() => {
            if (activePollingInterval) clearInterval(activePollingInterval);
            resetWorkflow();
        });
    } else {
        // Playlist cancellation popup
        activeCancelJobId = activePollingJobId;
        document.getElementById('cancelModal').classList.add('active');
    }
});

// Close Cancel Modal
document.getElementById('btnCloseModal').addEventListener('click', () => {
    document.getElementById('cancelModal').classList.remove('active');
    activeCancelJobId = null;
});

// Smart Cancellation Handler
async function handleCancelChoice(option) {
    if (!activeCancelJobId) return;
    document.getElementById('cancelModal').classList.remove('active');
    const apiBase = getApiBase();

    if (activePollingInterval) clearInterval(activePollingInterval);

    try {
        const resp = await fetch(`${apiBase}/playlist/${activeCancelJobId}/cancel`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ option })
        });
        const body = await resp.json();
        
        if (body.ok) {
            const data = body.data;
            alert(
                `Smart Cancellation Applied:\n` +
                `- Mode: ${data.option.toUpperCase()}\n` +
                `- Tracks Completed: ${data.completed_tracks}\n` +
                `- Tracks Failed: ${data.failed_tracks}\n` +
                `- Tracks Cancelled: ${data.cancelled_tracks}`
            );
            if (data.download_url) {
                window.open(`${apiBase.replace('/api/v1', '')}${data.download_url}`);
            }
        }
    } catch {
        alert('Smart cancellation encountered an error.');
    }
    resetWorkflow();
}

// Bind cancellation modal options
document.querySelectorAll('.cancel-option-btn').forEach(btn => {
    btn.addEventListener('click', () => {
        const option = btn.getAttribute('data-option');
        handleCancelChoice(option);
    });
});

// ── 11. Success State & Support Popup ────────────────────────────────────────
function showSuccessState(job) {
    document.getElementById('progressFrame').style.display = 'none';
    const successFrame = document.getElementById('successFrame');
    
    document.getElementById('successFormat').innerText = job.format.toUpperCase();
    document.getElementById('successQuality').innerText = job.format === 'flac' || job.format === 'wav' ? 'Lossless' : `${job.quality} kbps`;
    
    // Estimate size or display placeholder
    document.getElementById('successSize').innerText = job.zip_size_bytes ? `${(job.zip_size_bytes / 1024 / 1024).toFixed(1)} MB` : "Estimated 12.4 MB";

    const apiBase = getApiBase();
    const downloadBtn = document.getElementById('btnDownloadFile');
    downloadBtn.href = job.job_type === 'playlist' ? `${apiBase}/playlist/${job.job_id}/zip` : `${apiBase}/download/${job.job_id}`;

    successFrame.style.display = 'block';

    // Show optional support popup immediately
    setTimeout(() => {
        applyRandomDismissText();
        document.getElementById('supportPopup').classList.add('active');
    }, 800);
}

// Close Support Popup Modal
document.getElementById('btnCloseSupportPopup').addEventListener('click', () => {
    document.getElementById('supportPopup').classList.remove('active');
});

document.getElementById('btnDismissPopup').addEventListener('click', () => {
    document.getElementById('supportPopup').classList.remove('active');
});

// ── 12. Feedback Form Submission ─────────────────────────────────────────────
document.getElementById('feedbackForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const form = e.target;
    const btn = document.getElementById('btnSubmitFeedback');
    const responseMsg = document.getElementById('feedbackResponse');

    btn.disabled = true;
    btn.innerText = 'Submitting...';
    responseMsg.className = 'feedback-response-msg';
    responseMsg.innerText = '';

    try {
        const resp = await fetch(form.action, {
            method: form.method,
            headers: { 'Accept': 'application/json' },
            body: new FormData(form)
        });
        
        if (resp.ok) {
            responseMsg.className = 'feedback-response-msg success';
            responseMsg.innerText = 'Thank you! Your feedback has been submitted successfully.';
            form.reset();
        } else {
            responseMsg.className = 'feedback-response-msg error';
            responseMsg.innerText = 'Oops! There was a problem submitting your feedback. Please try again.';
        }
    } catch {
        responseMsg.className = 'feedback-response-msg error';
        responseMsg.innerText = 'Could not submit feedback due to network issues.';
    } finally {
        btn.disabled = false;
        btn.innerText = 'Submit Feedback';
    }
});

// ── 13. Initialization & Event Triggers ──────────────────────────────────────
applyRandomHero();
initNavigation();

checkAPIHealth();
setInterval(checkAPIHealth, 8000);

// Rotate Quick Tips every 7 seconds
setInterval(rotateQuickTip, 7000);
