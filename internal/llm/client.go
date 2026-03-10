package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an OpenAI-compatible chat completions API.
type Client struct {
	apiKey     string
	apiURL     string
	model      string
	httpClient *http.Client
}

// NewClient creates a new LLM client.
func NewClient(apiKey, apiURL, model string) *Client {
	return &Client{
		apiKey: apiKey,
		apiURL: strings.TrimRight(apiURL, "/"),
		model:  model,
		httpClient: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

// --- Chat Completions API types ---

// Message is a chat completion message.
type Message struct {
	Role       string      `json:"role"` // system, user, assistant, tool
	Content    string      `json:"content,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// ToolCall represents a tool call from the assistant.
type ToolCall struct {
	Index    int          `json:"index,omitempty"` // used in streaming deltas
	ID       string       `json:"id"`
	Type     string       `json:"type"` // "function"
	Function FunctionCall `json:"function"`
}

// FunctionCall is the function name + arguments JSON string.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// Tool describes a function the model can call.
type Tool struct {
	Type     string       `json:"type"` // "function"
	Function ToolFunction `json:"function"`
}

// ToolFunction is the function schema.
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// StreamOptions controls streaming behaviour.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatRequest is the request body for /chat/completions.
type ChatRequest struct {
	Model         string         `json:"model"`
	Messages      []Message      `json:"messages"`
	Tools         []Tool         `json:"tools,omitempty"`
	Stream        bool           `json:"stream,omitempty"`
	StreamOptions *StreamOptions `json:"stream_options,omitempty"`
}

// ChatResponse is the response from /chat/completions (non-streaming).
type ChatResponse struct {
	ID      string   `json:"id"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

// Choice is one completion choice.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage tracks token counts.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// StreamEvent is a parsed Server-Sent Event from a streaming response.
type StreamEvent struct {
	ID      string         `json:"id,omitempty"`
	Choices []StreamChoice `json:"choices,omitempty"`
	Usage   *Usage         `json:"usage,omitempty"`
}

// StreamChoice is a choice in a streaming chunk.
type StreamChoice struct {
	Index        int          `json:"index"`
	Delta        StreamDelta  `json:"delta"`
	FinishReason *string      `json:"finish_reason"`
}

// StreamDelta is the incremental content in a stream chunk.
type StreamDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   string     `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ChatCompletion sends a non-streaming chat completion request.
func (c *Client) ChatCompletion(messages []Message, tools []Tool) (*ChatResponse, error) {
	req := ChatRequest{
		Model:    c.model,
		Messages: messages,
		Tools:    tools,
		Stream:   false,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.apiURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &chatResp, nil
}

// StreamCallback is called for each streaming event.
type StreamCallback func(event StreamEvent)

// ChatCompletionStream sends a streaming chat completion request.
// It calls the callback for each parsed chunk; the final chunk includes usage.
// Returns the fully-assembled assistant message.
func (c *Client) ChatCompletionStream(messages []Message, tools []Tool, callback StreamCallback) (*Message, *Usage, error) {
	req := ChatRequest{
		Model:         c.model,
		Messages:      messages,
		Tools:         tools,
		Stream:        true,
		StreamOptions: &StreamOptions{IncludeUsage: true},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", c.apiURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Assemble the full message from stream deltas
	assembled := &Message{Role: "assistant"}
	toolCallBuilders := make(map[int]*ToolCall) // index -> accumulated tool call
	var usage *Usage

	scanner := bufio.NewScanner(resp.Body)
	// Increase buffer for potentially large streaming chunks
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var event StreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue // skip malformed chunks
		}

		if callback != nil {
			callback(event)
		}

		if event.Usage != nil {
			usage = event.Usage
		}

		for _, choice := range event.Choices {
			assembled.Content += choice.Delta.Content

			// Accumulate tool calls by index.
			// OpenAI streaming sends tool_calls with an integer "index".
			// The first chunk for an index has id + function.name;
			// subsequent chunks for the same index append to function.arguments.
			for _, tc := range choice.Delta.ToolCalls {
				idx := tc.Index
				if existing, ok := toolCallBuilders[idx]; ok {
					// Append arguments to existing tool call at this index
					existing.Function.Arguments += tc.Function.Arguments
				} else {
					// New tool call at this index
					toolCallBuilders[idx] = &ToolCall{
						ID:   tc.ID,
						Type: "function",
						Function: FunctionCall{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading stream: %w", err)
	}

	// Convert map to ordered slice
	if len(toolCallBuilders) > 0 {
		maxIdx := 0
		for idx := range toolCallBuilders {
			if idx > maxIdx {
				maxIdx = idx
			}
		}
		for i := 0; i <= maxIdx; i++ {
			if tc, ok := toolCallBuilders[i]; ok {
				assembled.ToolCalls = append(assembled.ToolCalls, *tc)
			}
		}
	}
	return assembled, usage, nil
}

// Model returns the configured model name.
func (c *Client) Model() string {
	return c.model
}
