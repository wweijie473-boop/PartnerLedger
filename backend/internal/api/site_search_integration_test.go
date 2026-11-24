package api

import (
	"net/http"
	"net/url"
	"testing"
)

func assertSiteSearch(t *testing.T, request func(string, string, string, string, int, any), token, term, view, key, id string, want bool) {
	t.Helper()
	var result struct{ Items []siteSearchHit }
	request(http.MethodGet, "/api/search/v1/records?q="+url.QueryEscape(term), token, "", 200, &result)
	found := false
	for _, hit := range result.Items {
		link, err := url.Parse(hit.Href)
		if err != nil || link.IsAbs() || link.Host != "" {
			t.Fatalf("invalid search link: %s", hit.Href)
		}
		if link.Query().Get("view") == view && link.Query().Get(key) == id {
			found = true
		}
	}
	if result.Items == nil || len(result.Items) > 4*siteResultLimit || found != want {
		t.Fatalf("search %q returned %v for %s=%s; want found=%t", term, result.Items, key, id, want)
	}
}
