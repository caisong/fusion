package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/0x2E/fusion/internal/config"
	"golang.org/x/text/encoding/simplifiedchinese"
)

type validateFeedBody struct {
	Data struct {
		Feeds []struct {
			Title string `json:"title"`
			Link  string `json:"link"`
		} `json:"feeds"`
	} `json:"data"`
}

func postValidateFeed(t *testing.T, h *Handler, url string) (int, validateFeedBody) {
	t.Helper()

	r := newTestRouter()
	r.POST("/api/feeds/validate", h.validateFeed)
	w := performRequest(
		r,
		http.MethodPost,
		"/api/feeds/validate",
		strings.NewReader(fmt.Sprintf(`{"url":%q}`, url)),
		map[string]string{"Content-Type": "application/json"},
	)

	var body validateFeedBody
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil && w.Code == http.StatusOK {
		t.Fatalf("decode response: %v", err)
	}
	return w.Code, body
}

// The add-feed dialog calls validate before creating a feed and blocks the
// creation when no feed is found, so a malformed document must yield an empty
// feeds list rather than an error or a fabricated entry.
func TestValidateFeedMalformedXMLReturnsNoFeeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprint(w, `<?xml version="1.0"?><rss><channel><title>Broken`)
	}))
	defer server.Close()

	h := &Handler{config: &config.Config{AllowPrivateFeeds: true}}

	code, body := postValidateFeed(t, h, server.URL)
	if code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", code)
	}
	if len(body.Data.Feeds) != 0 {
		t.Fatalf("expected 0 feeds for malformed XML, got %d: %+v", len(body.Data.Feeds), body.Data.Feeds)
	}
}

func TestValidateFeedValidRSSReturnsFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve the feed only at its real path so the finder's common-path
		// guesses (index.xml, atom.xml, ...) miss like they would in production.
		if r.URL.Path != "/feed.xml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Tech Daily</title><link>https://example.com</link>
<item><guid>g1</guid><title>Item</title><link>https://example.com/1</link></item>
</channel></rss>`)
	}))
	defer server.Close()

	h := &Handler{config: &config.Config{AllowPrivateFeeds: true}}

	code, body := postValidateFeed(t, h, server.URL+"/feed.xml")
	if code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", code)
	}
	if len(body.Data.Feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d: %+v", len(body.Data.Feeds), body.Data.Feeds)
	}
	if body.Data.Feeds[0].Title != "Tech Daily" {
		t.Fatalf("expected title %q, got %q", "Tech Daily", body.Data.Feeds[0].Title)
	}
	if body.Data.Feeds[0].Link != server.URL+"/feed.xml" {
		t.Fatalf("expected link %q, got %q", server.URL+"/feed.xml", body.Data.Feeds[0].Link)
	}
}

func TestValidateFeedUnreachableURLReturnsNoFeeds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	url := server.URL
	server.Close()

	h := &Handler{config: &config.Config{AllowPrivateFeeds: true}}

	code, body := postValidateFeed(t, h, url)
	if code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", code)
	}
	if len(body.Data.Feeds) != 0 {
		t.Fatalf("expected 0 feeds for unreachable URL, got %d", len(body.Data.Feeds))
	}
}

// Discovery probes common paths on the host, so a feed elsewhere on the host
// makes /feeds/validate return a non-empty list even though the entered URL is
// not a feed itself. The frontend must therefore match the entered URL against
// the returned links instead of treating a non-empty list as approval.
func TestValidateFeedReturnsCommonPathFeedForInvalidPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/feed.xml":
			w.Header().Set("Content-Type", "application/rss+xml")
			_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"><channel><title>Tech Daily</title><link>https://example.com</link>
<item><guid>g1</guid><title>Item</title><link>https://example.com/1</link></item>
</channel></rss>`)
		default:
			w.Header().Set("Content-Type", "application/xml")
			_, _ = fmt.Fprint(w, `<?xml version="1.0"?><rss><channel><title>Broken`)
		}
	}))
	defer server.Close()

	h := &Handler{config: &config.Config{AllowPrivateFeeds: true}}

	code, body := postValidateFeed(t, h, server.URL+"/broken.xml")
	if code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", code)
	}
	found := false
	for _, feed := range body.Data.Feeds {
		if feed.Link == server.URL+"/feed.xml" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected discovery to surface %q for a broken entered path, got %+v", server.URL+"/feed.xml", body.Data.Feeds)
	}
}

// A pasted feed URL must be fetched and parsed as-is with a single request.
// Handing it to feedfinder additionally probes ~20 well-known paths on the
// host, and WAF-protected sites rate-limit that burst, so discovery of the
// valid entered URL can come back empty. The GBK payload mirrors Discuz
// forum feeds (e.g. 52pojie), which declare a non-UTF-8 encoding.
func TestValidateFeedDirectGBKURLSingleRequest(t *testing.T) {
	title, err := simplifiedchinese.GBK.NewEncoder().String("吾爱破解 - 52pojie.cn - 『原创发布区』")
	if err != nil {
		t.Fatalf("encode gbk title: %v", err)
	}

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		if r.URL.Path != "/forum.php" || r.URL.Query().Get("mod") != "rss" || r.URL.Query().Get("fid") != "2" {
			// Behave like the forum's WAF: any probe other than the real
			// feed URL gets throttled.
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="gbk"?>
<rss version="2.0"><channel><title>%s</title><link>https://www.52pojie.cn/forum-2-1.html</link>
<item><title>Item</title><link>https://www.52pojie.cn/thread-1-1-1.html</link></item>
</channel></rss>`, title)
	}))
	defer server.Close()

	h := &Handler{config: &config.Config{AllowPrivateFeeds: true}}
	feedURL := server.URL + "/forum.php?mod=rss&fid=2"

	code, body := postValidateFeed(t, h, feedURL)
	if code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", code)
	}
	if len(body.Data.Feeds) != 1 {
		t.Fatalf("expected 1 feed, got %d: %+v", len(body.Data.Feeds), body.Data.Feeds)
	}
	if body.Data.Feeds[0].Title != "吾爱破解 - 52pojie.cn - 『原创发布区』" {
		t.Fatalf("expected decoded GBK title, got %q", body.Data.Feeds[0].Title)
	}
	if body.Data.Feeds[0].Link != feedURL {
		t.Fatalf("expected link %q, got %q", feedURL, body.Data.Feeds[0].Link)
	}
	if got := atomic.LoadInt32(&requests); got != 1 {
		t.Fatalf("expected exactly 1 request for a direct feed URL, got %d", got)
	}
}
