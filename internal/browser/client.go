package browser

import (
	"context"
	"log"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/rpcc"
)

// Connect establishes an RPC connection to Chrome DevTools Protocol target.
func Connect(ctx context.Context, url string) (*cdp.Client, error) {
	devt := devtool.New(url)
	pt, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		pt, err = devt.Create(ctx)
		if err != nil {
			return nil, err
		}
	}

	conn, err := rpcc.DialContext(ctx, pt.WebSocketDebuggerURL)
	if err != nil {
		return nil, err
	}

	return cdp.NewClient(conn), nil
}

// ConnectUntilSuccess repeatedly attempts to connect to Chrome with dual-stack (IPv4 & IPv6) fallback.
func ConnectUntilSuccess(ctx context.Context, url string, retryInterval time.Duration) *cdp.Client {
	urls := []string{url}
	if url == "http://127.0.0.1:9222" {
		urls = append(urls, "http://localhost:9222")
	}

	for {
		for _, u := range urls {
			client, err := Connect(ctx, u)
			if err == nil {
				return client
			}
		}
		log.Printf("Cannot connect to Chrome at %s. Ensure Chrome is running in debug mode. Retrying in %v...", url, retryInterval)
		time.Sleep(retryInterval)
	}
}
