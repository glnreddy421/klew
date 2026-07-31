package tail

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"unicode"
)

// Format identifies how a line was decoded.
type Format uint8

const (
	FormatRaw Format = iota
	FormatJSON
	FormatLogfmt
)

// Event is one parsed log line ready for Drain3.
// Value type so the hot path can avoid pointer churn on the channel.
type Event struct {
	Message string
	Format  Format
}

// Source streams Events until the reader EOF or context cancel.
type Source interface {
	Run(ctx context.Context, out chan<- Event) error
}

// ReaderSource tails structured JSON / logfmt / raw lines from an io.Reader.
type ReaderSource struct {
	R       io.Reader
	BufSize int // scanner buffer size; 0 → 64KiB
}

// Run reads line-delimited logs and pushes Events. It does not close out.
func (s *ReaderSource) Run(ctx context.Context, out chan<- Event) error {
	if s.R == nil {
		return io.ErrUnexpectedEOF
	}
	bufSize := s.BufSize
	if bufSize <= 0 {
		bufSize = 64 * 1024
	}

	sc := bufio.NewScanner(s.R)
	startCap := 4096
	if bufSize < startCap {
		startCap = bufSize
	}
	sc.Buffer(make([]byte, 0, startCap), bufSize)

	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		ev := decodeLine(line)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case out <- ev:
		}
	}
	return sc.Err()
}

func decodeLine(line string) Event {
	if line[0] == '{' {
		if msg, ok := decodeJSONMessage(line); ok {
			return Event{Message: msg, Format: FormatJSON}
		}
	}
	if msg, ok := decodeLogfmtMessage(line); ok {
		return Event{Message: msg, Format: FormatLogfmt}
	}
	return Event{Message: line, Format: FormatRaw}
}

func decodeJSONMessage(line string) (string, bool) {
	// Prefer common K8s / app fields without allocating a full map when possible.
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return "", false
	}
	for _, key := range []string{"msg", "message", "log", "MESSAGE"} {
		raw, ok := payload[key]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s, true
		}
	}
	// Fall back to compact body without structural keys for mining.
	delete(payload, "time")
	delete(payload, "timestamp")
	delete(payload, "ts")
	delete(payload, "level")
	delete(payload, "severity")
	delete(payload, "stream")
	b, err := json.Marshal(payload)
	if err != nil || len(b) == 0 || string(b) == "{}" {
		return line, true
	}
	return string(b), true
}

func decodeLogfmtMessage(line string) (string, bool) {
	if !strings.Contains(line, "=") {
		return "", false
	}
	// Prefer msg= / message= value; otherwise strip noisy keys and keep remainder.
	fields := splitLogfmt(line)
	if len(fields) == 0 {
		return "", false
	}
	for _, key := range []string{"msg", "message", "log"} {
		if v, ok := fields[key]; ok && v != "" {
			return v, true
		}
	}
	var b strings.Builder
	first := true
	for k, v := range fields {
		switch strings.ToLower(k) {
		case "time", "timestamp", "ts", "level", "severity", "stream":
			continue
		}
		if !first {
			b.WriteByte(' ')
		}
		first = false
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(v)
	}
	if b.Len() == 0 {
		return line, true
	}
	return b.String(), true
}

func splitLogfmt(line string) map[string]string {
	out := make(map[string]string, 8)
	i := 0
	for i < len(line) {
		for i < len(line) && unicode.IsSpace(rune(line[i])) {
			i++
		}
		if i >= len(line) {
			break
		}
		eq := strings.IndexByte(line[i:], '=')
		if eq < 0 {
			break
		}
		key := line[i : i+eq]
		i += eq + 1
		if i >= len(line) {
			out[key] = ""
			break
		}
		var val string
		if line[i] == '"' {
			i++
			end := i
			for end < len(line) && line[end] != '"' {
				if line[end] == '\\' && end+1 < len(line) {
					end += 2
					continue
				}
				end++
			}
			val = line[i:end]
			if end < len(line) && line[end] == '"' {
				end++
			}
			i = end
		} else {
			end := i
			for end < len(line) && !unicode.IsSpace(rune(line[end])) {
				end++
			}
			val = line[i:end]
			i = end
		}
		out[key] = val
	}
	return out
}
