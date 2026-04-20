package main

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Profiler struct {
	enabled   bool
	outPath   string
	events    []ProfileEvent
	startTime time.Time
}

type ProfileEvent struct {
	Name     string
	Duration time.Duration
}

func NewProfiler(enabled bool, outPath string) *Profiler {
	return &Profiler{
		enabled:   enabled,
		outPath:   outPath,
		startTime: time.Now(),
	}
}

func (p *Profiler) Record(name string, d time.Duration) {
	if !p.enabled {
		return
	}
	p.events = append(p.events, ProfileEvent{Name: name, Duration: d})
}

func (p *Profiler) Track(name string) func() {
	if !p.enabled {
		return func() {}
	}
	start := time.Now()
	return func() {
		p.Record(name, time.Since(start))
	}
}

func (p *Profiler) Write() error {
	if !p.enabled {
		return nil
	}
	var out io.Writer
	if p.outPath == "-" {
		out = os.Stdout
	} else {
		f, err := os.Create(p.outPath)
		if err != nil {
			return err
		}
		defer f.Close()
		out = f
	}

	total := time.Since(p.startTime)
	fmt.Fprintf(out, "Site Generation Profile Report\n")
	fmt.Fprintf(out, "==============================\n")
	for _, ev := range p.events {
		fmt.Fprintf(out, "%-40s %v\n", ev.Name+":", ev.Duration)
	}
	fmt.Fprintf(out, "------------------------------\n")
	fmt.Fprintf(out, "%-40s %v\n", "Total Time:", total)
	return nil
}
