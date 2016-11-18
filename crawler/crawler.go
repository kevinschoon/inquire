package crawler

import (
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/fetchbot"
)

// Return a fetchbot Mux
func mux(recorder *Recorder, parser Parser, out chan *url.URL) *fetchbot.Mux {
	mux := fetchbot.NewMux()
	mux.Response().Method("GET").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Parent is the current response we are recording
			parent := recorder.RecordResponse(res, err)
			// Array of all links contained in the response
			for _, link := range parser.Links(res) {
				// Record the link as a Node. If the link was already
				// recorded the existing Node is returned.
				recorder.RecordLink(parent, link)
				out <- link
			}
		}))
	return mux
}

type Crawler struct {
	Seed      *url.URL
	Recorder  *Recorder
	Scheduler *Scheduler
	maxDepth  int
	depth     int
	parser    Parser
	matcher   Matcher
	fetcher   *fetchbot.Fetcher
	running   bool
}

// NewCrawler returns a new Crawler structure
func NewCrawler(seed *url.URL, max int) *Crawler {
	crawler := &Crawler{
		Seed:      seed,
		Recorder:  NewRecorder(),
		Scheduler: NewScheduler(max),
		parser:    &DefaultParser{Seed: seed},
		matcher:   &DefaultMatcher{seed: seed},
	}
	crawler.fetcher = fetchbot.New(mux(crawler.Recorder, crawler.parser, crawler.Scheduler.in))
	crawler.fetcher.AutoClose = true
	crawler.fetcher.WorkerIdleTTL = 15 * time.Second
	return crawler
}

func (crawler *Crawler) Crawl() error {
	// Start processing
	q := crawler.fetcher.Start()
	if _, err := q.SendStringGet(crawler.Seed.String()); err != nil {
		return err
	}
	go func() { crawler.Scheduler.Start(q, crawler.matcher) }()
	q.Block()
	return nil
}
