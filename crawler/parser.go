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
	Links(*http.Response) []*url.URL
	Normalize(*url.URL) (*url.URL, error)
}

type DefaultParser struct {
	seed *url.URL
}

// NewParser creates a new Parser
func NewDefaultParser(seed string) (*DefaultParser, error) {
	u, err := url.Parse(seed)
	if err != nil {
		return nil, err
	}
	p := &DefaultParser{}
	u, err = url.Parse(purell.NormalizeURL(u, purell.FlagsUsuallySafeGreedy|purell.FlagRemoveFragment))
	if err != nil {
		return nil, err
	}
	p.seed = u
	return p, nil
}

// Links returns an array of unique, normalized url.URL
func (p *DefaultParser) Links(res *http.Response) []*url.URL {
	duplicates := map[string]bool{}
	links := []*url.URL{}
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		fmt.Println("ERR: Bad document, ", err)
		return links
	}
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		val, _ := s.Attr("href")
		if url, err := url.Parse(val); err == nil {
			if url.Scheme == "http" || url.Scheme == "https" || url.Scheme == "" {
				url, err = p.Normalize(url)
				if err != nil {
					fmt.Printf("ERR: Bad URL %s", err.Error())
					return
				}
				if _, ok := duplicates[url.String()]; !ok {
					duplicates[url.String()] = true
					links = append(links, url)
				}
			}
		}
	})
	return links
}

func (p *DefaultParser) Normalize(u *url.URL) (*url.URL, error) {
	raw := purell.NormalizeURL(u, purell.FlagsUsuallySafeGreedy|purell.FlagRemoveFragment)
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" {
		u.Scheme = p.seed.Scheme
	}
	if u.Host == "" {
		u.Host = p.seed.Host
	}
	u.RawQuery = ""
	return u, nil
}
