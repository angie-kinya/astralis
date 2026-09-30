package reading

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

const rssBody = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
<channel>
  <title>Example</title>
  <item>
    <title>Older &amp; wiser</title>
    <link>https://example.com/older</link>
    <atom:link href="https://example.com/older" rel="self"/>
    <description><![CDATA[<p>Hello <b>world</b></p><script>alert(1)</script>]]></description>
    <pubDate>Mon, 02 Jan 2006 15:04:05 +0000</pubDate>
  </item>
  <item>
    <title>Newer</title>
    <link>https://example.com/newer</link>
    <description>Plain</description>
    <pubDate>Tue, 3 Jan 2006 15:04:05 GMT</pubDate>
  </item>
  <item>
    <title>Unsafe link</title>
    <link>javascript:alert(1)</link>
    <pubDate>Tue, 03 Jan 2006 15:04:05 +0000</pubDate>
  </item>
  <item>
    <title>No date</title>
    <link>https://example.com/undated</link>
  </item>
</channel>
</rss>`

const atomBody = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Atom</title>
  <entry>
    <title type="html">An &lt;em&gt;Atom&lt;/em&gt; entry</title>
    <link rel="replies" href="https://example.com/comments"/>
    <link rel="alternate" href="https://example.com/atom-entry"/>
    <summary type="html">&lt;p&gt;Summary &amp;amp; more&lt;/p&gt;</summary>
    <published>2006-01-04T10:00:00Z</published>
    <updated>2006-01-05T10:00:00Z</updated>
  </entry>
</feed>`

func TestParseRSS(t *testing.T) {
	items, err := parse([]byte(rssBody), Feed{Name: "Ex", Category: Business})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 items (unsafe and undated dropped), got %d: %+v", len(items), items)
	}
	first := items[0]
	if first.Title != "Older & wiser" || first.URL != "https://example.com/older" {
		t.Errorf("unexpected first item: %+v", first)
	}
	if first.Summary != "Hello world" {
		t.Errorf("summary = %q, want %q", first.Summary, "Hello world")
	}
	if first.Source != "Ex" || first.Category != Business {
		t.Errorf("source or category not set: %+v", first)
	}
	if !items[1].PublishedAt.Equal(time.Date(2006, 1, 3, 15, 4, 5, 0, time.UTC)) {
		t.Errorf("date = %v", items[1].PublishedAt)
	}
}

func TestParseAtom(t *testing.T) {
	items, err := parse([]byte(atomBody), Feed{Name: "At", Category: Tech})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Title != "An Atom entry" {
		t.Errorf("title = %q", it.Title)
	}
	if it.URL != "https://example.com/atom-entry" {
		t.Errorf("url = %q, want the alternate link", it.URL)
	}
	if it.Summary != "Summary & more" {
		t.Errorf("summary = %q", it.Summary)
	}
	if !it.PublishedAt.Equal(time.Date(2006, 1, 4, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("published = %v, want the published date over updated", it.PublishedAt)
	}
}

func TestSkipSummary(t *testing.T) {
	items, err := parse([]byte(rssBody), Feed{Name: "HN", SkipSummary: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.Summary != "" {
			t.Errorf("summary should be empty, got %q", it.Summary)
		}
	}
}

func TestStripHTML(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<p>Hello <b>world</b></p>", "Hello world"},
		{"a &lt; b &amp;&amp; c", "a < b && c"},
		{"<script>alert(1)</script>Safe", "Safe"},
		{"<style>p{color:red}</style>Text", "Text"},
		{"&lt;p&gt;Double &lt;i&gt;escaped&lt;/i&gt;&lt;/p&gt;", "Double escaped"},
		{"  lots
	of   space  ", "lots of space"},
		{"caf&eacute; &#8217;quoted&#8217;", "café ’quoted’"},
	}
	for _, c := range cases {
		if got := StripHTML(c.in); got != c.want {
			t.Errorf("StripHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	long := ""
	for i := 0; i < 60; i++ {
		long += "word "
	}
	got := truncate(long, summaryRunes)
	if n := len([]rune(got)); n > summaryRunes+1 {
		t.Errorf("truncated to %d runes, want at most %d", n, summaryRunes+1)
	}
	if got[len(got)-len("…"):] != "…" {
		t.Errorf("want an ellipsis, got %q", got)
	}
	if truncate("short", 10) != "short" {
		t.Error("short strings must be unchanged")
	}
}

func serve(body string, status *atomic.Int32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if code := int(status.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(body))
	}))
}

func okStatus() *atomic.Int32 {
	var s atomic.Int32
	s.Store(http.StatusOK)
	return &s
}

func TestFailingFeedDoesNotBreakOthers(t *testing.T) {
	good := serve(rssBody, okStatus())
	defer good.Close()
	badStatus := okStatus()
	badStatus.Store(http.StatusInternalServerError)
	bad := serve("", badStatus)
	defer bad.Close()
	garbage := serve("this is not xml at all <<<", okStatus())
	defer garbage.Close()

	s := newService([]Feed{
		{Name: "Good", URL: good.URL, Category: Business},
		{Name: "Bad", URL: bad.URL, Category: Tech},
		{Name: "Garbage", URL: garbage.URL, Category: Tech},
	}, good.Client(), time.Now)

	items, err := s.Latest()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("want the good feed's 2 items, got %d", len(items))
	}
	if items[0].Title != "Newer" {
		t.Errorf("want newest first, got %q", items[0].Title)
	}
}

func TestAllFeedsFailing(t *testing.T) {
	status := okStatus()
	status.Store(http.StatusBadGateway)
	bad := serve("", status)
	defer bad.Close()

	s := newService([]Feed{{Name: "Bad", URL: bad.URL}}, bad.Client(), time.Now)
	items, err := s.Latest()
	if !errors.Is(err, ErrUnavailable) || len(items) != 0 {
		t.Fatalf("want ErrUnavailable and no items, got %v, %d items", err, len(items))
	}
}

func TestCacheServesStaleWhenRefreshFails(t *testing.T) {
	status := okStatus()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if code := int(status.Load()); code != http.StatusOK {
			w.WriteHeader(code)
			return
		}
		_, _ = w.Write([]byte(rssBody))
	}))
	defer srv.Close()

	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := newService([]Feed{{Name: "Ex", URL: srv.URL}}, srv.Client(), func() time.Time { return now })

	if items, err := s.Latest(); err != nil || len(items) != 2 {
		t.Fatalf("first fetch: %v, %d items", err, len(items))
	}

	// Within the TTL the cache answers without touching the network.
	now = now.Add(10 * time.Minute)
	if _, err := s.Latest(); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("want 1 upstream hit inside the TTL, got %d", hits.Load())
	}

	// After the TTL the refresh fails, and the old items are served.
	now = now.Add(cacheTTL)
	status.Store(http.StatusServiceUnavailable)
	items, err := s.Latest()
	if err != nil || len(items) != 2 {
		t.Fatalf("want 2 stale items, got %v, %d items", err, len(items))
	}
	if hits.Load() != 2 {
		t.Fatalf("want a refresh attempt after the TTL, got %d hits", hits.Load())
	}
}

func TestMergeDedupesAndCaps(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	byFeed := map[string][]Item{}
	for f := 0; f < 2; f++ {
		var items []Item
		for i := 0; i < 20; i++ {
			items = append(items, Item{
				Title:       "t",
				URL:         "https://example.com/" + string(rune('a'+i)),
				PublishedAt: base.Add(time.Duration(i) * time.Hour),
			})
		}
		byFeed[string(rune('A'+f))] = items
	}
	got := merge(byFeed)
	if len(got) != 20 {
		t.Fatalf("want 20 unique urls, got %d", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].PublishedAt.After(got[i-1].PublishedAt) {
			t.Fatal("not sorted newest first")
		}
	}
}
