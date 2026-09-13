package storage

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var invalidFilenameChars = regexp.MustCompile(`[\\/:*?"<>|]`)
var multiDashes = regexp.MustCompile(`-+`)
var spaceAroundDash = regexp.MustCompile(`\s*-\s*`)
var digitsOnly = regexp.MustCompile(`^\d+$`)
var sPrefixRegex = regexp.MustCompile(`^(?i)S(\d+)$`)
var epNumRegex = regexp.MustCompile(`(?i)(?:^|S\d+[:\s]*)E?(\d+)$`)
// ttmlLangRegex extracts the BCP-47 language code from a TTML xml:lang attribute.
var ttmlLangRegex = regexp.MustCompile(`xml:lang="([^"]+)"`)
// ttmlTextTypeRegex extracts the Netflix subtitle type (SUBS = regular, SDHS = SDH/closed captions).
var ttmlTextTypeRegex = regexp.MustCompile(`nttm:textType="([^"]+)"`)

// Sanitize removes characters that are invalid in Windows and Unix file systems.
func Sanitize(s string) string {
	s = invalidFilenameChars.ReplaceAllString(s, "-")
	s = spaceAroundDash.ReplaceAllString(s, "-")
	s = multiDashes.ReplaceAllString(s, "-")
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ".- ")
	return s
}

// FormatSeasonFolder formats a season string into a clean folder name like "Season 1".
func FormatSeasonFolder(season string) string {
	season = strings.TrimSpace(season)
	if season == "" {
		return ""
	}
	if digitsOnly.MatchString(season) {
		return fmt.Sprintf("Season %s", season)
	}
	if m := sPrefixRegex.FindStringSubmatch(season); len(m) == 2 {
		return fmt.Sprintf("Season %s", m[1])
	}
	return Sanitize(season)
}

// FormatEpisodeNum formats an episode string into standardized "E01" notation.
func FormatEpisodeNum(ep string) string {
	ep = strings.TrimSpace(ep)
	if ep == "" {
		return ""
	}
	if m := epNumRegex.FindStringSubmatch(ep); len(m) == 2 {
		num, err := strconv.Atoi(m[1])
		if err == nil {
			return fmt.Sprintf("E%02d", num)
		}
	}
	return Sanitize(ep)
}

// ResolveDownloadPath determines the folder and filename for an audio track.
// Format: <downloadDir>/<ShowTitle>/[Season <N>/]<EpisodeNum> - <EpisodeTitle> - <Language>.m4a
func ResolveDownloadPath(videoURL, downloadDir, showTitle, seasonNum, epNum, epTitle, lang string) string {
	if lang == "" {
		lang = "Audio"
	}

	showFolder := Sanitize(showTitle)
	u, err := url.Parse(videoURL)
	if err != nil {
		u = &url.URL{}
	}

	if showFolder == "" || strings.EqualFold(showFolder, "Netflix") {
		if strings.HasPrefix(u.Path, "/watch") {
			videoId := strings.TrimLeft(u.Path, "/watch/")
			showFolder = "Show_" + videoId
		} else {
			showFolder = "Netflix_Downloads"
		}
	}

	targetDir := filepath.Join(downloadDir, showFolder)
	seasonFolder := FormatSeasonFolder(seasonNum)
	if seasonFolder != "" {
		targetDir = filepath.Join(targetDir, seasonFolder)
	}
	_ = os.MkdirAll(targetDir, 0755)

	var fileName string
	cleanEpNum := FormatEpisodeNum(epNum)
	cleanEpTitle := Sanitize(epTitle)

	if cleanEpNum != "" && cleanEpTitle != "" {
		fileName = fmt.Sprintf("%s - %s - %s.m4a", cleanEpNum, cleanEpTitle, lang)
	} else if cleanEpNum != "" {
		fileName = fmt.Sprintf("%s - %s.m4a", cleanEpNum, lang)
	} else if cleanEpTitle != "" {
		fileName = fmt.Sprintf("%s - %s.m4a", cleanEpTitle, lang)
	} else if showTitle != "" && !strings.EqualFold(showTitle, "Netflix") {
		fileName = fmt.Sprintf("%s - %s.m4a", Sanitize(showTitle), lang)
	} else if strings.HasPrefix(u.Path, "/watch") {
		videoId := strings.TrimLeft(u.Path, "/watch/")
		if u.Query().Has("trackId") {
			trackId := u.Query().Get("trackId")
			fileName = fmt.Sprintf("%s-%s-%s.m4a", videoId, trackId, lang)
		} else {
			fileName = fmt.Sprintf("%s-%s.m4a", videoId, lang)
		}
	} else {
		fileName = fmt.Sprintf("DL-%s.m4a", lang)
	}

	return filepath.Join(targetDir, fileName)
}

// ExtractTTMLLang parses the xml:lang attribute from a TTML subtitle body.
// Netflix embeds the language as xml:lang="de" on the root <tt> element.
// Returns an empty string if the attribute cannot be found.
func ExtractTTMLLang(body []byte) string {
	// Fast regex scan — avoids a full XML parse for a single attribute.
	m := ttmlLangRegex.FindSubmatch(body)
	if len(m) == 2 {
		return string(m[1])
	}
	return ""
}

// ExtractTTMLSubtitleType returns a short suffix to distinguish regular subtitles
// from SDH/Closed Caption tracks that share the same language code.
// Netflix marks tracks with nttm:textType="SUBS" (regular) or "SDHS" / "SDHE" (SDH/CC).
// Returns "-cc" for SDH tracks, "" for regular subtitles.
func ExtractTTMLSubtitleType(body []byte) string {
	m := ttmlTextTypeRegex.FindSubmatch(body)
	if len(m) == 2 {
		t := strings.ToUpper(string(m[1]))
		if strings.HasPrefix(t, "SDH") || t == "CC" {
			return "-cc"
		}
	}
	return ""
}

// ResolveSubtitlePath determines the folder and filename for a TTML subtitle file.
// Format: <downloadDir>/<ShowTitle>/[Season <N>/]<EpisodeNum> - <EpisodeTitle> - <Lang>.ttml
func ResolveSubtitlePath(videoURL, downloadDir, showTitle, seasonNum, epNum, epTitle, lang string) string {
	if lang == "" {
		lang = "Sub"
	}

	showFolder := Sanitize(showTitle)
	u, err := url.Parse(videoURL)
	if err != nil {
		u = &url.URL{}
	}

	if showFolder == "" || strings.EqualFold(showFolder, "Netflix") {
		if strings.HasPrefix(u.Path, "/watch") {
			videoId := strings.TrimLeft(u.Path, "/watch/")
			showFolder = "Show_" + videoId
		} else {
			showFolder = "Netflix_Downloads"
		}
	}

	targetDir := filepath.Join(downloadDir, showFolder)
	seasonFolder := FormatSeasonFolder(seasonNum)
	if seasonFolder != "" {
		targetDir = filepath.Join(targetDir, seasonFolder)
	}
	_ = os.MkdirAll(targetDir, 0755)

	var fileName string
	cleanEpNum := FormatEpisodeNum(epNum)
	cleanEpTitle := Sanitize(epTitle)

	if cleanEpNum != "" && cleanEpTitle != "" {
		fileName = fmt.Sprintf("%s - %s - %s.ttml", cleanEpNum, cleanEpTitle, lang)
	} else if cleanEpNum != "" {
		fileName = fmt.Sprintf("%s - %s.ttml", cleanEpNum, lang)
	} else if cleanEpTitle != "" {
		fileName = fmt.Sprintf("%s - %s.ttml", cleanEpTitle, lang)
	} else if showTitle != "" && !strings.EqualFold(showTitle, "Netflix") {
		fileName = fmt.Sprintf("%s - %s.ttml", Sanitize(showTitle), lang)
	} else if strings.HasPrefix(u.Path, "/watch") {
		videoId := strings.TrimLeft(u.Path, "/watch/")
		fileName = fmt.Sprintf("%s-%s.ttml", videoId, lang)
	} else {
		fileName = fmt.Sprintf("DL-%s.ttml", lang)
	}

	return filepath.Join(targetDir, fileName)
}

