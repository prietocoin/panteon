package ingest

import (
	"strings"
	"testing"
)

func TestParseRSS(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
<title>Blog</title>
<item>
  <title>Incident: cache stampede</title>
  <link>https://example.com/a</link>
  <description><![CDATA[<p>A cache key expired and caused an outage.</p>]]></description>
  <pubDate>Mon, 09 Nov 2026 12:00:00 +0000</pubDate>
</item>
<item>
  <title>New feature</title>
  <link>https://example.com/b</link>
  <description>Not an incident.</description>
</item>
</channel></rss>`)

	items := parseFeed(body)
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Title != "Incident: cache stampede" {
		t.Fatalf("title = %q", items[0].Title)
	}
	if !strings.Contains(items[0].Content, "cache key expired") {
		t.Fatalf("content = %q", items[0].Content)
	}
	if items[0].URL != "https://example.com/a" {
		t.Fatalf("url = %q", items[0].URL)
	}
}

func TestParseAtom(t *testing.T) {
	body := []byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
<title>Blog</title>
<entry>
  <id>tag:example.com,2026:1</id>
  <title>Postmortem: pool exhaustion</title>
  <link href="https://example.com/postmortem"/>
  <summary>The pool exhausted under load.</summary>
  <updated>2026-11-09T12:00:00Z</updated>
</entry>
</feed>`)

	items := parseFeed(body)
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].URL != "https://example.com/postmortem" {
		t.Fatalf("url = %q", items[0].URL)
	}
	if !strings.Contains(items[0].Content, "pool exhausted") {
		t.Fatalf("content = %q", items[0].Content)
	}
	if items[0].Published.IsZero() {
		t.Fatal("published vacío")
	}
}

func TestMatchesKeywords(t *testing.T) {
	item := Item{Title: "Incident: API outage", Content: "root cause was a bad deploy"}
	if !matchesKeywords(Source{}, item) {
		t.Fatal("debía coincidir con las keywords por defecto")
	}
	custom := Source{Keywords: []string{"database"}}
	if matchesKeywords(custom, item) {
		t.Fatal("no debía coincidir con keyword personalizada")
	}
	custom.Keywords = []string{"outage"}
	if !matchesKeywords(custom, item) {
		t.Fatal("debía coincidir con keyword personalizada")
	}
}

func TestSourceLifecycle(t *testing.T) {
	service, err := New(t.TempDir(), nil, nil, nil)
	if err != nil {
		t.Fatalf("New(): %v", err)
	}
	sourceItem, err := service.AddSource(Source{
		Type:            "rss",
		Name:            "Blog",
		URL:             "https://example.com/feed.xml",
		IntervalMinutes: 15,
		Enabled:         true,
		AutoCreate:      true,
	})
	if err != nil {
		t.Fatalf("AddSource(): %v", err)
	}
	if sourceItem.ID == "" {
		t.Fatal("ID vacío")
	}
	if len(service.ListSources()) != 1 {
		t.Fatalf("sources = %d", len(service.ListSources()))
	}
	if !service.DeleteSource(sourceItem.ID) {
		t.Fatal("DeleteSource() = false")
	}
	if len(service.ListSources()) != 0 {
		t.Fatalf("sources = %d", len(service.ListSources()))
	}
}

func TestItemKeyStable(t *testing.T) {
	item := Item{ID: "a", URL: "https://example.com", Content: "texto"}
	if itemKey(item) != itemKey(item) {
		t.Fatal("itemKey inestable")
	}
}
