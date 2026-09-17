// Package sse reads server-sent event streams.
//
// It exists because the CLI and the client library both read swarf's
// revocation firehose, and an event stream whose events can be megabytes
// needs the same buffer in both.
package sse

import (
	"bufio"
	"io"
	"strings"
)

// The size bounds on a single event. A revocation event carries a CID per
// delegation in the chain, so it runs well past bufio.Scanner's default
// 64 KiB cap; MaxEventBytes is where a Scanner gives up with bufio.ErrTooLong
// rather than growing without limit.
const (
	initialEventBytes = 64 * 1024
	MaxEventBytes     = 4 * 1024 * 1024
)

// A Scanner reads events from a server-sent event stream. Successive calls to
// Scan advance to the next event, whose fields Event and Data then report.
// Scanning stops at the end of the stream or at the first read error, which
// Err returns.
//
// Fields other than event and data -- id, retry, comments -- are ignored, and
// an event carrying no data is not reported, which is what the event stream
// format calls for:
// https://html.spec.whatwg.org/multipage/server-sent-events.html#event-stream-interpretation
type Scanner struct {
	lines *bufio.Scanner
	event string
	data  string
}

// NewScanner returns a Scanner reading the event stream in r.
func NewScanner(r io.Reader) *Scanner {
	lines := bufio.NewScanner(r)
	lines.Buffer(make([]byte, initialEventBytes), MaxEventBytes)
	return &Scanner{lines: lines}
}

// Scan advances to the next event, reporting whether there was one.
func (s *Scanner) Scan() bool {
	var event string
	var data []string
	for s.lines.Scan() {
		line := s.lines.Text()
		if line == "" {
			if len(data) > 0 {
				s.event, s.data = event, strings.Join(data, "\n")
				return true
			}
			event = ""
			continue
		}
		if value, ok := strings.CutPrefix(line, "event:"); ok {
			event = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(line, "data:"); ok {
			data = append(data, strings.TrimPrefix(value, " "))
		}
	}
	s.event, s.data = "", ""
	return false
}

// Event is the type of the event Scan read, empty if the event named none.
func (s *Scanner) Event() string { return s.event }

// Data is the data of the event Scan read, its data lines joined by newlines.
func (s *Scanner) Data() string { return s.data }

// Err returns the first read error Scan met, if any. It is bufio.ErrTooLong
// when one event exceeded MaxEventBytes.
func (s *Scanner) Err() error { return s.lines.Err() }
