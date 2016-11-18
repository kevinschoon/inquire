package crawler

import (
	"net/http"
	"net/url"
)

type ResponseData struct {
	Code    int
	Headers http.Header
}

type Node struct {
	id   int
	URL  *url.URL
	Err  error
	data *ResponseData
}

// Record an HTTP Response
func (n *Node) Record(res *http.Response) {
	n.data = &ResponseData{
		Code:    res.StatusCode,
		Headers: res.Header,
	}
}

func (n Node) ID() int { return n.id }

// Nodes implements a sortable array of Nodes
type Nodes []*Node

func (n Nodes) Len() int           { return len(n) }
func (n Nodes) Less(i, j int) bool { return n[i].ID() < n[j].ID() }
func (n Nodes) Swap(i, j int)      { n[i], n[j] = n[j], n[i] }
