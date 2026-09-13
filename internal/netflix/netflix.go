package netflix

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/protocol/network"
	"github.com/mafredri/cdp/protocol/page"
	"github.com/mafredri/cdp/protocol/runtime"
	"golang.org/x/sync/errgroup"

	"github.com/IljaN/narr/internal/downloader"
)

// EventType defines events emitted during Netflix playback.
type EventType int

const (
	MediaUrlReceivedEvent EventType = iota
	SubtitleUrlReceivedEvent
	NavigatedEvent
)

// Event represents a stream or navigation event from Netflix.
type Event struct {
	Type    EventType
	Payload string
}

// IsURLSupported returns true if the URL belongs to Netflix.
func IsURLSupported(u *url.URL) bool {
	return strings.Contains(strings.ToLower(u.Host), "netflix.com")
}

// Client interacts with Netflix via Chrome DevTools Protocol.
type Client struct {
	chrome *cdp.Client
}

// New creates a new Netflix client instance.
func New(chrome *cdp.Client) *Client {
	return &Client{chrome: chrome}
}

// NavigateTo tells the browser to open the target Netflix URL.
func (c *Client) NavigateTo(ctx context.Context, targetURL string) error {
	navArgs := page.NewNavigateArgs(targetURL)
	_, err := c.chrome.Page.Navigate(ctx, navArgs)
	return err
}

// Listen starts monitoring network responses and internal document navigations.
func (c *Client) Listen(ctx context.Context) <-chan Event {
	responseReceived, err := c.chrome.Network.ResponseReceived(ctx)
	if err != nil {
		log.Fatal(err)
	}

	navigated, err := c.chrome.Page.NavigatedWithinDocument(ctx)
	if err != nil {
		log.Fatal(err)
	}

	eg := errgroup.Group{}
	eg.Go(func() error { return c.chrome.Network.Enable(ctx, network.NewEnableArgs()) })
	eg.Go(func() error { return c.chrome.Page.Enable(ctx) })
	if err := eg.Wait(); err != nil {
		log.Fatal(err)
	}

	events := make(chan Event)
	go func() {
		defer navigated.Close()
		defer responseReceived.Close()
		for {
			select {
			case <-navigated.Ready():
				ev, err := navigated.Recv()
				if err != nil {
					log.Fatal(err)
				}
				events <- Event{Type: NavigatedEvent, Payload: ev.URL}

			case <-responseReceived.Ready():
				ev, err := responseReceived.Recv()
				if err != nil {
					log.Fatal(err)
				}
				if IsMediaURL(ev.Response.URL) {
					events <- Event{Type: MediaUrlReceivedEvent, Payload: ev.Response.URL}
				} else if IsSubtitleURL(ev.Response.URL, ev.Response.MimeType) {
					events <- Event{Type: SubtitleUrlReceivedEvent, Payload: ev.Response.URL}
				}
			}
		}
	}()

	return events
}

// GetMetadata extracts show title, season, episode number, and episode title from Netflix DOM.
func (c *Client) GetMetadata(ctx context.Context) downloader.Metadata {
	var meta downloader.Metadata

	evalArgs := runtime.NewEvaluateArgs(`(() => {
		// Wake up Netflix player controls if hidden
		window.dispatchEvent(new MouseEvent('mousemove', { bubbles: true, clientX: 200, clientY: 200 }));

		let showTitle = '', seasonNum = '', episodeNum = '', episodeTitle = '';
		try {
			const c = window.netflix.falcorCache, vp = window.netflix.appContext.state.playerApp.getAPI().videoPlayer;
			const v = c.videos[vp.getVideoPlayerBySessionId(vp.getAllPlayerSessionIds()[0]).getMovieId()];
			if (v && v.summary && v.summary.value) {
				const s = v.summary.value;
				if (s.season != null && s.type === 'episode') seasonNum = String(s.season);
				if (s.episode != null && s.type === 'episode') episodeNum = String(s.episode);
			}
		} catch(e) {}

		// 1. Evidence overlay (visible in pause/overlay for both movies and series)
		const evTitle = document.querySelector('[data-uia="evidence-overlay"] h2, [data-uia="evidence-overlay"] h1, [data-uia="evidence-overlay-season-title"]');
		if (evTitle) showTitle = evTitle.textContent.trim();

		// 2. Video title in bottom controls bar
		const el = document.querySelector('[data-uia="video-title"]');
		if (el) {
			const ch = el.children;
			if (ch.length >= 1) {
				showTitle = ch[0].textContent.trim();
				if (ch.length >= 2) {
					const ep = ch[1].textContent.trim();
					const sm = ep.match(/S(?:eason\s*)?(\d+)/i);
					if (sm) seasonNum = sm[1];
					const em = ep.match(/E(?:pisode\s*)?(\d+)/i);
					if (em) episodeNum = em[1];
					else if (!episodeNum) episodeNum = ep;
				}
				if (ch.length >= 3) episodeTitle = ch[2].textContent.trim();
			} else if (el.textContent.trim()) {
				// Movie: title directly in el.textContent without child elements
				if (!showTitle) showTitle = el.textContent.trim();
			}
		}

		if (!showTitle) {
			const h4 = document.querySelector('.video-title h4, .video-title');
			if (h4) showTitle = h4.textContent.trim();
		}

		if (!episodeTitle) {
			const et = document.querySelector('[data-uia="evidence-overlay-episode-title"]');
			if (et) {
				const text = et.textContent.trim();
				const parts = text.split(':');
				if (parts.length > 1) {
					episodeTitle = parts[0].trim();
					if (!episodeNum) {
						const em = parts[1].match(/(\d+)/);
						if (em) episodeNum = em[1];
					}
				} else {
					episodeTitle = text;
				}
			}
		}

		if (!showTitle) {
			const dt = document.title.replace(/\s*[-|]\s*Netflix.*$/i, '').trim();
			if (dt && !dt.toLowerCase().includes('netflix')) showTitle = dt;
		}

		return { showTitle, seasonNum, episodeNum, episodeTitle };
	})()`).SetReturnByValue(true)

	for i := 0; i < 15; i++ {
		reply, err := c.chrome.Runtime.Evaluate(ctx, evalArgs)
		if err == nil && reply != nil && reply.Result.Value != nil {
			if err := json.Unmarshal(reply.Result.Value, &meta); err == nil {
				if meta.ShowTitle != "" && !strings.EqualFold(meta.ShowTitle, "Netflix") {
					return meta
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}

	return meta
}

// IsMediaURL checks if an intercepted request is an audio/video media segment.
func IsMediaURL(u string) bool {
	return strings.Contains(u, "/range/0-")
}

// IsSubtitleURL checks if an intercepted request is a TTML subtitle file.
// Netflix delivers subtitles as text/xml (TTML/IMSC) from the same nflxvideo.net CDN
// but WITHOUT a /range/ path — they are complete single-file downloads.
func IsSubtitleURL(u, mimeType string) bool {
	return strings.Contains(strings.ToLower(mimeType), "text/xml") &&
		strings.Contains(u, "nflxvideo.net")
}

// ToDownloadableURL converts a range URL to a full downloadable resource URL.
func (c *Client) ToDownloadableURL(audioURL string) string {
	u, err := url.Parse(audioURL)
	if err != nil {
		return audioURL
	}
	u.Path = ""
	return u.String()
}
