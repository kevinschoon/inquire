package crawler

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/simple"
	"github.com/gonum/graph/traverse"
)

type Node struct {
	id       int
	response *http.Response
	url      *url.URL
}

func (n Node) ID() int { return n.id }

func (n Node) DOTAttributes() []dot.Attribute {
	attributes := []dot.Attribute{
		dot.Attribute{Key: "label", Value: fmt.Sprintf("\"%s\"", n.url.String())},
	}
	if n.response != nil {
		code := n.response.StatusCode
		switch {
		case code >= 200 && code < 300:
			attributes = append(attributes, dot.Attribute{Key: "color", Value: "green"})
		case code >= 400:
			attributes = append(attributes, dot.Attribute{Key: "color", Value: "red"})
		}
	}
	return attributes
}

type Recorder struct {
	lock     sync.Mutex
	depth    int
	maxDepth int
	nodes    map[string]*Node
	schedule map[string]*Node
	graph    *simple.DirectedGraph
}

// Exceeded checks to see if the recorder has exceeded
// the maximum depth we can crawl.
func (r *Recorder) Exceeded() bool { return r.depth > r.maxDepth }

// Scheduled checks to see if the URL has already been
// scheduled for crawling.
func (r *Recorder) Scheduled(node *Node) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	_, ok := r.schedule[node.url.String()]
	return ok
}

// Schedule adds the Node to the schedule.
func (r *Recorder) Schedule(node *Node) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	url := node.url.String()
	if _, ok := r.schedule[url]; ok {
		return fmt.Errorf("ERR: %s already scheduled", url)
	}
	r.nodes[url] = node
	return nil
}

// Recorded checks if the URL has already been recorded.
func (r *Recorder) Recorded(node *Node) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	// Check if the node has been saved
	if node, ok := r.nodes[node.url.String()]; ok {
		// Verify the http response has been recorded
		if node.response != nil {
			return true
		}
	}
	return false
}

// Record records the URL to the and returns a Node.
// If the URL has already been recorded then the previously
// created Node is returned.
func (r *Recorder) Record(url *url.URL, res *http.Response) *Node {
	r.lock.Lock()
	defer r.lock.Unlock()
	// Check if a Node already exists for the URL
	node, _ := r.nodes[url.String()]
	if node == nil {
		node = &Node{
			id:  len(r.nodes),
			url: url,
		}
	}
	if node.response == nil && res != nil {
		node.response = res
		r.depth++
	}
	// Save the node into the Recorder
	r.nodes[node.url.String()] = node
	return node
}

type Crawler struct {
	Seed     *url.URL
	Fetcher  *fetchbot.Fetcher
	Recorder *Recorder
	parser   Parser
	matcher  Matcher
}

// Return a fetchbot Mux
func (crawler *Crawler) mux() *fetchbot.Mux {
	mux := fetchbot.NewMux()
	mux.Response().Method("GET").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			if err != nil {
				fmt.Errorf("ERR: %s", err.Error())
				return
			}
			// Parent is the current response we are recording
			parent := crawler.Recorder.Record(ctx.Cmd.URL(), res)
			// Check to see if we've crawled enough
			if crawler.Recorder.Exceeded() {
				ctx.Q.Cancel()
				return
			}
			// Array of all links contained in the response
			links := crawler.parser.Links(res)
			for _, link := range links {
				// Record the link as a Node. If the link was already
				// recorded the existing Node is returned.
				node := crawler.Recorder.Record(link, nil)
				// Ignore references to the same page
				if parent.ID() != node.ID() {
					// Create an edge between the parent and all of it's linked nodes.
					crawler.Recorder.graph.SetEdge(simple.Edge{F: parent, T: node, W: 0.0})
				}
				// If this Node matches our criteria as crawlable
				if crawler.matcher.Match(node) {
					// Node has already be recorded, nothing to do.
					if crawler.Recorder.Recorded(node) {
						continue
					}
					// Node has already been scheduled, nothing to do.
					if crawler.Recorder.Scheduled(node) {
						continue
					}
					// Set the Node as scheduled
					if err = crawler.Recorder.Schedule(node); err != nil {
						// If we encounter an error scheduling, just continue.
						fmt.Errorf("ERR: Problem scheduling: %s", node.url.String())
						continue
					}
					// Send GET request to Crawler queue
					if _, err = ctx.Q.SendStringGet(node.url.String()); err != nil {
						// Fatal error
						panic(err)
					}
				}
			}
		}))
	return mux
}

// NewCrawler returns a new Crawler structure
func NewCrawler(seed string, depth int) (*Crawler, error) {
	parser, err := NewDefaultParser(seed)
	if err != nil {
		return nil, err
	}
	crawler := &Crawler{
		Seed:    parser.seed,
		parser:  parser,
		matcher: &DefaultMatcher{seed: parser.seed},
		Recorder: &Recorder{
			maxDepth: depth,
			nodes:    map[string]*Node{},
			schedule: map[string]*Node{},
			graph:    simple.NewDirectedGraph(0.0, 0.0),
		},
	}
	crawler.Fetcher = fetchbot.New(crawler.mux())
	crawler.Fetcher.AutoClose = true
	crawler.Fetcher.WorkerIdleTTL = 5 * time.Second
	return crawler, nil
}

func (crawler *Crawler) Crawl() error {
	q := crawler.Fetcher.Start()
	// Start processing
	if _, err := q.SendStringGet(crawler.Seed.String()); err != nil {
		return err
	}
	q.Block()
	//Dot(r.graph)
	//Traverse(r.graph)
	return nil
}

func Dot(g *simple.DirectedGraph) {
	data, err := dot.Marshal(g, "inquire", "", "", false)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}

func Traverse(g *simple.DirectedGraph) {
	t := traverse.BreadthFirst{
		Visit: func(x, y graph.Node) { fmt.Println("Visited: ", x, " ", y) },
	}
	node := g.Node(0)
	for {
		fmt.Println("Node:: ", node)
		if node == nil {
			break
		}
		node = t.Walk(g, node, nil)
	}
}
