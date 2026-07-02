package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/iamhectordev/hector/pkg/llm/schema"
	"github.com/iamhectordev/hector/pkg/telem"
)

const defaultModel = "claude-opus-4-7"
const defaultMaxTokens = 8096

// Completer sends chat history to the Anthropic Messages API.
type Completer struct {
	inner sdk.Client
	model string
	opts  []option.RequestOption
}

func New(cfg Config) *Completer {
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = defaultModel
	}
	c := &Completer{model: model}
	c.inner = sdk.NewClient(append([]option.RequestOption{option.WithAPIKey(cfg.APIKey)}, c.opts...)...)
	return c
}

// WithClientOption appends a request option to the underlying SDK client.
// Intended for tests that need to redirect requests to a local server.
func (c *Completer) WithClientOption(opt option.RequestOption) {
	c.inner = sdk.NewClient(option.WithAPIKey("sk-ant-test"), opt)
}

func (c *Completer) TelemetryFields() []telem.Field {
	return []telem.Field{
		telem.String("llm.provider", "anthropic"),
		telem.String("llm.model", c.model),
	}
}

func (c *Completer) Complete(ctx context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	messages, err := mapMessages(req.Messages)
	if err != nil {
		return nil, err
	}

	tools, err := mapTools(req.Tools)
	if err != nil {
		return nil, err
	}

	params := sdk.MessageNewParams{
		Model:     sdk.Model(c.model),
		MaxTokens: defaultMaxTokens,
		Messages:  messages,
		Tools:     tools,
	}
	if req.System != "" {
		params.System = []sdk.TextBlockParam{{Text: req.System}}
	}

	msg, err := c.inner.Messages.New(ctx, params)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, fmt.Errorf("anthropic: nil response")
	}

	return mapResponse(msg)
}

func mapMessages(msgs []*schema.Message) ([]sdk.MessageParam, error) {
	out := make([]sdk.MessageParam, 0, len(msgs))
	i := 0
	for i < len(msgs) {
		msg := msgs[i]
		if msg == nil {
			i++
			continue
		}
		switch msg.Role {
		case schema.RoleSystem:
			// system is handled via MessageNewParams.System; skip inline
			i++
		case schema.RoleUser:
			out = append(out, sdk.NewUserMessage(userBlocks(msg)...))
			i++
		case schema.RoleAssistant:
			blocks, err := assistantBlocks(msg)
			if err != nil {
				return nil, err
			}
			out = append(out, sdk.NewAssistantMessage(blocks...))
			i++
		case schema.RoleTool:
			// Consecutive tool results → single user message with tool_result blocks.
			var resultBlocks []sdk.ContentBlockParamUnion
			for i < len(msgs) && msgs[i] != nil && msgs[i].Role == schema.RoleTool {
				resultBlocks = append(resultBlocks, sdk.NewToolResultBlock(msgs[i].ToolCallID, msgs[i].Content, false))
				i++
			}
			out = append(out, sdk.NewUserMessage(resultBlocks...))
		default:
			return nil, fmt.Errorf("anthropic: unsupported role %q", msg.Role)
		}
	}
	return out, nil
}

func userBlocks(msg *schema.Message) []sdk.ContentBlockParamUnion {
	if len(msg.Parts) == 0 {
		return []sdk.ContentBlockParamUnion{sdk.NewTextBlock(msg.Content)}
	}
	blocks := make([]sdk.ContentBlockParamUnion, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch part.Type {
		case schema.MessagePartTypeText:
			blocks = append(blocks, sdk.NewTextBlock(part.Text))
		case schema.MessagePartTypeImage:
			if part.Image != nil {
				blocks = append(blocks, sdk.NewImageBlockBase64(part.Image.MIMEType, part.Image.Base64Data))
			}
		}
	}
	return blocks
}

func assistantBlocks(msg *schema.Message) ([]sdk.ContentBlockParamUnion, error) {
	blocks := make([]sdk.ContentBlockParamUnion, 0, len(msg.ToolCalls)+1)
	if msg.Content != "" {
		blocks = append(blocks, sdk.NewTextBlock(msg.Content))
	}
	for _, call := range msg.ToolCalls {
		var input any
		if len(call.Arguments) > 0 {
			input = json.RawMessage(call.Arguments)
		} else {
			input = map[string]any{}
		}
		blocks = append(blocks, sdk.NewToolUseBlock(call.ID, input, call.Name))
	}
	if len(blocks) == 0 {
		return nil, fmt.Errorf("anthropic: assistant message has no content and no tool calls")
	}
	return blocks, nil
}

func mapTools(defs []schema.ToolDefinition) ([]sdk.ToolUnionParam, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	tools := make([]sdk.ToolUnionParam, 0, len(defs))
	for _, def := range defs {
		schema, err := toolInputSchema(def.Parameters)
		if err != nil {
			return nil, fmt.Errorf("anthropic: tool %q parameters: %w", def.Name, err)
		}
		t := sdk.ToolUnionParamOfTool(schema, def.Name)
		if def.Description != "" {
			t.OfTool.Description = param.NewOpt(def.Description)
		}
		tools = append(tools, t)
	}
	return tools, nil
}

func toolInputSchema(raw json.RawMessage) (sdk.ToolInputSchemaParam, error) {
	if len(raw) == 0 {
		return sdk.ToolInputSchemaParam{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return sdk.ToolInputSchemaParam{}, err
	}
	s := sdk.ToolInputSchemaParam{}
	if v, ok := m["properties"]; ok {
		s.Properties = v
		delete(m, "properties")
	}
	if v, ok := m["required"]; ok {
		if items, ok := v.([]any); ok {
			for _, item := range items {
				if str, ok := item.(string); ok {
					s.Required = append(s.Required, str)
				}
			}
		}
		delete(m, "required")
	}
	delete(m, "type") // constant.Object default
	if len(m) > 0 {
		s.ExtraFields = m
	}
	return s, nil
}

func mapResponse(msg *sdk.Message) (*schema.Message, error) {
	reply := schema.AssistantMessage("")

	switch msg.StopReason {
	case sdk.StopReasonEndTurn:
		reply.FinishReason = schema.FinishReasonStop
	case sdk.StopReasonToolUse:
		reply.FinishReason = schema.FinishReasonToolCalls
	case sdk.StopReasonMaxTokens:
		reply.FinishReason = schema.FinishReasonLength
	}

	for _, block := range msg.Content {
		switch block.Type {
		case "text":
			reply.Content = block.Text
		case "tool_use":
			reply.ToolCalls = append(reply.ToolCalls, schema.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: json.RawMessage(block.Input),
			})
		}
	}
	return reply, nil
}
