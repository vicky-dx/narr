package main

import (
	"fmt"
	"net/url"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/IljaN/narr/internal/browser"
	"github.com/IljaN/narr/internal/netflix"
)

var ErrUnsupportedUrl = fmt.Errorf("VideoURL is not a supported Netflix URL")
var ErrDownloadDirInvalid = fmt.Errorf("DownloadDir is invalid")

type Args struct {
	VideoURL    *url.URL `arg:"positional,required" help:"url of the video to download audio from. Must be a Netflix URL."`
	DownloadDir string   `arg:"positional" default:"." help:"directory where to download the audio files. Defaults to current working directory."`
	ChromeURL   *url.URL `arg:"-c, --chrome-url" default:"http://127.0.0.1:9222" help:"url of the chrome debugger."`
	Browser     string   `arg:"-b, --browser" help:"launch browser automatically (chrome, edge, brave, or executable path)."`
	ProfileDir  string   `arg:"-p, --profile-dir" help:"custom browser user profile directory. Defaults to ~/.narr/profiles/<browser>."`
	Headless    bool     `arg:"--headless" help:"run launched browser in headless mode."`
}

var Version = "0.3.0"

func (Args) Version() string {
	return Version
}

func mustParse(a *Args) {
	argParser := arg.MustParse(a)
	if err := validateArgs(a); err != nil {
		argParser.Fail(err.Error())
	}
}

func validateArgs(a *Args) error {
	if !netflix.IsURLSupported(a.VideoURL) {
		return ErrUnsupportedUrl
	}

	if a.ProfileDir != "" && a.Browser == "" {
		a.Browser = "chrome"
	}

	if a.Browser != "" && !browser.IsBrowserSupported(a.Browser) {
		if fi, err := os.Stat(a.Browser); err != nil || fi.IsDir() {
			return fmt.Errorf("unsupported browser '%s'. Supported: chrome, edge, brave, or path to executable", a.Browser)
		}
	}

	if err := processDownloadDir(a); err != nil {
		return ErrDownloadDirInvalid
	}

	return nil
}

func processDownloadDir(a *Args) error {
	var path = a.DownloadDir
	if path != "." {
		a.DownloadDir = path
		return nil
	}

	if cwd, err := os.Getwd(); err == nil {
		a.DownloadDir = cwd
		return nil
	} else {
		return err
	}
}
