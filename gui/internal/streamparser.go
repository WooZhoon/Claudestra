package internal

import (
	"bufio"
	"encoding/json"
	"io"
	"path/filepath"
	"strings"
)

// ── Claude CLI stream-json wire format ──

type streamEvent struct {
	Type   string          `json:"type"`   // "stream_event", "result", "system", "assistant", ...
	Event  *streamSubEvent `json:"event"`  // present when Type == "stream_event"
	Result string          `json:"result"` // present when Type == "result"
}

type streamSubEvent struct {
	Type         string        `json:"type"` // "content_block_start", "content_block_delta", "content_block_stop", ...
	Index        int           `json:"index"`
	ContentBlock *contentBlock `json:"content_block"`
	Delta        *streamDelta  `json:"delta"`
}

type contentBlock struct {
	Type string `json:"type"` // "thinking", "text", "tool_use"
	Name string `json:"name"` // tool name (for tool_use)
}

type streamDelta struct {
	Type        string `json:"type"`         // "thinking_delta", "text_delta", "signature_delta", "input_json_delta"
	Text        string `json:"text"`         // for text_delta
	Thinking    string `json:"thinking"`     // for thinking_delta
	PartialJSON string `json:"partial_json"` // for input_json_delta
}

// ── Callbacks ──

type StreamCallbacks struct {
	OnText     func(text string)                   // accumulated text chunk (flushed on space or newline)
	OnThinking func(text string)                   // accumulated thinking chunk
	OnToolUse  func(toolName string, input string) // tool invocation (name + summarized input)
	OnResult   func(result string)
}

// ParseStream reads Claude CLI stream-json lines from reader and calls
// the appropriate callbacks.
//
// Flush 전략:
//   - text/thinking: 공백 또는 줄바꿈이 포함된 토큰 도착 시 즉시 flush (단어 단위 스트리밍)
//   - text는 content_block_stop에서 flush하지 않음 (tool 사이 텍스트 이어붙이기)
func ParseStream(reader io.Reader, cb StreamCallbacks) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 128*1024), 512*1024)

	var textBuf strings.Builder
	var thinkBuf strings.Builder
	var toolInputBuf strings.Builder
	var currentBlockType string
	var currentToolName string

	flushText := func() {
		if textBuf.Len() > 0 && cb.OnText != nil {
			cb.OnText(textBuf.String())
			textBuf.Reset()
		}
	}
	flushThink := func() {
		if thinkBuf.Len() > 0 && cb.OnThinking != nil {
			cb.OnThinking(thinkBuf.String())
			thinkBuf.Reset()
		}
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var evt streamEvent
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "stream_event":
			if evt.Event == nil {
				continue
			}
			switch evt.Event.Type {
			case "content_block_start":
				if evt.Event.ContentBlock != nil {
					currentBlockType = evt.Event.ContentBlock.Type
					if currentBlockType == "tool_use" {
						flushText() // tool 시작 전 남은 텍스트 flush
						currentToolName = evt.Event.ContentBlock.Name
						toolInputBuf.Reset()
					}
				}

			case "content_block_delta":
				if evt.Event.Delta == nil {
					continue
				}
				switch evt.Event.Delta.Type {
				case "text_delta":
					textBuf.WriteString(evt.Event.Delta.Text)
					// 공백이나 줄바꿈이 포함되면 즉시 flush (단어 단위 스트리밍)
					if strings.ContainsAny(evt.Event.Delta.Text, " \n") {
						flushText()
					}
				case "thinking_delta":
					thinkBuf.WriteString(evt.Event.Delta.Thinking)
					if strings.ContainsAny(evt.Event.Delta.Thinking, " \n") {
						flushThink()
					}
				case "input_json_delta":
					toolInputBuf.WriteString(evt.Event.Delta.PartialJSON)
				}

			case "content_block_stop":
				// text는 여기서 flush하지 않음 — tool 호출 사이 텍스트를 이어붙여서
				// 단어 중간에서 끊기는 현상 방지.
				flushThink()
				if currentBlockType == "tool_use" && cb.OnToolUse != nil {
					summary := summarizeToolInput(currentToolName, toolInputBuf.String())
					cb.OnToolUse(currentToolName, summary)
					toolInputBuf.Reset()
				}
				currentBlockType = ""
			}

		case "result":
			flushText()
			flushThink()
			if cb.OnResult != nil {
				cb.OnResult(evt.Result)
			}
		}
	}

	// Final flush
	flushText()
	flushThink()
}

// summarizeToolInput extracts a short description from tool input JSON.
func summarizeToolInput(tool, rawJSON string) string {
	if rawJSON == "" {
		return ""
	}

	var input map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &input); err != nil {
		return ""
	}

	switch tool {
	case "Read":
		if fp, ok := input["file_path"].(string); ok {
			return filepath.Base(fp)
		}
	case "Glob":
		if p, ok := input["pattern"].(string); ok {
			return p
		}
	case "Grep":
		if p, ok := input["pattern"].(string); ok {
			return "\"" + p + "\""
		}
	case "Bash":
		if cmd, ok := input["command"].(string); ok {
			if len(cmd) > 50 {
				cmd = cmd[:50] + "..."
			}
			return cmd
		}
	case "Write":
		if fp, ok := input["file_path"].(string); ok {
			return filepath.Base(fp)
		}
	case "Edit":
		if fp, ok := input["file_path"].(string); ok {
			return filepath.Base(fp)
		}
	}
	return ""
}
