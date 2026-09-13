package storage

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitize(t *testing.T) {
	assert.Equal(t, "Show-Title", Sanitize("Show:Title"))
	assert.Equal(t, "Episode-Name", Sanitize(`Episode "Name"?`))
	assert.Equal(t, "A-B-C", Sanitize("A/B\\C*"))
}

func TestFormatSeasonFolder(t *testing.T) {
	assert.Equal(t, "Season 1", FormatSeasonFolder("1"))
	assert.Equal(t, "Season 2", FormatSeasonFolder("Season 2"))
	assert.Equal(t, "Season 3", FormatSeasonFolder("S3"))
	assert.Equal(t, "Part 1", FormatSeasonFolder("Part 1"))
	assert.Equal(t, "", FormatSeasonFolder(""))
}

func TestFormatEpisodeNum(t *testing.T) {
	assert.Equal(t, "E01", FormatEpisodeNum("1"))
	assert.Equal(t, "E01", FormatEpisodeNum("E1"))
	assert.Equal(t, "E01", FormatEpisodeNum("E01"))
	assert.Equal(t, "E12", FormatEpisodeNum("12"))
	assert.Equal(t, "E05", FormatEpisodeNum("S2:E5"))
	assert.Equal(t, "", FormatEpisodeNum(""))
}

func TestResolveDownloadPath_WithSeason(t *testing.T) {
	tmp := t.TempDir()
	path := ResolveDownloadPath(
		"https://www.netflix.com/watch/81748888?trackId=264293154",
		tmp,
		"DANG!",
		"1",
		"1",
		"Pilot",
		"English",
	)

	expectedSuffix := filepath.Join("DANG!", "Season 1", "E01 - Pilot - English.m4a")
	assert.True(t, strings.HasSuffix(path, expectedSuffix), "expected path to end with %s, got %s", expectedSuffix, path)
}

func TestResolveDownloadPath_Metadata(t *testing.T) {
	tmp := t.TempDir()
	path := ResolveDownloadPath(
		"https://www.netflix.com/watch/81748894?trackId=200257858",
		tmp,
		"DANG!",
		"Season 1",
		"E7",
		"Oh No, I'm Still Talking",
		"German",
	)

	expectedSuffix := filepath.Join("DANG!", "Season 1", "E07 - Oh No, I'm Still Talking - German.m4a")
	assert.True(t, strings.HasSuffix(path, expectedSuffix), "expected path to end with %s, got %s", expectedSuffix, path)
}

func TestResolveDownloadPath_Fallback(t *testing.T) {
	tmp := t.TempDir()
	path := ResolveDownloadPath(
		"https://www.netflix.com/watch/81405170?trackId=254015180",
		tmp,
		"",
		"",
		"",
		"",
		"English",
	)

	assert.True(t, strings.Contains(path, "Show_81405170"))
	assert.True(t, strings.HasSuffix(path, ".m4a"))
}

func TestAtomicWriteAndExists(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "test_audio.m4a")

	assert.False(t, Exists(target))

	header := []byte("header")
	body := bytes.NewReader([]byte("body_content"))

	written, err := WriteAtomic(target, header, body)
	assert.NoError(t, err)
	assert.Equal(t, int64(18), written)

	assert.True(t, Exists(target))

	data, err := os.ReadFile(target)
	assert.NoError(t, err)
	assert.Equal(t, "headerbody_content", string(data))
}
