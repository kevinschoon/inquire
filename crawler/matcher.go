package crawler

import "net/url"

// Matcher is used to evaluate a node and
// determine if it should be crawled.
type Matcher interface {
	Match(node *Node) bool
}

// Match is used to evaluate a Node and
// determine if it should be crawled.
type DefaultMatcher struct {
	seed *url.URL
}

// Match matches a given Node
func (m DefaultMatcher) Match(node *Node) bool {
	switch {
	// Never crawl outside of the seed domain
	case m.seed.Host != node.url.Host:
		return false
	}
	return true
}
