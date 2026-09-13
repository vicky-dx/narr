# Narr

Netflix Audio & Subtitle Ripper — Automatically intercept and download audio tracks and subtitles from Netflix as you watch. 🎵 🔤

## Features

- **Audio & Subtitle Interception:** Automatically intercepts and downloads audio (`.m4a`) and subtitles (`.ttml`) directly from the stream.
- **Auto-Browser Launch:** Automatically launches Chrome, Edge, or Brave with remote debugging enabled and a persistent profile so you only log in once.
- **Smart Organization:** Saves everything to `./downloads/` organized by show, season, and episode:
  - `downloads/<ShowTitle>/Season <N>/<EpisodeNum> - <EpisodeTitle> - <Language>.m4a`
  - `downloads/<ShowTitle>/Season <N>/<EpisodeNum> - <EpisodeTitle> - <Language>.ttml`
- **SDH / Closed Caption Detection:** Identifies SDH/CC subtitle tracks and tags them with a `-cc` suffix (e.g. `en-cc.ttml`) to prevent collisions with regular subtitle tracks.
- **Deduplication & Resume:** Skips files that have already been downloaded, preventing duplicate network requests and overwrites.
- **Windows Resilient:** Employs retry backoff when saving files to protect against Windows Defender and search indexer file locks.

## Usage

### Option A: Automatic Browser Launch (Recommended)

Narr can automatically launch your preferred browser with an isolated user profile:

```bash
# Launch Google Chrome with dedicated persistent profile
narr --browser chrome "https://www.netflix.com/watch/12345678"

# Launch Microsoft Edge
narr --browser edge "https://www.netflix.com/watch/12345678"

# Launch Brave Browser
narr --browser brave "https://www.netflix.com/watch/12345678"

# Specify a custom profile folder
narr -b chrome -p "./chrome-debug-profile" "https://www.netflix.com/watch/12345678"
```
*Note: The default profile is saved to `~/.narr/profiles/<browser>`, preserving your Netflix login session.*

### Option B: Connect to an Existing Browser

Start your browser manually with remote debugging enabled:

```bash
# Chrome
google-chrome --remote-debugging-port=9222

# Brave
brave-browser --remote-debugging-port=9222

# Edge (Windows)
msedge.exe --remote-debugging-port=9222
```

Then run narr:

```bash
narr "https://www.netflix.com/watch/12345678"
```

### Live Interception

While Narr is running, you can navigate to any episode or switch audio/subtitle languages in the Netflix player. Narr will detect the new streams and download them on the fly:

```text
2026/09/13 14:05:17 Ꙫ Successfully connected to browser debugger at http://127.0.0.1:9222
2026/09/13 14:05:26 🎬 Metadata detected: DANG! [Season 1 E06 - The Milking Room]
2026/09/13 14:05:27 ▼ [5bafc98020e5] Downloading ... to downloads\DANG!\Season 1\E06 - The Milking Room - German.m4a
2026/09/13 14:05:28 🔤 [626894bd4614] Downloading ... to downloads\DANG!\Season 1\E06 - The Milking Room - de.ttml
2026/09/13 14:05:30 ✓ [626894bd4614] Finished    downloads\DANG!\Season 1\E06 - The Milking Room - de.ttml, got 81058 bytes in 1.8s
2026/09/13 14:06:05 ✓ [5bafc98020e5] Finished    downloads\DANG!\Season 1\E06 - The Milking Room - German.m4a, got 36670020 bytes in 38.2s
```

## CLI Reference

```text
Usage: narr [--chrome-url CHROME-URL] [--browser BROWSER] [--profile-dir PROFILE-DIR] [--headless] VIDEOURL [DOWNLOADDIR]

Positional arguments:
  VIDEOURL               url of the video to download audio and subtitles from. Must be a Netflix URL.
  DOWNLOADDIR            directory where to download the audio and subtitle files. Defaults to ./downloads.

Options:
  --chrome-url CHROME-URL, -c CHROME-URL
                         url of the chrome debugger. [default: http://127.0.0.1:9222]
  --browser BROWSER, -b BROWSER
                         launch browser automatically (chrome, edge, brave, or executable path).
  --profile-dir PROFILE-DIR, -p PROFILE-DIR
                         custom browser user profile directory. Defaults to ~/.narr/profiles/<browser>.
  --headless             run launched browser in headless mode.
  --help, -h             display this help and exit
  --version              display version and exit
```

## Project Structure

```text
├── internal/
│   ├── browser/     # Browser launcher & CDP debugger connection management
│   ├── downloader/  # Concurrent download queue, deduplication & status reporting
│   ├── media/       # MP4 ISOBMFF header probing & audio language extraction
│   ├── netflix/     # Netflix DOM metadata extraction & media/subtitle stream interception
│   └── storage/     # Path resolution, TTML subtitle parsing, and atomic file operations
├── args.go          # CLI argument parsing and validation
└── main.go          # Main application loop and event handling
```

## Build & Test

```bash
# Run tests
go test ./...

# Build binary
go build -o narr.exe .
```
