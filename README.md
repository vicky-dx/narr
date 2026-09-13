# Narr
Netflix Audio Ripper - Download audio tracks from Netflix to sample your favourite shows. :musical_note:

## Usage

### Option A: Automatic Browser Launch (Recommended)
Narr can automatically launch Chrome, Edge, or Brave with an isolated user profile and connect to it:

```bash
# Launch Google Chrome with dedicated persistent profile
narr --browser chrome "https://www.netflix.com/watch/12345678"

# Launch Microsoft Edge
narr --browser edge "https://www.netflix.com/watch/12345678"

# Launch Brave Browser
narr --browser brave "https://www.netflix.com/watch/12345678"

# Specify a custom profile folder
narr -b chrome -p "./my-profile" "https://www.netflix.com/watch/12345678"
```
*Note: The default profile is saved to `~/.narr/profiles/<browser>`, so you only need to log in to Netflix once!*

### Option B: Connect to an Existing Browser
Alternatively, start your browser manually with remote debugging enabled:
```bash
 google-chrome --remote-debugging-port=9222
 brave-browser --remote-debugging-port=9222
 ./msedge.exe  --remote-debugging-port=9222
```
And run narr:
```bash
narr "https://www.netflix.com/watch/12345678"
```

Observe the progress in the terminal:

```bash
2023/02/18 18:34:25 ▼ Downloading https://www.netflix.com/watch/81237996?trackId=14170056  ⟾  /home/looper/81237996-14170056-4037200794235010051
2023/02/18 18:34:35 ✓ Finished    https://www.netflix.com/watch/81237996?trackId=14170056  ⟾  /home/looper/81237996-14170056-4037200794235010051, got 65346400 bytes
```

You can navigate to any other show or episode or change the language of the audio track while narr is running. It will
download the audio track of the currently playing episode.

```bash
2023/02/18 18:34:25 ▼ Downloading https://www.netflix.com/watch/81237996?trackId=14170056  ⟾  /home/looper/81237996-14170056-4037200794235010051
2023/02/18 18:34:35 ✓ Finished    https://www.netflix.com/watch/81237996?trackId=14170056  ⟾  /home/looper/81237996-14170056-4037200794235010051, got 65346400 bytes
2023/02/18 16:59:42 🗺Navigate to https://www.netflix.com/watch/81238005?trackId=14170056 
2023/02/18 16:59:43 ▼ Downloading https://www.netflix.com/watch/81238005?trackId=14170056  ⟾  /home/looper/81238005-14170056-605394647632969758
2023/02/18 16:59:53 ✓ Finished    https://www.netflix.com/watch/81238005?trackId=14170056  ⟾  /home/looper/81238005-14170056-605394647632969758, got 65346400 bytes
```

It is also possible to navigate to the Netflix home page. Narr will then download audio tracks of trailers or previews.

## How it works

Narr uses the [Chrome DevTools Protocol](https://chromedevtools.github.io/devtools-protocol/) to communicate with the
browser. It intercepts network media requests, extracts show/episode metadata from the DOM, and losslessly downloads audio tracks in `.m4a` format.

## Architecture (SRP, DRY, KISS, YAGNI)

The codebase is organized into modular packages under `internal/` with strict **Single Responsibility** and **zero premature abstractions**:

```text
├── internal/
│   ├── netflix/     # Netflix client (DOM & stream interception via CDP)
│   ├── browser/     # CDP connection management & dual-stack (IPv4/IPv6) fallback
│   ├── media/       # MP4 ISOBMFF header probing & ISO-639-2 language decoding
│   ├── storage/     # Path resolution, OS character sanitization & atomic file writes
│   └── downloader/  # Concurrent worker pool, deduplication & status event reporting
├── args.go          # CLI argument validation & versioning
└── main.go          # Dependency injection & application orchestration
```

### Key Principles:
- **Single Responsibility (SRP):** Each package handles exactly one domain (browser connection, storage paths, media probing, or downloader queue).
- **Don't Repeat Yourself (DRY):** Shared metadata types and path resolution logic are centralized and reused across components.
- **YAGNI & KISS:** No over-engineered provider registries or unused interfaces. Direct, readable, and fully testable Go code.

### Features:
- **Automatic Organization:** Automatically creates directories organized by show and season (`<ShowTitle>/Season <N>/`) and formats files as `E<EpisodeNumber> - <EpisodeTitle> - <Language>.m4a` (e.g. `DANG/Season 1/E01 - Pilot - English.m4a`).
- **Smart Deduplication & Skip:** Skips downloads immediately if the target file already exists on disk or was already queued.
- **Atomic Downloads:** Writes to `.downloading` files and renames upon completion to guarantee uncorrupted audio files.

## Build & Test

```bash
# Run all unit tests
go test -v ./...

# Build binary
go build -o narr.exe .
```

## Flags

```bash
Usage: narr [--chrome-url CHROME-URL] [--browser BROWSER] [--profile-dir PROFILE-DIR] [--headless] VIDEOURL [DOWNLOADDIR]

Positional arguments:
  VIDEOURL               url of the video to download audio from. Must be a supported platform URL (e.g. Netflix).
  DOWNLOADDIR            directory where to download the audio files. Defaults to current working directory.

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

