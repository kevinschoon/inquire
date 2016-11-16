package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/gonum/graph"
	"github.com/gonum/graph/simple"
	"github.com/gonum/graph/traverse"
)

type Node struct {
	id       int
	Response *http.Response
	Url      *url.URL
	err      error
}

func (n Node) ID() int { return n.id }

// Nodes implements a sortable array of Nodes
type Nodes []*Node

func (n Nodes) Len() int           { return len(n) }
func (n Nodes) Less(i, j int) bool { return n[i].ID() < n[j].ID() }
func (n Nodes) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }

// Scheduler schedules a Node for crawling.
type Scheduler struct {
	lock     sync.Mutex
	schedule map[string]bool
}

func (s *Scheduler) Scheduled(node *Node) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	key := node.Url.String()
	_, ok := s.schedule[key]
	return ok
}

func (s *Scheduler) Schedule(node *Node) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	key := node.Url.String()
	if _, ok := s.schedule[key]; !ok {
		s.schedule[key] = true
		return nil
	}
	return fmt.Errorf("Node %s already scheduled", key)
}

// Recorder saves the result of crawling a Node
// in a graph structure.
type Recorder struct {
	lock  sync.Mutex
	nodes map[string]int // map URL to Node ID
	graph *simple.DirectedGraph
}

func (r *Recorder) Nodes() []*Node {
	ns := r.graph.Nodes()
	nodes := make(Nodes, len(ns))
	for i, n := range ns {
		nodes[i] = n.(*Node)
	}
	sort.Sort(sort.Reverse(nodes))
	return nodes
}

// Record records the URL to the and returns a Node.
// If the URL has already been recorded then the previously
// created Node is returned.
func (r *Recorder) Record(url *url.URL, res *http.Response) *Node {
	r.lock.Lock()
	defer r.lock.Unlock()
	// Check if a Node already exists for the URL
	var node *Node
	key := url.String()
	if id, ok := r.nodes[key]; ok {
		node = r.graph.Node(id).(*Node)
	}
	if node == nil {
		node = &Node{
			id: r.graph.NewNodeID(),
			//id:  len(r.nodes),
			Url: url,
		}
		r.graph.AddNode(node)
	}
	// If the Node has no recorded response
	// and we were given one, record it.
	if node.Response == nil && res != nil {
		node.Response = res
	}
	// Save the node into the Recorder
	r.nodes[key] = node.ID()
	return node
}

// Next returns the next eligable Node to schedule
func (r *Recorder) Next(matcher Matcher) (*Node, int) {
	if len(r.graph.Nodes()) == 0 {
		return nil, 0
	}
	var depth int
	t := traverse.BreadthFirst{}
	match := t.Walk(r.graph, r.graph.Node(0), func(n graph.Node, i int) bool {
		depth = i
		current := n.(*Node)
		return matcher.Match(current)
	})
	// No nodes met our criteria
	if match == nil {
		return nil, depth
	}
	node := match.(*Node)
	return node, depth
}

// Return a fetchbot Mux
func mux(recorder *Recorder, parser Parser) *fetchbot.Mux {
	mux := fetchbot.NewMux()
	mux.Response().Method("GET").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Parent is the current response we are recording
			parent := recorder.Record(ctx.Cmd.URL(), res)
			// Attach any error to the Node
			parent.err = err
			// Array of all links contained in the response
			links := parser.Links(res)
			for _, link := range links {
				// Record the link as a Node. If the link was already
				// recorded the existing Node is returned.
				node := recorder.Record(link, nil)
				// Ignore references to the same page
				if parent.ID() != node.ID() {
					// Create an edge between the parent and all of it's linked nodes.
					recorder.graph.SetEdge(simple.Edge{F: parent, T: node, W: 0.0})
				}
			}
		}))
	return mux
}

type Crawler struct {
	Seed     *url.URL
	Recorder *Recorder
	maxDepth int
	parser   Parser
	matcher  Matcher
	fetcher  *fetchbot.Fetcher
}

// NewCrawler returns a new Crawler structure
func NewCrawler(seed *url.URL, maxDepth int) *Crawler {
	crawler := &Crawler{
		Seed:     seed,
		maxDepth: maxDepth,
		Recorder: &Recorder{
			nodes: map[string]int{},
			graph: simple.NewDirectedGraph(0.0, 0.0),
		},
		parser:  &DefaultParser{seed: seed},
		matcher: &DefaultMatcher{seed: seed},
	}
	crawler.fetcher = fetchbot.New(mux(crawler.Recorder, crawler.parser))
	crawler.fetcher.AutoClose = true
	crawler.fetcher.WorkerIdleTTL = 15 * time.Second
	return crawler
}

func (crawler *Crawler) Crawl() error {
	scheduler := &Scheduler{schedule: map[string]bool{}}
	// Start processing
	q := crawler.fetcher.Start()
	if _, err := q.SendStringGet(crawler.Seed.String()); err != nil {
		return err
	}
	running := true
	go func() {
		for running {
			node, depth := crawler.Recorder.Next(crawler.matcher)
			if depth >= crawler.maxDepth {
				fmt.Println("shutdown", crawler.maxDepth, depth)
				q.Cancel()
				break
			}
			if node != nil {
				if !scheduler.Scheduled(node) {
					scheduler.Schedule(node)
					if _, err := q.SendStringGet(node.Url.String()); err != nil {
						panic(err)
					}
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
	q.Block()
	running = false
	return nil
}
