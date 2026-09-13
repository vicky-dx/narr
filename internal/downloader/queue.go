package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-queue/queue"

	"github.com/IljaN/narr/internal/media"
	"github.com/IljaN/narr/internal/storage"
)

// Queue coordinates concurrent background downloads and status reporting.
type Queue struct {
	pool             *queue.Queue
	statusMsgs       chan Status
	seenURLs         sync.Map // dedup by source URL (audio)
	seenSubtitleURLs sync.Map // dedup by source URL (subtitles)
	seenPaths        sync.Map // dedup by destination path (prevents race on same file)
	prober           media.Prober
}

// NewQueue creates a new download worker queue with up to 8 concurrent workers.
func NewQueue(prober media.Prober) *Queue {
	if prober == nil {
		prober = media.NewProber()
	}
	return &Queue{
		pool:       queue.NewPool(8),
		statusMsgs: make(chan Status, 50),
		seenURLs:   sync.Map{},
		prober:     prober,
	}
}

// Release shuts down the worker pool and releases associated resources.
func (q *Queue) Release() {
	q.pool.Release()
}

// OnStatusReceived registers a callback invoked whenever a task's status changes.
func (q *Queue) OnStatusReceived(callback func(Status)) {
	go func() {
		for msg := range q.statusMsgs {
			callback(msg)
		}
	}()
}

// QueueDownload enqueues a media download task if not already processed in this session.
func (q *Queue) QueueDownload(t Task) error {
	// Deduplicate by URL within session
	if _, loaded := q.seenURLs.LoadOrStore(t.SrcURL, true); loaded {
		return nil
	}

	go func(t Task) {
		taskId := newTaskId()
		q.statusMsgs <- Queuing{&taskInfo{taskId, t}}

		_ = q.pool.QueueTask(func(ctx context.Context) error {
			start := time.Now()

			resp, err := http.Get(t.SrcURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, t.VideoURL)
			}

			header := make([]byte, 3000)
			if _, err = io.ReadAtLeast(resp.Body, header, 3000); err != nil {
				return fmt.Errorf("cannot read media header: %w", err)
			}

			probeRes, err := q.prober.Probe(header)
			if err != nil {
				return fmt.Errorf("probe error on %s: %w", t.VideoURL, err)
			}

			if !probeRes.IsAudio {
				return nil
			}

			downloadPath := storage.ResolveDownloadPath(
				t.VideoURL,
				t.DownloadDir,
				t.Meta.ShowTitle,
				t.Meta.SeasonNum,
				t.Meta.EpisodeNum,
				t.Meta.EpisodeTitle,
				probeRes.Language,
			)
			t.FullFilePath = downloadPath

			// Deduplicate by destination path to prevent concurrent writes to the same file
			if _, claimed := q.seenPaths.LoadOrStore(downloadPath, true); claimed {
				return nil
			}

			// Skip if file already exists on disk
			if storage.Exists(downloadPath) {
				q.statusMsgs <- Skipped{&taskInfo{taskId, t}}
				return nil
			}

			q.statusMsgs <- Begin{&taskInfo{taskId, t}}

			bytesWritten, err := storage.WriteAtomic(downloadPath, header, resp.Body)
			if err != nil {
				return err
			}

			q.statusMsgs <- Finished{
				taskInfo:      &taskInfo{taskId, t},
				bytesReceived: bytesWritten,
				duration:      time.Since(start),
			}

			return nil
		})
	}(t)

	return nil
}

// QueueSubtitleDownload enqueues a TTML subtitle download if not already processed in this session.
// Unlike audio, subtitles require no MP4 probing — the full XML body is read directly,
// the xml:lang attribute is extracted to name the file, and the content is saved as .ttml.
func (q *Queue) QueueSubtitleDownload(t Task) error {
	// Deduplicate subtitle URLs separately from audio URLs
	if _, loaded := q.seenSubtitleURLs.LoadOrStore(t.SrcURL, true); loaded {
		return nil
	}

	go func(t Task) {
		taskId := newTaskId()
		q.statusMsgs <- Queuing{&taskInfo{taskId, t}}

		_ = q.pool.QueueTask(func(ctx context.Context) error {
			start := time.Now()

			resp, err := http.Get(t.SrcURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status %d for subtitle %s", resp.StatusCode, t.SrcURL)
			}

			// Read the full TTML body (subtitles are small text files, not chunked)
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("cannot read subtitle body: %w", err)
			}

			// Extract language from xml:lang="xx" attribute in the TTML root element.
			// Also detect SDH/CC tracks (nttm:textType="SDHS") so they get a distinct
			// filename suffix (-cc) and don't collide with regular subtitle files.
			lang := storage.ExtractTTMLLang(body)
			lang = lang + storage.ExtractTTMLSubtitleType(body) // e.g. "de" or "de-cc"
			t.SubtitleLang = lang

			downloadPath := storage.ResolveSubtitlePath(
				t.VideoURL,
				t.DownloadDir,
				t.Meta.ShowTitle,
				t.Meta.SeasonNum,
				t.Meta.EpisodeNum,
				t.Meta.EpisodeTitle,
				lang,
			)
			t.FullFilePath = downloadPath

			// Deduplicate by destination path to prevent concurrent writes to the same file
			if _, claimed := q.seenPaths.LoadOrStore(downloadPath, true); claimed {
				return nil
			}

			// Skip if subtitle file already exists
			if storage.Exists(downloadPath) {
				q.statusMsgs <- Skipped{&taskInfo{taskId, t}}
				return nil
			}

			q.statusMsgs <- Begin{&taskInfo{taskId, t}}

			// Write the full TTML body atomically (already in memory — no streaming needed)
			tempPath := fmt.Sprintf("%s.%d.downloading", downloadPath, time.Now().UnixNano())
			if err := os.WriteFile(tempPath, body, 0644); err != nil {
				return err
			}
			if err := storage.RenameWithRetry(tempPath, downloadPath); err != nil {
				_ = os.Remove(tempPath)
				return err
			}

			q.statusMsgs <- Finished{
				taskInfo:      &taskInfo{taskId, t},
				bytesReceived: int64(len(body)),
				duration:      time.Since(start),
			}

			return nil
		})
	}(t)

	return nil
}
