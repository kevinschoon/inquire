/*
Inquire is a lightweight web crawler and http discovery tool.
I am often asked "what is this website running on?" or "who is hosting that?".
To answer such a question I need to employ multiple commandline tools
to inspect host headers, perform DNS lookup, inspect source code,
etc. Inquire attempts to implement general purpose functionality for
answering these questions.
*/
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"github.com/gonum/graph"
	"github.com/gonum/graph/encoding/dot"
	"github.com/gonum/graph/simple"
	"github.com/gonum/graph/traverse"
)

var (
	seed     = flag.String("seed", "", "seed URL")
	maxDepth = flag.Int("depth", 5, "Maximum depth")
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
	nodes    map[string]*Node
	schedule map[string]*Node
	graph    *simple.DirectedGraph
	parser   *Parser
	matcher  *Match
}

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
	}
	// Save the node into the Recorder
	r.nodes[node.url.String()] = node
	return node
}

// Match is used to evaluate a Node and
// determine if it should be crawled.
type Match struct {
	seed *url.URL
}

// Match matches a given Node
func (m Match) Match(node *Node) bool {
	switch {
	// Never crawl outside of the seed domain
	case m.seed.Host != node.url.Host:
		return false
	}
	return true
}

// Parser parses an HTTP request returning specific information
type Parser struct {
	seed *url.URL
}

// NewParser creates a new Parser
func NewParser(seed string) (*Parser, error) {
	u, err := url.Parse(seed)
	if err != nil {
		return nil, err
	}
	p := &Parser{}
	s, err := p.Normalize(u)
	if err != nil {
		return nil, err
	}
	p.seed = s
	return p, nil
}

// Links returns an array of unique, normalized url.URL
func (p *Parser) Links(res *http.Response) []*url.URL {
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

func (p *Parser) Normalize(u *url.URL) (*url.URL, error) {
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

func main() {
	flag.Parse()

	// Create the muxer
	mux := fetchbot.NewMux()

	parser, err := NewParser(*seed)
	if err != nil {
		panic(err)
	}
	// Recorder
	r := &Recorder{
		nodes:    map[string]*Node{},
		schedule: map[string]*Node{},
		graph:    simple.NewDirectedGraph(0.0, 0.0),
		parser:   parser,
		matcher:  &Match{seed: parser.seed},
	}
	*seed = parser.seed.String()

	// Handle all errors the same
	mux.HandleErrors(fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
	}))

	mux.Response().Method("GET").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			if err != nil {
				fmt.Errorf("ERR: %s", err.Error())
				return
			}
			r.depth++
			// Parent is the current response we are recording
			parent := r.Record(ctx.Cmd.URL(), res)
			// Array of all links contained in the response
			links := r.parser.Links(res)
			for _, link := range links {
				// Record the link as a Node. If the link was already
				// recorded the existing Node is returned.
				node := r.Record(link, nil)
				// Ignore references to the same page
				if parent.ID() != node.ID() {
					// Create an edge between the parent and all of it's linked nodes.
					r.graph.SetEdge(simple.Edge{F: parent, T: node, W: 0.0})
				}
				// If this Node matches our criteria as crawlable
				if r.matcher.Match(node) {
					// Node has already be recorded, nothing to do.
					if r.Recorded(node) {
						continue
					}
					// Node has already been scheduled, nothing to do.
					if r.Scheduled(node) {
						continue
					}
					// Set the Node as scheduled
					if err = r.Schedule(node); err != nil {
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

	f := fetchbot.New(mux)
	//f.DisablePoliteness = true
	//f.CrawlDelay = 1 * time.Microsecond
	f.AutoClose = true
	f.WorkerIdleTTL = 5 * time.Second

	q := f.Start()

	go func() {
		for {
			if r.depth > *maxDepth {
				if err := q.Cancel(); err != nil {
					fmt.Println("ERR: ", err)
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()

	// Start processing
	if _, err = q.SendStringGet(*seed); err != nil {
		panic(err)
	}
	q.Block()
	//Dot(r.graph)
	Traverse(r.graph)

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
