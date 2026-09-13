package downloader

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/IljaN/narr/internal/media"
)

func fileServerHandler(directory string) http.Handler {
	return http.StripPrefix("/files/", http.FileServer(http.Dir(directory)))
}

func TestDownloadQueue_Formats(t *testing.T) {
	ts := httptest.NewServer(fileServerHandler("../../test/testdata"))
	defer ts.Close()

	t.Run("ISO6Download", func(t *testing.T) {
		q := NewQueue(media.NewProber())
		defer q.Release()

		downloadDir := t.TempDir()

		var finishedPath string
		q.OnStatusReceived(func(status Status) {
			if s, ok := status.(Finished); ok {
				finishedPath = s.Task().FullFilePath
			}
		})

		task := Task{
			SrcURL:      ts.URL + "/files/audio_iso6.mp4",
			DownloadDir: downloadDir,
			VideoURL:    "http://example.com/video.mp4",
		}

		err := q.QueueDownload(task)
		assert.NoError(t, err)

		for i := 0; i < 100; i++ {
			if finishedPath != "" {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		assert.NotEmpty(t, finishedPath)
		assert.FileExists(t, finishedPath)
		assertIdenticalFiles(t, "../../test/testdata/audio_iso6.mp4", finishedPath)
	})

	t.Run("MP42Download", func(t *testing.T) {
		q := NewQueue(media.NewProber())
		defer q.Release()

		downloadDir := t.TempDir()

		var finishedPath string
		q.OnStatusReceived(func(status Status) {
			if s, ok := status.(Finished); ok {
				finishedPath = s.Task().FullFilePath
			}
		})

		task := Task{
			SrcURL:      ts.URL + "/files/audio_mp42.mp4",
			DownloadDir: downloadDir,
			VideoURL:    "http://example.com/video.mp4",
		}

		err := q.QueueDownload(task)
		assert.NoError(t, err)

		for i := 0; i < 100; i++ {
			if finishedPath != "" {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		assert.NotEmpty(t, finishedPath)
		assert.FileExists(t, finishedPath)
		assertIdenticalFiles(t, "../../test/testdata/audio_mp42.mp4", finishedPath)
	})

	t.Run("InvalidFile", func(t *testing.T) {
		q := NewQueue(media.NewProber())
		defer q.Release()

		downloadDir := t.TempDir()

		task := Task{
			SrcURL:      ts.URL + "/files/vid.mp4",
			DownloadDir: downloadDir,
			VideoURL:    "http://example.com/video.mp4",
		}

		err := q.QueueDownload(task)
		assert.NoError(t, err)

		for i := 0; i < 20; i++ {
			time.Sleep(50 * time.Millisecond)
			assertDirIsEmpty(t, downloadDir)
		}
	})
}

func TestDownloadQueue_Statuses(t *testing.T) {
	ts := httptest.NewServer(fileServerHandler("../../test/testdata"))
	defer ts.Close()

	q := NewQueue(media.NewProber())
	defer q.Release()

	var hasQueued, hasBegun, hasFinished bool

	q.OnStatusReceived(func(status Status) {
		switch status.(type) {
		case Queuing:
			hasQueued = true
		case Begin:
			hasBegun = true
		case Finished:
			hasFinished = true
		}
	})

	task := Task{
		SrcURL:      ts.URL + "/files/audio_iso6.mp4",
		DownloadDir: t.TempDir(),
		VideoURL:    "http://example.com/video.mp4",
	}

	err := q.QueueDownload(task)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		if hasQueued && hasBegun && hasFinished {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	assert.True(t, hasQueued)
	assert.True(t, hasBegun)
	assert.True(t, hasFinished)
}

func TestDownloadQueue_SkipAlreadyDownloaded(t *testing.T) {
	ts := httptest.NewServer(fileServerHandler("../../test/testdata"))
	defer ts.Close()

	downloadDir := t.TempDir()
	task := Task{
		SrcURL:      ts.URL + "/files/audio_iso6.mp4",
		DownloadDir: downloadDir,
		VideoURL:    "http://example.com/video.mp4",
		Meta: Metadata{
			ShowTitle:    "TestShow",
			EpisodeNum:   "E1",
			EpisodeTitle: "Pilot",
		},
	}

	q1 := NewQueue(media.NewProber())
	defer q1.Release()

	var finished bool
	q1.OnStatusReceived(func(status Status) {
		if _, ok := status.(Finished); ok {
			finished = true
		}
	})

	err := q1.QueueDownload(task)
	assert.NoError(t, err)

	for i := 0; i < 100; i++ {
		if finished {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, finished)

	// Second queue attempt: file exists on disk, should emit Skipped
	q2 := NewQueue(media.NewProber())
	defer q2.Release()

	var skipped bool
	q2.OnStatusReceived(func(status Status) {
		if _, ok := status.(Skipped); ok {
			skipped = true
		}
	})

	err = q2.QueueDownload(task)
	assert.NoError(t, err)

	for i := 0; i < 50; i++ {
		if skipped {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.True(t, skipped)
}

func TestDownloadQueue_DeduplicateURLs(t *testing.T) {
	q := NewQueue(media.NewProber())
	defer q.Release()

	task := Task{
		SrcURL:      "http://example.com/test-url.mp4",
		DownloadDir: t.TempDir(),
		VideoURL:    "http://example.com/video.mp4",
	}

	err := q.QueueDownload(task)
	assert.NoError(t, err)

	err = q.QueueDownload(task)
	assert.NoError(t, err)
}

func assertIdenticalFiles(t *testing.T, filePath1, filePath2 string) bool {
	h1, err := calculateMD5(filePath1)
	if err != nil {
		return assert.Fail(t, "failed to calculate hash: %v", err)
	}
	h2, err := calculateMD5(filePath2)
	if err != nil {
		return assert.Fail(t, "failed to calculate hash: %v", err)
	}
	return assert.Equal(t, h1, h2)
}

func calculateMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func assertDirIsEmpty(t *testing.T, dir string) bool {
	f, err := os.Open(dir)
	if err != nil {
		return assert.Fail(t, "failed to open dir: %v", err)
	}
	defer f.Close()

	entries, err := f.Readdir(1)
	if err != nil && err != io.EOF {
		return assert.Fail(t, "dir read error: %v", err)
	}
	return len(entries) == 0
}
