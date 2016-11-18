package crawler

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"net/http"
	"net/url"
)

// Parser parses an HTTP request returning specific information
type Parser interface {
	Links(*http.Response) map[string]*url.URL
}

type DefaultParser struct {
	Seed *url.URL
}

// Links returns an array of unique, normalized url.URL
func (p *DefaultParser) Links(res *http.Response) map[string]*url.URL {
	links := map[string]*url.URL{}
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		fmt.Println("ERR: Bad document, ", err)
		return links
	}
	for _, node := range doc.Find("*").Nodes {
		for _, value := range node.Attr {
			if value.Key == "href" || value.Key == "src" {
				if u := p.normalize(value.Val); u != nil {
					key := u.String()
					if _, ok := links[key]; !ok {
						links[key] = u
					}
				}
			}
		}
	}
	return links
}

func (p *DefaultParser) normalize(raw string) *url.URL {
	raw, err := purell.NormalizeURLString(raw, purell.FlagsUsuallySafeGreedy|purell.FlagRemoveFragment)
	if err != nil {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	if u.Scheme == "" {
		u.Scheme = p.Seed.Scheme
	}
	if u.Host == "" {
		u.Host = p.Seed.Host
	}
	// Never follow url params
	u.RawQuery = ""
	return u
}
