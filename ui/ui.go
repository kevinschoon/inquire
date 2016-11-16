/*
ui implements a cool console UI with https://github.com/gizak/termui
*/
package ui

import (
	"fmt"
	ui "github.com/gizak/termui"
	"github.com/kevinschoon/inquire/crawler"
	"math"
)

// UI runs the crawler and updates the user's console
func UI(crawler *crawler.Crawler) error {
	if err := ui.Init(); err != nil {
		return err
	}
	defer ui.Close()

	sinps := (func() []float64 {
		n := 400
		ps := make([]float64, n)
		for i := range ps {
			ps[i] = 1 + math.Sin(float64(i)/5)
		}
		return ps
	})()
	sinpsint := (func() []int {
		ps := make([]int, len(sinps))
		for i, v := range sinps {
			ps[i] = int(100*v + 10)
		}
		return ps
	})()

	title := ui.NewPar(fmt.Sprintf("Crawling %s", crawler.Seed.String()))
	title.Height = 3
	title.Width = 17
	title.X = 20
	title.BorderLabel = "Inquire"

	spark := ui.Sparkline{}
	spark.Height = 8
	spdata := sinpsint
	spark.Data = spdata[:100]
	spark.LineColor = ui.ColorCyan
	spark.TitleColor = ui.ColorWhite

	sp := ui.NewSparklines(spark)
	sp.Height = 11
	sp.BorderLabel = "Sparkline"

	lc := ui.NewLineChart()
	lc.BorderLabel = "braille-mode Line Chart"
	lc.Data = sinps
	lc.Height = 11
	lc.AxesColor = ui.ColorWhite
	lc.LineColor = ui.ColorYellow | ui.AttrBold

	gs := make([]*ui.Gauge, 3)
	for i := range gs {
		gs[i] = ui.NewGauge()
		//gs[i].LabelAlign = ui.AlignCenter
		gs[i].Height = 2
		gs[i].Border = false
		gs[i].Percent = i * 10
		gs[i].PaddingBottom = 1
		gs[i].BarColor = ui.ColorRed
	}

	responses := ui.NewList()
	responses.BorderLabel = "Crawled"
	responses.Items = []string{}
	responses.Height = ui.TermHeight()
	// build layout
	ui.Body.AddRows(
		ui.NewRow(
			ui.NewCol(6, 0, sp),
			ui.NewCol(6, 0, lc),
		),
		ui.NewRow(
			ui.NewCol(12, 0, responses),
		))

	// calculate layout
	ui.Body.Align()

	ui.Render(ui.Body)

	// Shut down on "q"
	ui.Handle("/sys/kbd/q", func(ui.Event) {
		ui.StopLoop()
	})

	ui.Handle("/timer/1s", func(e ui.Event) {
		responses.Items = []string{}
		for _, node := range crawler.Recorder.Nodes() {
			var code int
			if node.Response != nil {
				code = node.Response.StatusCode
			}
			line := fmt.Sprintf("%d - %s - %d", node.ID(), node.Url.String(), code)
			responses.Items = append(responses.Items, line)
		}
		ui.Render(ui.Body)
	})

	ui.Handle("/sys/wnd/resize", func(e ui.Event) {
		ui.Body.Width = ui.TermWidth()
		responses.Height = ui.TermHeight()
		ui.Body.Align()
		ui.Clear()
		ui.Render(ui.Body)
	})

	ui.Loop()
	return nil
}
