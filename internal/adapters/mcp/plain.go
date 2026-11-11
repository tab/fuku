package mcp

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"fuku/internal/adapters/terminal"
)

// Banner label and call line column width constants
const (
	bannerLabelWidth = 9
	callLabelWidth   = 20
	callOutcomeWidth = 9
)

// Plain writes the banner and one line per tool call to stderr free of escape sequences, one write at a time
type Plain struct {
	stderr io.Writer
	mu     sync.Mutex
}

// NewPlain creates the plain console for a pipe, such as the MCP log of the host
func NewPlain(stderr io.Writer) *Plain {
	return &Plain{stderr: stderr}
}

// Open has nothing to prepare
func (p *Plain) Open() {}

// Banner writes the protocol, the project, the instance state, the tools and how to stop
func (p *Plain) Banner(banner Banner) {
	label := func(name string) string {
		return terminal.PadRight(name, bannerLabelWidth)
	}

	p.write(
		"[+] fuku mcp · stdio · protocol "+banner.Protocol,
		fmt.Sprintf(" %s %s · %s", label("project"), banner.Project, banner.Fingerprint),
		fmt.Sprintf(" %s %s · %s", label("instance"), banner.State, banner.Detail),
		fmt.Sprintf(" %s %s", label("tools"), strings.Join(banner.Tools, " ")),
		"Serving MCP · close stdin or Ctrl+C to stop",
	)
}

// Call writes the line of one tool call: the target, the outcome or the failure, and the duration
func (p *Plain) Call(call Call) {
	target := terminal.PadRight(call.Target, callLabelWidth) + " "

	if call.Failure != "" {
		p.write(fmt.Sprintf(" ✘ %s%s%s · %s", target, strings.Repeat(" ", callOutcomeWidth+1), call.Failure, call.Elapsed()))

		return
	}

	p.write(fmt.Sprintf(" ✔ %s%s %s", target, terminal.PadRight(call.Outcome, callOutcomeWidth), call.Elapsed()))
}

// Close has nothing to release
func (p *Plain) Close() {}

// write prints the lines in one write, so concurrent calls never interleave
func (p *Plain) write(lines ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	fmt.Fprint(p.stderr, strings.Join(lines, "\n")+"\n")
}
