package crawler

import (
	"github.com/gonum/graph/simple"
	"github.com/kevinschoon/fetchbot"
	"log"
	"net/http"
	"net/url"
	"sort"
	"sync"
)

func NewRecorder(opts *Options) *Recorder {
	return &Recorder{
		nodes: map[string]int{},
		graph: simple.NewDirectedGraph(0.0, 0.0),
		log:   opts.Logger,
	}
}

// Recorder saves the result of crawling a Node
// in a graph structure.
type Recorder struct {
	lock  sync.Mutex
	nodes map[string]int // map URL to Node ID
	graph *simple.DirectedGraph
	log   *log.Logger
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

// RecordResponse creates (or updates) a node with the http.Response
func (r *Recorder) RecordResponse(ctx *fetchbot.Context, res *http.Response, err error) *Node {
	r.lock.Lock()
	defer r.lock.Unlock()
	var node *Node
	key := res.Request.URL.String()
	r.log.Printf("recording response for %s\n", key)
	if id, ok := r.nodes[key]; ok {
		node = r.graph.Node(id).(*Node)
	}
	if node == nil {
		node = &Node{
			id:  r.graph.NewNodeID(),
			URL: res.Request.URL,
			Err: err,
		}
		r.graph.AddNode(node)
	}
	node.Record(ctx.Duration, res)
	r.nodes[key] = node.ID()
	return node
}

// RecordLink creates (or updates) a node and adds an Edge to the Graph
func (r *Recorder) RecordLink(parent *Node, url *url.URL) *Node {
	r.lock.Lock()
	defer r.lock.Unlock()
	var node *Node
	key := url.String()
	r.log.Printf("recording link %s --> %s\n", parent.URL.String(), url.String())
	if id, ok := r.nodes[key]; ok {
		node = r.graph.Node(id).(*Node)
	}
	if node == nil {
		node = &Node{
			id:  r.graph.NewNodeID(),
			URL: url,
		}
		r.graph.AddNode(node)
	}
	if parent.ID() != node.ID() {
		r.graph.SetEdge(simple.Edge{F: parent, T: node, W: 0.0})
	}
	r.nodes[key] = node.ID()
	return node
}
