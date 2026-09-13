package media

import (
	"os"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestDecodeLanguage(t *testing.T) {
	// 'd', 'e', 'u' => [4, 5, 21]
	assert.Equal(t, "German", DecodeLanguage([3]byte{4, 5, 21}))
	// 'e', 'n', 'g' => [5, 14, 7]
	assert.Equal(t, "English", DecodeLanguage([3]byte{5, 14, 7}))
	// 'u', 'n', 'd'
	assert.Equal(t, "Audio", DecodeLanguage([3]byte{21, 14, 4}))
}

func TestProbe_IsAudio(t *testing.T) {
	testCases := []struct {
		filePath string
		isAudio  bool
	}{
		{"../../test/testdata/audio_iso6.mp4", true},
		{"../../test/testdata/audio_mp42.mp4", true},
		{"../../test/testdata/vid.mp4", false},
	}

	prober := NewProber()

	for _, tc := range testCases {
		t.Run(tc.filePath, func(t *testing.T) {
			data, err := os.ReadFile(tc.filePath)
			if err != nil {
				t.Fatal(err)
			}

			result, err := prober.Probe(data)
			assert.NoError(t, err)
			assert.Equal(t, tc.isAudio, result.IsAudio)
		})
	}
}

func TestProbe_Info(t *testing.T) {
	prober := NewProber()

	data, err := os.ReadFile("../../test/testdata/audio_iso6.mp4")
	assert.NoError(t, err)

	result, err := prober.Probe(data)
	assert.NoError(t, err)
	assert.True(t, result.IsAudio)
	assert.True(t, result.IsXHEAAC)
	assert.Equal(t, "iso6", result.MajorBrand)
	assert.Equal(t, "Ukrainian", result.Language)

	data, err = os.ReadFile("../../test/testdata/audio_mp42.mp4")
	assert.NoError(t, err)

	result, err = prober.Probe(data)
	assert.NoError(t, err)
	assert.True(t, result.IsAudio)
	assert.False(t, result.IsXHEAAC)
	assert.Equal(t, "mp42", result.MajorBrand)
	assert.Equal(t, "Audio", result.Language)
}
