package crawler

import "net/url"

// Matcher is used to evaluate a node and
// determine if it should be crawled.
type Matcher interface {
	Match(*url.URL) bool
}

// Match is used to evaluate a Node and
// determine if it should be crawled.
type DefaultMatcher struct {
	Seed *url.URL
}

// Match matches a given Node
func (m DefaultMatcher) Match(u *url.URL) bool {
	switch {
	// Never re-crawl initial seed
	case m.Seed.String() == u.String():
		return false
	// Never crawl outside of the seed domain
	case m.Seed.Host != u.Host:
		return false
	}
	return true
}
