package sse

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net/http"
	"sync"

	ginsse "github.com/gin-contrib/sse"
)

type Event = ginsse.Event

// Forward wraps upstream.Body so callers can read the original SSE stream while
// completed events are decoded and sent to the returned channel. The event
// channel is closed when forwarding finishes.
func Forward(upstream *http.Response) <-chan Event {
	events := make(chan Event, 16)
	reader, writer := io.Pipe()
	body := upstream.Body
	upstream.Body = &forwardedBody{
		PipeReader: reader,
		upstream:   body,
	}

	go func() {
		defer close(events)
		err := forward(body, writer, events)
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	return events
}

type forwardedBody struct {
	*io.PipeReader
	upstream io.Closer
	once     sync.Once
	err      error
}

func (b *forwardedBody) Close() error {
	b.once.Do(func() {
		b.err = errors.Join(b.PipeReader.Close(), b.upstream.Close())
	})
	return b.err
}

func forward(upstream io.ReadCloser, downstream *io.PipeWriter, events chan<- Event) error {
	defer upstream.Close()

	reader := bufio.NewReader(upstream)
	var frame bytes.Buffer
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			if _, writeErr := downstream.Write(line); writeErr != nil {
				return writeErr
			}
			frame.Write(line)
			if isBlankLine(line) {
				if err := sendEvents(frame.Bytes(), events); err != nil {
					return err
				}
				frame.Reset()
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if frame.Len() > 0 {
					return sendEvents(frame.Bytes(), events)
				}
				return nil
			}
			return err
		}
	}
}

func isBlankLine(line []byte) bool {
	return len(bytes.TrimRight(line, "\r\n")) == 0
}

func sendEvents(frame []byte, events chan<- Event) error {
	decoded, err := ginsse.Decode(bytes.NewReader(normalizeFrame(frame)))
	if err != nil {
		return err
	}
	for _, event := range decoded {
		events <- event
	}
	return nil
}

func normalizeFrame(frame []byte) []byte {
	frame = bytes.ReplaceAll(frame, []byte("\r\n"), []byte("\n"))
	frame = bytes.ReplaceAll(frame, []byte("\r"), []byte("\n"))
	return frame
}
