package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"time"

	bench "github.com/fjl/goleveldb-bench"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

func main() {
	var (
		width    = flag.Int("width", 15, "with of plot in cm")
		height   = flag.Int("height", 10, "height of plot in cm")
		plotType = flag.String("plot", "bps", "type of plot (bps, abstime)")
		out      = flag.String("out", "", "output filename")
	)
	flag.Parse()
	if *out == "" {
		log.Fatal("-out is required")
	}
	reports := bench.MustReadReports(flag.Args())
	plt, err := plot.New()
	if err != nil {
		log.Fatal(err)
	}
	switch *plotType {
	case "bps":
		plotBPS(plt, reports)
	case "abstime":
		plotAbsTime(plt, reports)
	default:
		log.Fatalf("unknown plot type %q", *plotType)
	}
	if err := plt.Save(vg.Length(*width)*vg.Centimeter, vg.Length(*height)*vg.Centimeter, *out); err != nil {
		log.Fatal(err)
	}
}

// reduceEvents aggregates progress events so there are ~n total events.
// This smoothes out the line in the plot.
func reduceEvents(events []bench.Progress, n int) []bench.Progress {
	group := len(events) / n
	if group <= 1 || len(events) == 0 {
		return events
	}
	grouped := make([]bench.Progress, 0, n)
	for i, ev := range events {
		if i%group == 0 {
			grouped = append(grouped, bench.Progress{})
		}
		end := len(grouped) - 1
		grouped[end].Delta += ev.Delta
		grouped[end].Duration += ev.Duration
		grouped[end].Processed = ev.Processed
	}
	return grouped
}

// plotBPS adds BPS vs. database size plots for all reports.
func plotBPS(plt *plot.Plot, reports []bench.Report) {
	plt.X.Tick.Marker = megabyteTicks{unit: "mb"}
	plt.X.Label.Text = "database size"
	plt.Y.Label.Text = "write speed"
	plt.Y.Tick.Marker = megabyteTicks{unit: "mb/s"}
	addPlots(plt, reports, toBPSPlot)
}

// plotAbsTime adds time/size plots for all reports.
func plotAbsTime(plt *plot.Plot, reports []bench.Report) {
	plt.X.Label.Text = "time (s)"
	plt.Y.Label.Text = "database size"
	plt.Y.Tick.Marker = megabyteTicks{unit: "mb"}
	addPlots(plt, reports, toAbsTimePlot)
}

type xyFunc func([]bench.Progress) plotter.XYer

func addPlots(plt *plot.Plot, reports []bench.Report, toXY xyFunc) {
	for i, r := range reports {
		if len(r.Events) == 0 {
			log.Printf("Warning: report %s has 0 progress events", r.Name)
			continue
		}
		evs := reduceEvents(r.Events, 400)
		l, err := plotter.NewLine(toXY(evs))
		if err != nil {
			log.Fatal(err)
		}
		l.Color = plotutil.Color(i)
		plt.Add(l)
		plt.Legend.Add(r.Name, l)
	}
}

// bpsPlot plots X = db size against Y = bytes per second written.
type bpsPlot []bench.Progress

func toBPSPlot(events []bench.Progress) plotter.XYer {
	return bpsPlot(events)
}

func (p bpsPlot) Len() int {
	return len(p)
}

func (p bpsPlot) XY(i int) (float64, float64) {
	x := float64(p[i].Processed)
	return x, p[i].BPS()
}

// absTimePlot plots X = time against Y = bytes written.
type absTimePlot []bench.Progress

func toAbsTimePlot(events []bench.Progress) plotter.XYer {
	for i := range events {
		if i > 0 {
			events[i].Duration += events[i-1].Duration
		}
	}
	return absTimePlot(events)
}

func (p absTimePlot) Len() int {
	return len(p)
}

func (p absTimePlot) XY(i int) (float64, float64) {
	return float64(p[i].Duration / time.Second), float64(p[i].Processed)
}

// megabyteTicks emits axis labels corresponding to megabytes written.
type megabyteTicks struct{ unit string }

func (mt megabyteTicks) Ticks(min, max float64) (t []plot.Tick) {
	const numLabels = 5
	mag := nextPowerOfTwo(max - min)
	dist := nextPowerOfTwo(mag / numLabels)
	for s := nextPowerOfTwo(min); s < max; s += dist {
		t = append(t, plot.Tick{Value: s, Label: fmt.Sprintf("%.2f %s", s/1024/1024, mt.unit)})
	}
	t = append(t, plot.Tick{Value: max, Label: fmt.Sprintf("%.2f %s", max/1024/1024, mt.unit)})
	return t
}

func nextPowerOfTwo(f float64) float64 {
	return math.Pow(2, math.Ceil(math.Log2(f)))
}
