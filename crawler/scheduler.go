package crawler

import (
	"github.com/PuerkitoBio/fetchbot"
	"log"
	"net/url"
	"time"
)

const SchedulerInterval time.Duration = 500 * time.Millisecond

// Scheduler schedules a Node for crawling.
type Scheduler struct {
	running  bool
	schedule map[string]bool
	max      int
	in       chan *url.URL
}

func (s *Scheduler) Depth() int { return len(s.schedule) }

func (s *Scheduler) Stop() { s.running = false }

func (s *Scheduler) Start(q *fetchbot.Queue, match Matcher) error {
	s.running = true
	for s.running {
		var u *url.URL
		if len(s.schedule) >= s.max {
			break
		}
		select {
		case u = <-s.in:
			if match.Match(u) {
				key := u.String()
				log.Println("Processing: ", u.String())
				if _, ok := s.schedule[key]; ok {
					continue
				} else {
					s.schedule[key] = true
					if _, err := q.SendStringGet(key); err != nil {
						return err
					}
				}
			}
		default:
			time.Sleep(SchedulerInterval)
		}
	}
	return nil
}

func NewScheduler(max int) *Scheduler {
	return &Scheduler{
		schedule: make(map[string]bool),
		in:       make(chan *url.URL),
		max:      max,
	}
}
