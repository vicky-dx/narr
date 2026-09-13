package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/IljaN/narr/internal/browser"
	"github.com/IljaN/narr/internal/downloader"
	"github.com/IljaN/narr/internal/media"
	"github.com/IljaN/narr/internal/netflix"
	"github.com/IljaN/narr/internal/storage"
)

func main() {
	args := &Args{}
	mustParse(args)

	ctx := context.Background()

	// 1. Optionally launch browser instance if requested
	if args.Browser != "" {
		port := 9222
		if args.ChromeURL != nil && args.ChromeURL.Port() != "" {
			var p int
			if _, err := fmt.Sscanf(args.ChromeURL.Port(), "%d", &p); err == nil && p > 0 {
				port = p
			}
		}

		launchCfg := browser.LaunchConfig{
			Browser:    args.Browser,
			ProfileDir: args.ProfileDir,
			Port:       port,
			Headless:   args.Headless,
		}

		log.Printf("🚀 Launching browser (%s) on port %d...", args.Browser, port)
		proc, err := browser.Launch(ctx, launchCfg)
		if err != nil {
			log.Fatalf("Failed to launch browser: %v", err)
		}
		defer func() {
			if proc != nil {
				_ = proc.Kill()
			}
		}()

		if err := browser.WaitForDebugger(ctx, args.ChromeURL.String(), 15*time.Second); err != nil {
			log.Fatalf("Failed waiting for browser debugger: %v", err)
		}
	}

	// 2. Connect to browser debugger
	chrome := browser.ConnectUntilSuccess(ctx, args.ChromeURL.String(), 1*time.Second)
	log.Printf("Ꙫ Successfully connected to browser debugger at %s", args.ChromeURL.String())

	nflx := netflix.New(chrome)

	// 3. Initialize Media Prober & Downloader Queue
	prober := media.NewProber()
	queue := downloader.NewQueue(prober)
	defer queue.Release()

	// 4. Register download status logger
	queue.OnStatusReceived(func(status downloader.Status) {
		task := status.Task()
		isSubtitle := task.SubtitleLang != ""
		icon := "▼"
		if isSubtitle {
			icon = "🔤"
		}
		switch s := status.(type) {
		case downloader.Queuing:
			break
		case downloader.Begin:
			log.Printf("%s [%s] Downloading %s to %s", icon, s.TaskId(), task.VideoURL, task.FullFilePath)
		case downloader.Finished:
			log.Printf("✓ [%s] Finished %s  ⟾  %s (got %d bytes in %.2fs)",
				s.TaskId(), task.VideoURL, task.FullFilePath, s.BytesReceived(), s.Duration().Seconds())
		case downloader.Skipped:
			log.Printf("⏭ [%s] Already downloaded, skipping: %s", s.TaskId(), task.FullFilePath)
		}
	})

	// 5. Navigate to initial URL using Netflix client
	if err := nflx.NavigateTo(ctx, args.VideoURL.String()); err != nil {
		log.Fatalf("Failed to navigate to %s: %v", args.VideoURL.String(), err)
	}

	// 6. Listen for media and navigation events
	currentURL := args.VideoURL.String()
	// lastMeta caches the most complete metadata seen for the current episode.
	// Subtitle events fire at slightly different times than audio events, so we
	// reuse the last good metadata instead of calling GetMetadata() again and
	// risking an incomplete result (e.g. missing SeasonNum → wrong folder).
	var lastMeta downloader.Metadata

	for event := range nflx.Listen(ctx) {
		switch event.Type {
		case netflix.MediaUrlReceivedEvent:
			meta := nflx.GetMetadata(ctx)
			// Keep the best metadata: prefer a result that has SeasonNum over one that doesn't.
			if meta.SeasonNum != "" || lastMeta.IsEmpty() {
				lastMeta = meta
			}
			if !lastMeta.IsEmpty() {
				seasonStr := ""
				if lastMeta.SeasonNum != "" {
					seasonStr = fmt.Sprintf("Season %s ", lastMeta.SeasonNum)
				}
				epStr := storage.FormatEpisodeNum(lastMeta.EpisodeNum)
				if epStr != "" {
					epStr += " - "
				}
				log.Printf("🎬 Metadata detected: %s [%s%s%s]", lastMeta.ShowTitle, seasonStr, epStr, lastMeta.EpisodeTitle)
			}
			task := downloader.Task{
				SrcURL:      nflx.ToDownloadableURL(event.Payload),
				VideoURL:    currentURL,
				DownloadDir: args.DownloadDir,
				Meta:        lastMeta,
			}
			if err := queue.QueueDownload(task); err != nil {
				log.Printf("Error queueing audio download: %v", err)
			}

		case netflix.SubtitleUrlReceivedEvent:
			// Reuse cached metadata so subtitles always land in the same folder as audio.
			// If we have no cached metadata yet, do a fresh fetch.
			meta := lastMeta
			if meta.IsEmpty() {
				meta = nflx.GetMetadata(ctx)
				lastMeta = meta
			}
			task := downloader.Task{
				SrcURL:      event.Payload,
				VideoURL:    currentURL,
				DownloadDir: args.DownloadDir,
				Meta:        meta,
			}
			if err := queue.QueueSubtitleDownload(task); err != nil {
				log.Printf("Error queueing subtitle download: %v", err)
			}

		case netflix.NavigatedEvent:
			log.Printf("ᐅ Navigated to %s", event.Payload)
			currentURL = event.Payload
			lastMeta = downloader.Metadata{} // reset on navigation to new episode
		}
	}
}
