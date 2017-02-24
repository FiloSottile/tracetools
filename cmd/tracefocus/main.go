package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"

	"github.com/FiloSottile/tracetools/pprof"
	"github.com/FiloSottile/tracetools/trace"
)

var usageMessage = `Usage: tracefocus -filter=ServeHTTP [binary] trace.out

 -filter=REGEX  Only include events caused by functions that
                match REGEX, either by it being a caller, or by
                it having started the goroutine.`

var (
	filter = flag.String("filter", "", "")
)

func filterStack(Stk []*trace.Frame, re *regexp.Regexp) bool {
	for _, f := range Stk {
		if re.FindStringIndex(f.Fn) != nil {
			return true
		}
	}
	return false
}

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
		log.Fatalln("Faile to compile filter regex:", err)
	}

	events, err := pprof.LoadTrace(traceFile, programBinary)
	if err != nil {
		log.Fatal(err)
	}

	var childG = make(map[uint64]struct{})
	var lastGLen int
	for {
		for _, ev := range events {
			if ev.Type != trace.EvGoCreate {
				continue
			}
			if _, ok := childG[ev.G]; !ok && !filterStack(ev.Stk, re) {
				continue
			}
			childG[ev.Args[0]] = struct{}{}
		}
		if len(childG) == lastGLen {
			break
		}
		lastGLen = len(childG)
	}

	prof := make(map[uint64]pprof.Record)
	for _, ev := range events {
		if ev.Type != trace.EvGoBlockNet || ev.Link == nil || ev.StkID == 0 || len(ev.Stk) == 0 {
			continue
		}
		if _, ok := childG[ev.G]; !ok && !filterStack(ev.Stk, re) {
			continue
		}
		rec := prof[ev.StkID]
		rec.Stk = ev.Stk
		rec.N++
		rec.Time += ev.Link.Ts - ev.Ts
		prof[ev.StkID] = rec
	}
	if err := pprof.BuildProfile(prof).Write(os.Stdout); err != nil {
		log.Fatal(err)
	}
}
