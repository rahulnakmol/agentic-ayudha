package ui

import (
	"fmt"
	"sync"
	"time"
)

// Progress is one long step shown live: a spinner with the elapsed time at
// Rich, a start line and a result line at Plain, nothing for machines.
type Progress struct {
	p     *Printer
	mu    sync.Mutex
	label string
	start time.Time
	stop  chan struct{}
	done  chan struct{}
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Start begins a step. Call Done exactly once.
func (p *Printer) Start(label string) *Progress {
	pr := &Progress{p: p, label: label, start: p.Now()}
	switch {
	case p.Level == Plain:
		p.Println(label + " ...")
	case p.Level == Rich && !p.Static:
		pr.stop, pr.done = make(chan struct{}), make(chan struct{})
		go pr.spin()
	}
	return pr
}

func (pr *Progress) spin() {
	defer close(pr.done)
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for i := 0; ; i++ {
		pr.mu.Lock()
		fmt.Fprintf(pr.p.W, "\r\x1b[K%s %s %s", pr.p.Style(Accent, spinner[i%len(spinner)]), pr.label, pr.p.Style(Muted, elapsed(pr.p.Now().Sub(pr.start))))
		pr.mu.Unlock()
		select {
		case <-pr.stop:
			return
		case <-t.C:
		}
	}
}

// Update changes the step's label while it runs (Rich and Plain).
func (pr *Progress) Update(label string) {
	pr.mu.Lock()
	changed := label != pr.label
	pr.label = label
	pr.mu.Unlock()
	if changed && pr.p.Level == Plain {
		pr.p.Println(label + " ...")
	}
}

// Done ends the step with a status word (PASS, WARN, FAIL, …) and a detail.
func (pr *Progress) Done(status, detail string) {
	if pr.stop != nil {
		close(pr.stop)
		<-pr.done
		fmt.Fprint(pr.p.W, "\r\x1b[K")
	}
	if !pr.p.Human() {
		return
	}
	line := pr.p.Mark(status) + " " + pr.label
	if detail != "" {
		line += ": " + detail
	}
	pr.p.Println(line + " " + pr.p.Style(Muted, "("+elapsed(pr.p.Now().Sub(pr.start))+")"))
}

func elapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}
