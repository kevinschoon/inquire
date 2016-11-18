package crawler

import (
	"github.com/PuerkitoBio/fetchbot"
	"log"
	"net/url"
	"os"
)

// Scheduler schedules a Node for crawling.
type Scheduler struct {
	schedule map[string]bool
	matcher  Matcher
	max      int
	in       chan *url.URL
	shutdown chan os.Signal
	log      *log.Logger
}

func (s *Scheduler) Depth() int { return len(s.schedule) }

func (s *Scheduler) Run(q *fetchbot.Queue) {
loop:
	for {
		var u *url.URL
		// Check depth
		depth := s.Depth()
		if depth >= s.max {
			s.log.Printf("Maximum depth reached: %d\n", depth)
			break
		}
		select {
		case u = <-s.in:
			if s.matcher.Match(u) {
				key := u.String()
				if _, ok := s.schedule[key]; ok {
					s.log.Printf("url %s was already scheduled, ignoring\n", key)
					continue
				} else {
					s.schedule[key] = true
					s.log.Printf("scheduling url %s for crawling\n", key)
					if _, err := q.SendStringGet(key); err != nil {
						s.log.Fatal(err)
					}
				}
			}
		case <-s.shutdown:
			s.log.Printf("Shutting down scheduler")
			break loop
		}
	}
}

func NewScheduler(opts *Options) *Scheduler {
	return &Scheduler{
		shutdown: make(chan os.Signal, 1),
		schedule: make(map[string]bool),
		in:       make(chan *url.URL),
		max:      opts.MaxDepth,
		matcher:  opts.Matcher,
		log:      opts.Logger,
	}
}
