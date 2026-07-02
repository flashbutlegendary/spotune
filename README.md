# Spotune

Get Spotify tracks in true lossless FLAC from Tidal, Qobuz & Amazon Music — no account required.

![Windows](https://img.shields.io/badge/Windows-10%2B-0078D6?style=for-the-badge&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSI1MTIiIGhlaWdodD0iNTEyIiB2aWV3Qm94PSIwIDAgMjAgMjAiPjxwYXRoIGZpbGw9IiNmZmZmZmYiIGZpbGwtcnVsZT0iZXZlbm9kZCIgZD0iTTIwIDEwLjg3M1YyMEw4LjQ3OSAxOC41MzdsLjAwMS03LjY2NEgyMFptLTEzLjEyIDAtLjAwMSA3LjQ2MUwwIDE3LjQ2MXYtNi41ODhoNi44OFpNMjAgOS4yNzNIOC40OGwtLjAwMS03LjgxTDIwIDB2OS4yNzNaNi44NzkgMS42NjZsLjAwMSA3LjYwN0gwVjIuNTM5bDYuODc5LS44NzNaIi8+PC9zdmc+)
![macOS](https://img.shields.io/badge/macOS-10.13%2B-000000?style=for-the-badge&logo=apple&logoColor=white)
![Linux](https://img.shields.io/badge/Linux-Any-FCC624?style=for-the-badge&logo=linux&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![Wails](https://img.shields.io/badge/Wails-v2-red?style=for-the-badge)

### [Download Latest Release](https://github.com/flashbutlegendary/spotune/releases)

---

## ✨ Features

- 🎵 **True Lossless FLAC** — fetch hi-fi audio from Tidal, Qobuz & Amazon Music
- 🔍 **Spotify-powered metadata** — search by Spotify track, album, or playlist URL
- 🏷️ **Rich tagging** — embeds full metadata: title, artist, album art, lyrics, ISRC & more
- 📃 **Lyrics support** — synced & unsynced lyrics via LRCLIB
- 🗂️ **Batch downloads** — download entire albums and playlists in one go
- 🚫 **No account needed** — no Spotify, Tidal, or Qobuz login required
- 🖥️ **Cross-platform** — native desktop app powered by Wails v2 + Go

---

## 🚀 Getting Started

1. Download the latest release for your platform from the [Releases page](https://github.com/flashbutlegendary/spotune/releases)
2. Run the app — no installation required
3. Paste any Spotify track, album, or playlist URL
4. Choose your preferred source (Tidal, Qobuz, or Amazon Music)
5. Hit download — your lossless files will be ready in seconds

---

## 🔧 Building from Source

**Prerequisites:**
- [Go 1.21+](https://go.dev)
- [Node.js 18+](https://nodejs.org) & [pnpm](https://pnpm.io)
- [Wails v2 CLI](https://wails.io) — `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- [ffmpeg](https://ffmpeg.org) (in PATH)

```bash
git clone https://github.com/flashbutlegendary/spotune.git
cd spotune
wails build
```

For development with hot reload:

```bash
wails dev
```

---

## ❓ FAQ

<details>
<summary>Is this software free?</summary>

_Yes. Spotune is completely free.
You do not need an account, login, or subscription.
All you need is an internet connection._

</details>

<details>
<summary>Can using this software get my Spotify account suspended or banned?</summary>

_No.
Spotune has no connection to your Spotify account.
Spotify metadata is obtained through reverse engineering of the Spotify Web Player — not through user authentication._

</details>

<details>
<summary>Where does the audio come from?</summary>

_Audio is fetched using third-party APIs from Tidal, Qobuz, and Amazon Music._

</details>

<details>
<summary>Why does metadata fetching sometimes fail?</summary>

_This usually happens because your IP address has been rate-limited.
You can wait and try again later, or use a VPN to bypass the rate limit._

</details>

<details>
<summary>Why does Windows Defender or antivirus flag or delete the file?</summary>

_This is a false positive.
It may occur because the executable is packaged as a portable binary.
If you are concerned, you can fork the repository and build the software yourself from source._

</details>

---

## ⚠️ Disclaimer

This project is for **educational and private use only**. The developer does not condone or encourage copyright infringement.

**Spotune** is a third-party tool and is not affiliated with, endorsed by, or connected to Spotify, Tidal, Qobuz, Amazon Music, or any other streaming service.

You are solely responsible for:

1. Ensuring your use of this software complies with your local laws.
2. Reading and adhering to the Terms of Service of the respective platforms.
3. Any legal consequences resulting from the misuse of this tool.

The software is provided "as is", without warranty of any kind. The author assumes no liability for any bans, damages, or legal issues arising from its use.

---

## 🙏 API Credits

[MusicBrainz](https://musicbrainz.org) · [LRCLIB](https://lrclib.net) · [Songlink/Odesli](https://song.link) · [Songstats](https://songstats.com)

---

> [!TIP]
>
> **Star the repo** to get notified of all new releases instantly ⭐
