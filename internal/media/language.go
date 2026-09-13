package media

import "strings"

// LanguageMap maps ISO-639-2 three-letter codes to user-friendly English names.
var LanguageMap = map[string]string{
	"eng": "English",
	"deu": "German",
	"ger": "German",
	"hin": "Hindi",
	"spa": "Spanish",
	"fre": "French",
	"fra": "French",
	"ita": "Italian",
	"jpn": "Japanese",
	"kor": "Korean",
	"zho": "Chinese",
	"chi": "Chinese",
	"tam": "Tamil",
	"tel": "Telugu",
	"rus": "Russian",
	"por": "Portuguese",
	"ukr": "Ukrainian",
}

// DecodeLanguage decodes the 3-byte packed ISO-639-2 language code from an MP4 mdhd box.
func DecodeLanguage(lang [3]byte) string {
	b := make([]byte, 3)
	for i := 0; i < 3; i++ {
		if lang[i] > 0 && lang[i] <= 26 {
			b[i] = lang[i] + 0x60
		} else {
			b[i] = lang[i]
		}
	}
	code := strings.TrimSpace(string(b))
	if name, found := LanguageMap[code]; found {
		return name
	}
	if code != "" && code != "und" && code != "   " {
		return strings.ToUpper(code)
	}
	return "Audio"
}
