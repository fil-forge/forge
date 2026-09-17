package sse_test

import (
	"bufio"
	"strings"
	"testing"

	"github.com/fil-forge/swarf/internal/sse"
	"github.com/stretchr/testify/require"
)

type event struct {
	name string
	data string
}

func TestScannerReadsEvents(t *testing.T) {
	for _, test := range []struct {
		name   string
		stream string
		want   []event
	}{
		{
			name:   "one named event",
			stream: "event: revocation\ndata: {}\n\n",
			want:   []event{{name: "revocation", data: "{}"}},
		},
		{
			name:   "data lines are joined by newlines",
			stream: "event: revocation\ndata: one\ndata: two\n\n",
			want:   []event{{name: "revocation", data: "one\ntwo"}},
		},
		{
			// The service writes an id line before every revocation.
			name:   "other fields and comments are ignored",
			stream: ": keep-alive\nid: bafy\nretry: 1000\nevent: revocation\ndata: {}\n\n",
			want:   []event{{name: "revocation", data: "{}"}},
		},
		{
			name:   "successive events",
			stream: "event: revocation\ndata: one\n\nevent: error\ndata: two\n\n",
			want:   []event{{name: "revocation", data: "one"}, {name: "error", data: "two"}},
		},
		{
			// Per the event stream format, an event with no data is not
			// dispatched -- and it must not reset the next event either.
			name:   "an event with no data is not reported",
			stream: "event: revocation\n\nevent: revocation\ndata: {}\n\n",
			want:   []event{{name: "revocation", data: "{}"}},
		},
		{
			name:   "data with no event name is reported unnamed",
			stream: "data: {}\n\n",
			want:   []event{{name: "", data: "{}"}},
		},
		{
			// A truncated stream: the caller reconnects rather than acting on
			// half an event.
			name:   "an unterminated event is not reported",
			stream: "event: revocation\ndata: {}\n",
			want:   nil,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			scanner := sse.NewScanner(strings.NewReader(test.stream))
			var got []event
			for scanner.Scan() {
				got = append(got, event{name: scanner.Event(), data: scanner.Data()})
			}
			require.NoError(t, scanner.Err())
			require.Equal(t, test.want, got)
		})
	}
}

func TestScannerRejectsAnOversizedEvent(t *testing.T) {
	stream := "event: revocation\ndata: " + strings.Repeat("a", sse.MaxEventBytes+1) + "\n\n"
	scanner := sse.NewScanner(strings.NewReader(stream))

	require.False(t, scanner.Scan())
	require.ErrorIs(t, scanner.Err(), bufio.ErrTooLong,
		"an event past MaxEventBytes must be reported, not silently dropped")
}

func TestScannerReadsPastTheDefaultBufioLimit(t *testing.T) {
	// The whole reason this package exists: bufio.Scanner's own default caps a
	// token at 64 KiB, and a revocation with a long delegation path exceeds it.
	data := strings.Repeat("a", 512*1024)
	scanner := sse.NewScanner(strings.NewReader("event: revocation\ndata: " + data + "\n\n"))

	require.True(t, scanner.Scan())
	require.Equal(t, "revocation", scanner.Event())
	require.Equal(t, data, scanner.Data())
	require.NoError(t, scanner.Err())
}
