package crawler

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/PuerkitoBio/fetchbot"
)

// Return a fetchbot Mux
func mux(crawler *Crawler) *fetchbot.Mux {
	mux := fetchbot.NewMux()
	mux.Response().Method("GET").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Parent is the current response we are recording
			parent := crawler.recorder.RecordResponse(res, err)
			// Array of all links contained in the response
			for _, link := range crawler.opts.Parser.Links(res) {
				// Record the link as a Node. If the link was already
				// recorded the existing Node is returned.
				crawler.recorder.RecordLink(parent, link)
				crawler.scheduler.in <- link
			}
		}))
	return mux
}

type Options struct {
	Seed     *url.URL
	Logger   *log.Logger
	MaxDepth int
	Matcher  Matcher
	Parser   Parser
}

func (c *Crawler) Block() {}

type Crawler struct {
	recorder  *Recorder
	scheduler *Scheduler
	fetcher   *fetchbot.Fetcher
	log       *log.Logger
	opts      *Options
	running   bool
}

// NewCrawler returns a new Crawler structure
func NewCrawler(opts *Options) *Crawler {
	crawler := &Crawler{
		recorder:  NewRecorder(opts),
		scheduler: NewScheduler(opts),
		opts:      opts,
		log:       opts.Logger,
	}
	crawler.fetcher = fetchbot.New(mux(crawler))
	crawler.log.Println("Crawler initialized")
	return crawler
}

func (crawler *Crawler) Run(shutdown chan os.Signal) error {
	var wg sync.WaitGroup
	wg.Add(2)
	q := crawler.fetcher.Start()
	if _, err := q.SendStringGet(crawler.opts.Seed.String()); err != nil {
		return err
	}

	go func() {
		defer wg.Done()
		crawler.scheduler.Run(q)
	}()

	go func() {
		defer wg.Done()
		q.Block()
	}()

	go func() {
		sig := <-shutdown
		crawler.scheduler.shutdown <- sig
		q.Cancel()
	}()

	wg.Wait()
	return nil
}
