package media

import (
	"bytes"
	"github.com/abema/go-mp4"
)

const xHEAACOTI = 42 // Audio Object Type Identifier for xHE-AAC

// ProbeResult contains media information extracted from an MP4 header.
type ProbeResult struct {
	IsAudio    bool
	MajorBrand string
	IsXHEAAC   bool
	Language   string
}

// Prober defines the contract for probing media byte streams.
type Prober interface {
	Probe(header []byte) (ProbeResult, error)
}

// DefaultProber implements Prober using abema/go-mp4.
type DefaultProber struct{}

// NewProber returns a default MP4 prober.
func NewProber() Prober {
	return &DefaultProber{}
}

// Probe parses the MP4 header and checks if it contains a supported audio stream.
func (p *DefaultProber) Probe(header []byte) (ProbeResult, error) {
	result := ProbeResult{
		Language: "Audio",
	}

	info, err := mp4.Probe(bytes.NewReader(header))
	if err != nil {
		return result, err
	}

	majorBrand := string(info.MajorBrand[:])

	// Netflix delivers audio in mp42 or iso6 containers
	if (majorBrand == "mp42" || majorBrand == "iso6") && len(info.Tracks) == 1 && info.Tracks[0].Codec == mp4.CodecMP4A {
		result.IsAudio = true
		result.IsXHEAAC = info.Tracks[0].MP4A.AudOTI == xHEAACOTI
		result.MajorBrand = majorBrand

		// Extract mdhd box for language metadata
		boxes, _ := mp4.ExtractBoxesWithPayload(bytes.NewReader(header), nil, []mp4.BoxPath{
			{mp4.BoxTypeMoov(), mp4.BoxTypeTrak(), mp4.BoxTypeMdia(), mp4.BoxTypeMdhd()},
		})
		for _, b := range boxes {
			if mdhd, ok := b.Payload.(*mp4.Mdhd); ok {
				result.Language = DecodeLanguage(mdhd.Language)
				break
			}
		}
	}

	return result, nil
}
