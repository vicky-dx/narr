package downloader

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Metadata holds show and episode metadata.
type Metadata struct {
	ShowTitle    string `json:"showTitle"`
	SeasonNum    string `json:"seasonNum"`
	EpisodeNum   string `json:"episodeNum"`
	EpisodeTitle string `json:"episodeTitle"`
}

// IsEmpty checks if metadata has no title information.
func (m Metadata) IsEmpty() bool {
	return m.ShowTitle == "" && m.SeasonNum == "" && m.EpisodeNum == "" && m.EpisodeTitle == ""
}

// Task represents a pending or active media download job.
type Task struct {
	SrcURL       string
	VideoURL     string
	DownloadDir  string
	FullFilePath string
	Meta         Metadata
	// SubtitleLang holds the BCP-47 language code extracted from the TTML xml:lang attribute.
	// Only set for subtitle tasks.
	SubtitleLang string
}

// Status is implemented by all download status event types.
type Status interface {
	TaskId() string
	Task() Task
}

type taskInfo struct {
	taskId string
	task   Task
}

func (d *taskInfo) TaskId() string {
	return d.taskId
}

func (d *taskInfo) Task() Task {
	return d.task
}

// Queuing is emitted when a download job is accepted into the queue.
type Queuing struct {
	*taskInfo
}

// Begin is emitted when a download has begun after probing.
type Begin struct {
	*taskInfo
}

// Skipped is emitted when a target file already exists and download is skipped.
type Skipped struct {
	*taskInfo
}

// Finished is emitted when a download completes successfully.
type Finished struct {
	bytesReceived int64
	duration      time.Duration
	*taskInfo
}

// BytesReceived returns the total number of bytes downloaded.
func (f *Finished) BytesReceived() int64 {
	return f.bytesReceived
}

// Duration returns the elapsed download time.
func (f *Finished) Duration() time.Duration {
	return f.duration
}

func newTaskId() string {
	bytes := make([]byte, 6)
	_, _ = rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
