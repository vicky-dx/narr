package netflix

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsURLSupported(t *testing.T) {
	tests := []struct {
		urlStr   string
		expected bool
	}{
		{"https://www.netflix.com/watch/81405170", true},
		{"https://netflix.com/watch/81405170", true},
		{"https://subdomain.netflix.com/title/123", true},
		{"https://www.hotstar.com/in/shows/123", false},
		{"https://www.youtube.com/watch?v=123", false},
	}

	for _, tt := range tests {
		u, err := url.Parse(tt.urlStr)
		assert.NoError(t, err)
		assert.Equal(t, tt.expected, IsURLSupported(u), "testing %s", tt.urlStr)
	}
}

func TestIsMediaURL(t *testing.T) {
	assert.True(t, IsMediaURL("https://ipv4-c001-bom001.ix.nflxvideo.net/range/0-123456?o=1"))
	assert.False(t, IsMediaURL("https://www.netflix.com/api/shakti/v1/metadata"))
	assert.False(t, IsMediaURL("https://ipv4-c001-bom001.ix.nflxvideo.net/manifest.mpd"))
}

func TestToDownloadableURL(t *testing.T) {
	c := &Client{}
	input := "https://ipv4-c001-bom001.ix.nflxvideo.net/range/0-123456?o=1&v=2"
	expected := "https://ipv4-c001-bom001.ix.nflxvideo.net?o=1&v=2"

	res := c.ToDownloadableURL(input)
	assert.Equal(t, expected, res)
}
