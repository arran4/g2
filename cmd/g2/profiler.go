package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

type Profiler struct {
	mu        sync.Mutex
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
	p.mu.Lock()
	defer p.mu.Unlock()
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
		defer func() { _ = f.Close() }()
		out = f
	}

	total := time.Since(p.startTime)
	_, _ = fmt.Fprintf(out, "Site Generation Profile Report\n")
	_, _ = fmt.Fprintf(out, "==============================\n")
	for _, ev := range p.events {
		_, _ = fmt.Fprintf(out, "%-40s %v\n", ev.Name+":", ev.Duration)
	}
	_, _ = fmt.Fprintf(out, "------------------------------\n")
	_, _ = fmt.Fprintf(out, "%-40s %v\n", "Total Time:", total)
	return nil
}
