/*
ui implements a cool console UI with https://github.com/gizak/termui
*/
package ui

import (
	"fmt"
	"github.com/gizak/termui"
	"github.com/kevinschoon/inquire/crawler"
	"os"
	"time"
)

// UI runs the crawler and updates the user's console
func UI(c *crawler.Crawler, shutdown chan os.Signal) error {
	if err := termui.Init(); err != nil {
		return err
	}
	defer termui.Close()

	// Current section
	currentSection := "nodes"

	title := termui.NewPar("[Q](bg-white,fg-black)uit [N](bg-white,fg-black)odes [S](bg-white,fg-black)chedule [L](bg-white,fg-black)ogs")
	title.Height = 3
	title.Border = true

	// Top Left Panel
	leftStatus := termui.NewList()
	leftStatus.Items = []string{}
	leftStatus.Height = 8

	// Top Right Panel
	rightStatus := termui.NewList()
	rightStatus.Items = []string{}
	rightStatus.Height = 8

	// Main section
	mainSection := termui.NewList()
	mainSection.BorderLabel = ""
	mainSection.Items = []string{}
	mainSection.Height = termui.TermHeight()
	// build layout
	termui.Body.AddRows(
		termui.NewRow(
			termui.NewCol(12, 0, title),
		),
		termui.NewRow(
			termui.NewCol(6, 0, leftStatus),
			termui.NewCol(6, 0, rightStatus),
		),
		termui.NewRow(
			termui.NewCol(12, 0, mainSection),
		),
	)

	// calculate layout
	termui.Body.Align()

	termui.Render(termui.Body)

	termui.Handle("/timer/1s", func(e termui.Event) {
		status := c.Status()
		//leftStatus.Items = LeftStatus(c)
		rightStatus.Items = RightStatus(status)
		mainSection.Items, mainSection.BorderLabel = MainSection(status, currentSection)
		termui.Render(termui.Body)
	})

	termui.Handle("/sys/wnd/resize", func(e termui.Event) {
		termui.Body.Width = termui.TermWidth()
		//mainSection.Height = termui.TermHeight()
		termui.Body.Align()
		termui.Clear()
		termui.Render(termui.Body)
	})

	// Quit
	termui.Handle("/sys/kbd/q", func(termui.Event) {
		shutdown <- os.Interrupt
		termui.StopLoop()
	})

	// Display Nodes
	termui.Handle("/sys/kbd/n", func(termui.Event) {
		currentSection = "nodes"
		status := c.Status()
		mainSection.Items, mainSection.BorderLabel = MainSection(status, currentSection)
		termui.Render(termui.Body)
	})

	// Display Schedule
	termui.Handle("/sys/kbd/s", func(termui.Event) {
		currentSection = "schedule"
		status := c.Status()
		mainSection.Items, mainSection.BorderLabel = MainSection(status, currentSection)
		termui.Render(termui.Body)
	})

	// Display Logs
	termui.Handle("/sys/kbd/l", func(termui.Event) {
		currentSection = "logs"
		status := c.Status()
		mainSection.Items, mainSection.BorderLabel = MainSection(status, currentSection)
		termui.Render(termui.Body)
	})

	termui.Loop()
	return nil
}

// RightStatus returns status data for the top right panel
func RightStatus(status *crawler.Status) []string {
	s := make([]string, 3)
	if status.Running {
		s[0] = "Scheduler: [online](fg-green)"
	} else {
		s[0] = "Scheduler: [offline](fg-red)"
	}
	s[1] = fmt.Sprintf("Depth: %d", status.Depth)
	s[2] = fmt.Sprintf("Seed: %s", status.Options.Seed.String())
	return s
}

// LeftStatus returns status data for the top left panel
func LeftStatus(s *crawler.Status) []string {
	// TODO
	return []string{"", "", "", ""}
}

// MainSection updates the main section of the terminal
// It returns the section data and boader label.
func MainSection(status *crawler.Status, section string) ([]string, string) {
	var label string
	results := []string{}
	switch section {
	case "nodes":
		label = "STATUS DURATION SIZE    URL"
		for _, node := range status.Nodes {
			var (
				status   = "[?](fg-white)"
				duration = time.Duration(0)
				length   = int64(0)
				url      = node.URL.String()
			)
			if data := node.Data(); data != nil {
				switch {
				case data.Code >= 200 && data.Code < 300:
					status = fmt.Sprintf("[%d](fg-green)", data.Code)
				case data.Code > 400:
					status = fmt.Sprintf("[%d](fg-red)", data.Code)
				}
				length = data.Length
				duration = data.Duration
			}
			results = append(results, fmt.Sprintf("%s    %s        %d          %s", status, duration.String(), length, url))
		}
	case "schedule":
		label = "SCHEDULE"
		results = status.Schedule
	case "logs":
		label = "LOGS"
		results = status.Logs
	}
	return results, label
}
