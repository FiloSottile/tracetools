package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"time"

	"github.com/FiloSottile/tracetools/pprof"
	"github.com/FiloSottile/tracetools/trace"
)

var usageMessage = `Usage: tracehist -filter=ServeHTTP [binary] trace.out

 -filter=REGEX   Syscall events matching this regex will be plotted.`

var (
	filter = flag.String("filter", "", "")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, usageMessage)
		os.Exit(2)
	}
	flag.Parse()

	// Go 1.7 traces embed symbol info and does not require the binary.
	// But we optionally accept binary as first arg for Go 1.5 traces.
	var programBinary, traceFile string
	switch flag.NArg() {
	case 1:
		traceFile = flag.Arg(0)
	case 2:
		programBinary = flag.Arg(0)
		traceFile = flag.Arg(1)
	default:
		flag.Usage()
	}

	if *filter == "" {
		flag.Usage()
	}
	re, err := regexp.Compile(*filter)
	if err != nil {
		log.Fatalln("Failed to compile filter regex:", err)
	}

	events, err := pprof.LoadTrace(traceFile, programBinary)
	if err != nil {
		log.Fatal(err)
	}

	durations := make(map[uint64][]int64)
	names := make(map[uint64]string)
	for _, ev := range events {
		if ev.Type != trace.EvGoSysCall || ev.Link == nil || ev.StkID == 0 || len(ev.Stk) == 0 {
			continue
		}
		if re.FindStringIndex(ev.Stk[0].Fn) == nil {
			continue
		}
		d := durations[ev.StkID]
		d = append(d, ev.Link.Ts-ev.Ts)
		durations[ev.StkID] = d
		names[ev.StkID] = ev.Stk[0].Fn
	}

	for StkID, dur := range durations {
		hist := newHistogram()
		for _, d := range dur {
			hist.Observe(time.Duration(d))
		}
		fmt.Println("Histogram of durations for", names[StkID])
		hist.Print(false)
	}
}
