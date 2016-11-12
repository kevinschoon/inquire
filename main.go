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
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/PuerkitoBio/fetchbot"
	"github.com/PuerkitoBio/goquery"
	"github.com/PuerkitoBio/purell"
	"github.com/gonum/graph/encoding/dot"
	graph "github.com/gonum/graph/simple"
)

var seed = flag.String("seed", "", "seed URL")

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
	lock    sync.Mutex
	counter int
	nodes   map[string]Node
	graph   *graph.DirectedGraph
}

func (r *Recorder) Node(url string) *Node {
	r.lock.Lock()
	defer r.lock.Unlock()
	if node, ok := r.nodes[url]; ok {
		return &node
	}
	return nil
}

func (r *Recorder) Add(url *url.URL, res *http.Response) *Node {
	r.lock.Lock()
	defer r.lock.Unlock()
	if node, ok := r.nodes[url.String()]; ok {
		return &node
	}
	r.counter++
	node := Node{
		id:       r.counter,
		response: res,
		url:      url,
	}
	r.nodes[url.String()] = node
	return &node
}

func main() {
	flag.Parse()

	// Parse the provided seed
	u, err := url.Parse(*seed)
	if err != nil {
		log.Fatal(err)
	}
	u = Normalize(u)
	// Create the muxer
	mux := fetchbot.NewMux()

	// Recorder
	r := &Recorder{
		nodes: map[string]Node{},
		graph: graph.NewDirectedGraph(0.0, 0.0),
	}

	// Handle all errors the same
	mux.HandleErrors(fetchbot.HandlerFunc(func(ctx *fetchbot.Context, res *http.Response, err error) {
		fmt.Printf("[ERR] %s %s - %s\n", ctx.Cmd.Method(), ctx.Cmd.URL(), err)
	}))

	mux.Response().Method("GET").Handler(fetchbot.HandlerFunc(
		func(ctx *fetchbot.Context, res *http.Response, err error) {
			// Process the body to find the links
			parent := r.Add(res.Request.URL, res)
			for _, link := range Links(res) {
				if link.Host != u.Host {
					node := r.Add(link, nil)
					r.graph.SetEdge(graph.Edge{F: parent, T: node, W: 0.0})
					continue
				}
				if node := r.Node(link.String()); node == nil {
					if _, err = ctx.Q.SendStringGet(link.String()); err != nil {
						panic(err)
					}
				}
			}
		}))

	f := fetchbot.New(mux)
	f.AutoClose = true
	f.WorkerIdleTTL = 5 * time.Second

	// Start processing
	q := f.Start()
	if _, err = q.SendStringGet(Normalize(u).String()); err != nil {
		panic(err)
	}
	q.Block()
	data, err := dot.Marshal(r.graph, "inquire", "", "", false)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))
}

// Links returns an array of unique, normalized url.URL
func Links(res *http.Response) []*url.URL {
	filtered := map[string]bool{}
	links := []*url.URL{}
	doc, err := goquery.NewDocumentFromResponse(res)
	if err != nil {
		fmt.Println("ERR: Bad document, ", err)
		return links
	}
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		val, _ := s.Attr("href")
		if url, err := url.Parse(val); err == nil {
			url = Normalize(url)
			if _, ok := filtered[url.String()]; !ok {
				filtered[url.String()] = true
				links = append(links, url)
			}
		}
	})
	return links
}

func Normalize(u *url.URL) *url.URL {
	raw := purell.NormalizeURL(u, purell.FlagsUsuallySafeGreedy)
	if u, err := url.Parse(raw); err == nil {
		return u
	}
	panic(u.String())
}
