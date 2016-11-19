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
	"log"
	"net/url"
	"os"
	"os/signal"
)

var (
	rawURL   = flag.String("seed", "", "Seed URL")
	maxDepth = flag.Int("depth", 5, "Maximum depth")
	debug    = flag.Bool("debug", false, "Debug crawler")
)

func main() {
	flag.Parse()
	seed, err := url.Parse(*rawURL)
	if err != nil {
		fmt.Println("Error: ", err.Error())
		os.Exit(1)
	}
	logger := log.New(os.Stdout, "", log.LstdFlags)
	c := crawler.NewCrawler(&crawler.Options{
		Seed:     seed,
		MaxDepth: *maxDepth,
		Matcher:  &crawler.DefaultMatcher{Seed: seed},
		Parser:   &crawler.DefaultParser{Seed: seed},
		Logger:   logger,
		Debug:    *debug,
	})
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)
	if *debug {
		if err := c.Run(shutdown); err != nil {
			log.Fatal(err)
		}
	} else {
		// TODO Send buffered log to UI
		go func() {
			if err := c.Run(shutdown); err != nil {
				log.Fatal(err)
			}
		}()
		if err = ui.UI(c, shutdown); err != nil {
			log.Fatal(err)
		}
	}
}
