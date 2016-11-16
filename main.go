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
	"github.com/kevinschoon/inquire/crawler"
	"github.com/kevinschoon/inquire/ui"
	"net/url"
	"os"
)

var (
	seed      = flag.String("seed", "", "seed URL")
	maxDepth  = flag.Int("depth", 5, "Maximum depth")
	disableUI = flag.Bool("disableUI", false, "Disable UI")
)

func main() {
	flag.Parse()
	u, err := url.Parse(*seed)
	if err != nil {
		fmt.Println("Error: ", err.Error())
		os.Exit(1)
	}
	c := crawler.NewCrawler(u, *maxDepth)
	go func() {
		if err = c.Crawl(); err != nil {
			fmt.Println("Error: ", err.Error())
			os.Exit(1)
		}
	}()
	if err = ui.UI(c); err != nil {
		fmt.Println("Error: ", err.Error())
		os.Exit(1)
	}

	/*
		if !*disableUI {
			if err = ui.UI(c); err != nil {
				fmt.Println("Error: ", err.Error())
				caught = err
			}
		}
	*/
}
