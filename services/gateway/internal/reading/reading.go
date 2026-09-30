// Package reading serves a short list of recent finance, technology and
// business headlines for the Resources page.
//
// The feeds are a fixed allowlist in this file. Nothing about which feed to
// fetch ever comes from a request, so there is no way to point the server at
// an arbitrary URL. Nothing about the person asking is recorded either: the
// list is the same for everyone, and clicks go straight to the publisher.
package reading

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type Category string

const (
	Finance  Category = "finance"
	Tech     Category = "tech"
	Business Category = "business"
)

type Feed struct {
	Name     string
	URL      string
	Category Category
	// Some aggregators put boilerplate ("Article URL: ... Points: 12") where a
	// summary would be. Better no summary than that one.
	SkipSummary bool
	// Drop items whose URL contains this, for sections of an otherwise free
	// site that sit behind a paywall.
	ExcludePath string
}

// Feeds is the allowlist. Only publishers whose articles open without a
// paywall or sign-up belong here. The Verge was left out because it now
// meters articles.
var Feeds = []Feed{
	{Name: "BBC Business", URL: "https://feeds.bbci.co.uk/news/business/rss.xml", Category: Business},
	{Name: "CNBC", URL: "https://www.cnbc.com/id/10000664/device/rss/rss.html", Category: Finance, ExcludePath: "/pro/"},
	{Name: "BBC Technology", URL: "https://feeds.bbci.co.uk/news/technology/rss.xml", Category: Tech},
	{Name: "Ars Technica", URL: "https://feeds.arstechnica.com/arstechnica/index", Category: Tech},
	{Name: "TechCabal", URL: "https://techcabal.com/feed/", Category: Business},
	{Name: "Rest of World", URL: "https://restofworld.org/feed/latest/", Category: Tech},
	{Name: "Hacker News", URL: "https://hnrss.org/frontpage", Category: Tech, SkipSummary: true},
}

type Item struct {
	Title       string    `json:"title"`
	Source      string    `json:"source"`
	Category    Category  `json:"category"`
	PublishedAt time.Time `json:"publishedAt"`
	Summary     string    `json:"summary"`
	URL         string    `json:"url"`
}

const (
	feedTimeout  = 5 * time.Second
	maxBody      = 2 << 20 // 2 MB
	cacheTTL     = 30 * time.Minute
	failRetry    = time.Minute // after every feed failed, try again sooner than the TTL
	maxItems     = 30
	summaryRunes = 220
)

var ErrUnavailable = errors.New("no feed could be read")

type Service struct {
	feeds []Feed
	http  *http.Client
	ttl   time.Duration
	now   func() time.Time

	refresh sync.Mutex // one refresh at a time; waiting callers then read the fresh cache

	mu     sync.Mutex // guards the fields below
	byFeed map[string][]Item
	next   time.Time
}

func New() *Service {
	return newService(Feeds, &http.Client{Timeout: feedTimeout}, time.Now)
}

func newService(feeds []Feed, client *http.Client, now func() time.Time) *Service {
	return &Service{
		feeds:  feeds,
		http:   client,
		ttl:    cacheTTL,
		now:    now,
		byFeed: make(map[string][]Item),
	}
}

// Latest returns up to 30 items, newest first. Each feed keeps its last good
// items, so a feed that fails a refresh is served stale rather than dropped.
// The error is non-nil only when there is nothing at all to serve.
func (s *Service) Latest() ([]Item, error) {
	if s.due() {
		s.refresh.Lock()
		if s.due() { // another caller may have refreshed while we waited
			s.fetchAll()
		}
		s.refresh.Unlock()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	items := merge(s.byFeed)
	if len(items) == 0 {
		return items, ErrUnavailable
	}
	return items, nil
}

func (s *Service) due() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return !s.now().Before(s.next)
}

func (s *Service) fetchAll() {
	type result struct {
		feed  Feed
		items []Item
		err   error
	}
	results := make(chan result, len(s.feeds))
	var wg sync.WaitGroup
	for _, f := range s.feeds {
		wg.Add(1)
		go func(f Feed) {
			defer wg.Done()
			// Detached from the request: the cache is shared, so one caller
			// hanging up must not cancel a refresh everyone else is waiting on.
			ctx, cancel := context.WithTimeout(context.Background(), feedTimeout)
			defer cancel()
			items, err := s.fetch(ctx, f)
			results <- result{f, items, err}
		}(f)
	}
	wg.Wait()
	close(results)

	s.mu.Lock()
	defer s.mu.Unlock()
	ok := 0
	for r := range results {
		if r.err != nil || len(r.items) == 0 {
			continue // keep whatever this feed gave us last time
		}
		s.byFeed[r.feed.URL] = r.items
		ok++
	}
	if ok == 0 {
		s.next = s.now().Add(failRetry)
	} else {
		s.next = s.now().Add(s.ttl)
	}
}

func (s *Service) fetch(ctx context.Context, f Feed) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Astralis/1.0 (reading list)")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", f.Name, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("%s: body over %d bytes", f.Name, maxBody)
	}
	return parse(body, f)
}

// ------------------------------------------------------------------ parsing

type rssDoc struct {
	Items []struct {
		Title       string   `xml:"title"`
		Links       []string `xml:"link"` // atom:link also matches here, with no text
		GUID        string   `xml:"guid"`
		Description string   `xml:"description"`
		PubDate     string   `xml:"pubDate"`
		Date        string   `xml:"date"` // dc:date
	} `xml:"channel>item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
}

type atomDoc struct {
	Entries []struct {
		Title     string     `xml:"title"`
		Links     []atomLink `xml:"link"`
		Summary   string     `xml:"summary"`
		Content   string     `xml:"content"`
		Published string     `xml:"published"`
		Updated   string     `xml:"updated"`
	} `xml:"entry"`
}

// parse reads RSS 2.0 or Atom, decided by the root element.
func parse(body []byte, f Feed) ([]Item, error) {
	root, err := rootName(body)
	if err != nil {
		return nil, err
	}
	var items []Item
	switch root {
	case "rss":
		var doc rssDoc
		if err := decode(body, &doc); err != nil {
			return nil, err
		}
		for _, it := range doc.Items {
			link := firstURL(append(it.Links, it.GUID)...)
			items = appendItem(items, f, it.Title, link, it.Description, firstNonEmpty(it.PubDate, it.Date))
		}
	case "feed":
		var doc atomDoc
		if err := decode(body, &doc); err != nil {
			return nil, err
		}
		for _, e := range doc.Entries {
			var alt, other []string
			for _, l := range e.Links {
				if l.Rel == "" || l.Rel == "alternate" {
					alt = append(alt, l.Href)
				}
				other = append(other, l.Href)
			}
			link := firstURL(append(alt, other...)...)
			items = appendItem(items, f, e.Title, link, firstNonEmpty(e.Summary, e.Content), firstNonEmpty(e.Published, e.Updated))
		}
	default:
		return nil, fmt.Errorf("%s: not RSS or Atom (root %q)", f.Name, root)
	}
	return items, nil
}

func appendItem(items []Item, f Feed, title, link, summary, date string) []Item {
	title = truncate(StripHTML(title), 200)
	published, ok := parseDate(date)
	if title == "" || link == "" || !ok {
		return items // undated items cannot be ordered, and linkless ones cannot be read
	}
	if f.ExcludePath != "" && strings.Contains(link, f.ExcludePath) {
		return items
	}
	it := Item{Title: title, Source: f.Name, Category: f.Category, PublishedAt: published.UTC(), URL: link}
	if !f.SkipSummary {
		it.Summary = truncate(StripHTML(summary), summaryRunes)
	}
	return append(items, it)
}

func rootName(body []byte) (string, error) {
	d := newDecoder(body)
	for {
		tok, err := d.Token()
		if err != nil {
			return "", err
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

func decode(body []byte, v any) error { return newDecoder(body).Decode(v) }

func newDecoder(body []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(body))
	d.Strict = false // real feeds contain bare ampersands and HTML entities
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charsetReader
	return d
}

// charsetReader covers the encodings feeds actually declare. Anything else
// fails that one feed, which the caller already tolerates.
func charsetReader(label string, r io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "utf-8", "utf8", "us-ascii", "ascii":
		return r, nil
	case "iso-8859-1", "latin1", "latin-1":
		b, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		runes := make([]rune, len(b))
		for i, c := range b {
			runes[i] = rune(c) // Latin-1 bytes are the first 256 code points
		}
		return strings.NewReader(string(runes)), nil
	}
	return nil, fmt.Errorf("unsupported charset %q", label)
}

// firstURL returns the first absolute http(s) URL. Anything else, such as a
// javascript: link from a compromised feed, is never passed to the browser.
func firstURL(candidates ...string) string {
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		u, err := url.Parse(c)
		if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
			return u.String()
		}
	}
	return ""
}

var dateLayouts = []string{
	time.RFC1123Z, time.RFC1123, time.RFC3339, time.RFC3339Nano,
	"Mon, 2 Jan 2006 15:04:05 -0700", "Mon, 2 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 -0700", "Mon, 02 Jan 2006 15:04 -0700",
}

func parseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ------------------------------------------------------------------ text

var (
	reBlocks = regexp.MustCompile(`(?is)<(script|style)\b.*?</(script|style)\s*>`)
	reTags   = regexp.MustCompile(`<[a-zA-Z/!?][^<>]*>`)
)

// StripHTML turns feed HTML into plain text. It runs the tag pass twice
// because some feeds escape their HTML a second time, so the first unescape
// reveals another layer of tags.
func StripHTML(s string) string {
	s = reBlocks.ReplaceAllString(s, " ")
	s = reTags.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = reBlocks.ReplaceAllString(s, " ")
	s = reTags.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// truncate cuts at a word boundary and never splits a character.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n]
	if i := strings.LastIndexByte(string(r), ' '); i > n/2 {
		return strings.TrimRight(string(r)[:i], " ,;:.") + "…"
	}
	return strings.TrimRight(string(r), " ,;:.") + "…"
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

// merge flattens every feed's items, drops repeated URLs, and keeps the
// newest 30.
func merge(byFeed map[string][]Item) []Item {
	seen := make(map[string]bool)
	all := []Item{}
	for _, items := range byFeed {
		for _, it := range items {
			if seen[it.URL] {
				continue
			}
			seen[it.URL] = true
			all = append(all, it)
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].PublishedAt.After(all[j].PublishedAt) })
	if len(all) > maxItems {
		all = all[:maxItems]
	}
	return all
}
