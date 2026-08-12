package terminal

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EventType represents the category of shell event emitted by OSC 133 integration.
type EventType string

const (
	EventPromptStart  EventType = "prompt_start"
	EventCommandStart EventType = "command_start"
	EventCommandEnd   EventType = "command_end"
	EventEditorStart  EventType = "editor_start"
	EventEditorEnd    EventType = "editor_end"
)

// ShellEvent represents a parsed semantic shell event.
type ShellEvent struct {
	Type       EventType
	Command    string
	ExitCode   int
	Duration   time.Duration
	PWD        string
	Timestamp  time.Time
	RawPayload string
}

// TelemetryParser is a streaming parser for OSC 133 and custom shell integration markers.
// It is safe for concurrent use and supports fragmented byte streams across network chunks.
type TelemetryParser struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	OnEvent func(ShellEvent)
}

// NewTelemetryParser creates a new TelemetryParser instance.
func NewTelemetryParser() *TelemetryParser {
	return &TelemetryParser{}
}

// Feed streams new raw bytes through the parser, scanning for OSC sequences and emitting events.
func (p *TelemetryParser) Feed(data []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.buf.Write(data)
	bufBytes := p.buf.Bytes()

	for len(bufBytes) > 0 {
		// Look for ESC (0x1b)
		escIdx := bytes.IndexByte(bufBytes, 0x1b)
		if escIdx == -1 {
			// No escape sequence in buffer, discard old bytes to prevent unbounded growth
			// Keep at most 1 byte in case ESC is the next byte
			p.buf.Reset()
			return
		}

		// Advance past non-ESC bytes
		bufBytes = bufBytes[escIdx:]

		// Check if we have enough bytes for "\x1b]133;" (6 bytes)
		if len(bufBytes) < 6 {
			// Incomplete sequence, keep remaining bytes in buffer for next feed
			p.buf.Reset()
			p.buf.Write(bufBytes)
			return
		}

		if bufBytes[1] != ']' || !bytes.HasPrefix(bufBytes[2:], []byte("133;")) {
			// Not an OSC 133 sequence, advance by 1 byte
			bufBytes = bufBytes[1:]
			continue
		}

		// Find string terminator (BEL: 0x07 or ST: \x1b\)
		termIdx := -1
		termLen := 0
		for i := 6; i < len(bufBytes); i++ {
			if bufBytes[i] == 0x07 {
				termIdx = i
				termLen = 1
				break
			}
			if bufBytes[i] == 0x1b && i+1 < len(bufBytes) && bufBytes[i+1] == '\\' {
				termIdx = i
				termLen = 2
				break
			}
		}

		if termIdx == -1 {
			// Incomplete sequence, keep in buffer waiting for terminator
			p.buf.Reset()
			p.buf.Write(bufBytes)
			return
		}

		// Extract payload between "133;" and terminator
		payload := string(bufBytes[6:termIdx])
		p.parseAndEmit(payload)

		// Advance past the terminator
		bufBytes = bufBytes[termIdx+termLen:]
	}

	p.buf.Reset()
	if len(bufBytes) > 0 {
		p.buf.Write(bufBytes)
	}
}

// parseAndEmit parses an OSC 133 payload string and triggers the OnEvent callback.
func (p *TelemetryParser) parseAndEmit(payload string) {
	if len(payload) == 0 {
		return
	}

	// Payload starts with marker type: A, B, C, D, G, H, etc.
	markerType := string(payload[0])
	rest := ""
	if len(payload) > 1 && payload[1] == ';' {
		rest = payload[2:]
	}

	event := ShellEvent{
		Timestamp:  time.Now(),
		RawPayload: payload,
	}

	params := parseKeyValuePairs(rest)

	switch markerType {
	case "A":
		event.Type = EventPromptStart
		if pwd, ok := params["pwd"]; ok {
			event.PWD = decodeBase64OrRaw(pwd)
		}
	case "B":
		event.Type = EventCommandStart
		if cmd, ok := params["cmd"]; ok {
			event.Command = decodeBase64OrRaw(cmd)
		}
		if pwd, ok := params["pwd"]; ok {
			event.PWD = decodeBase64OrRaw(pwd)
		}
	case "C":
		// Command execution marker
		return
	case "D":
		event.Type = EventCommandEnd
		// Check for exit code in params (e.g. exit=1 or plain number)
		if exitStr, ok := params["exit"]; ok {
			if code, err := strconv.Atoi(exitStr); err == nil {
				event.ExitCode = code
			}
		} else if rest != "" && !strings.Contains(rest, "=") {
			// Standard fallback: \x1b]133;D;127\x07
			if code, err := strconv.Atoi(strings.TrimSpace(rest)); err == nil {
				event.ExitCode = code
			}
		}

		if durStr, ok := params["dur"]; ok {
			if durMs, err := strconv.ParseInt(durStr, 10, 64); err == nil {
				event.Duration = time.Duration(durMs) * time.Millisecond
			}
		}

		if cmd, ok := params["cmd"]; ok {
			event.Command = decodeBase64OrRaw(cmd)
		}
		if pwd, ok := params["pwd"]; ok {
			event.PWD = decodeBase64OrRaw(pwd)
		}
	case "G":
		event.Type = EventEditorStart
		event.Command = decodeBase64OrRaw(rest)
	case "H":
		event.Type = EventEditorEnd
	default:
		return
	}

	if p.OnEvent != nil {
		p.OnEvent(event)
	}
}

// parseKeyValuePairs parses semicolon-delimited key=value strings.
func parseKeyValuePairs(input string) map[string]string {
	result := make(map[string]string)
	if input == "" {
		return result
	}

	parts := strings.Split(input, ";")
	for _, part := range parts {
		if idx := strings.IndexByte(part, '='); idx != -1 {
			k := strings.TrimSpace(part[:idx])
			v := strings.TrimSpace(part[idx+1:])
			result[k] = v
		}
	}
	return result
}

// decodeBase64OrRaw attempts to decode a standard or unpadded base64 string, falling back to the raw string.
func decodeBase64OrRaw(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err == nil {
		return string(decoded)
	}
	rawDecoded, err := base64.RawStdEncoding.DecodeString(s)
	if err == nil {
		return string(rawDecoded)
	}
	return s
}
