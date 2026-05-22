package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iamhectordev/hector/pkg/llm/schema"
	sdkopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/param"
	"github.com/openai/openai-go/v3/shared"
)

const defaultModel = "gpt-4o-mini"

// Completer sends chat history to OpenAI Chat Completions.
type Completer struct {
	inner   sdkopenai.Client
	model   string
	bodyLog BodyLogConfig
}

type Option func(*Completer)

func New(apiKey, model string, opts ...Option) *Completer {
	model = strings.TrimSpace(model)
	if model == "" {
		model = defaultModel
	}

	c := &Completer{
		model: model,
	}
	for _, opt := range opts {
		opt(c)
	}

	options := []option.RequestOption{option.WithAPIKey(strings.TrimSpace(apiKey))}
	if c.bodyLog.Enabled {
		options = append(options, option.WithMiddleware(c.bodyLogMiddleware()))
	}
	c.inner = sdkopenai.NewClient(options...)
	return c
}

func (c *Completer) Complete(ctx context.Context, req schema.CompletionRequest) (*schema.Message, error) {
	toolNames, tools, err := mapTools(req.Tools)
	if err != nil {
		return nil, err
	}

	params := make([]sdkopenai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if req.System != "" {
		params = append(params, sdkopenai.SystemMessage(req.System))
	}
	for _, msg := range req.Messages {
		if msg == nil {
			continue
		}

		switch msg.Role {
		case schema.RoleSystem:
			params = append(params, sdkopenai.SystemMessage(msg.Content))
		case schema.RoleUser:
			params = append(params, userMessageParam(msg))
		case schema.RoleAssistant:
			params = append(params, assistantMessageParam(msg, toolNames))
		case schema.RoleTool:
			params = append(params, sdkopenai.ToolMessage(msg.Content, msg.ToolCallID))
		default:
			return nil, fmt.Errorf("llm: unsupported role %q", msg.Role)
		}
	}

	if len(params) == 0 {
		return nil, fmt.Errorf("llm: no messages")
	}

	completion, err := c.inner.Chat.Completions.New(ctx, sdkopenai.ChatCompletionNewParams{
		Model:    sdkopenai.ChatModel(c.model),
		Messages: params,
		Tools:    tools,
	})
	if err != nil {
		return nil, err
	}
	if completion == nil {
		return nil, fmt.Errorf("llm: nil completion")
	}
	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("llm: no choices returned")
	}

	choice := completion.Choices[0]
	reply := schema.AssistantMessage(choice.Message.Content)
	switch choice.FinishReason {
	case "stop":
		reply.FinishReason = schema.FinishReasonStop
	case "tool_calls":
		reply.FinishReason = schema.FinishReasonToolCalls
	case "length":
		reply.FinishReason = schema.FinishReasonLength
	}
	for _, call := range choice.Message.ToolCalls {
		name := call.Function.Name
		if original, ok := toolNames.fromOpenAI[name]; ok {
			name = original
		}
		reply.ToolCalls = append(reply.ToolCalls, schema.ToolCall{
			ID:        call.ID,
			Name:      name,
			Arguments: json.RawMessage(call.Function.Arguments),
		})
	}
	return reply, nil
}

func userMessageParam(msg *schema.Message) sdkopenai.ChatCompletionMessageParamUnion {
	if len(msg.Parts) == 0 {
		return sdkopenai.UserMessage(msg.Content)
	}
	return sdkopenai.UserMessage(contentPartParams(msg.Parts))
}

func contentPartParams(parts []schema.MessagePart) []sdkopenai.ChatCompletionContentPartUnionParam {
	out := make([]sdkopenai.ChatCompletionContentPartUnionParam, 0, len(parts))
	for _, part := range parts {
		switch part.Type {
		case schema.MessagePartTypeText:
			out = append(out, sdkopenai.TextContentPart(part.Text))
		case schema.MessagePartTypeImage:
			if part.Image == nil {
				continue
			}
			out = append(out, sdkopenai.ImageContentPart(sdkopenai.ChatCompletionContentPartImageImageURLParam{
				URL:    imageDataURL(part.Image),
				Detail: part.Image.Detail,
			}))
		}
	}
	return out
}

func imageDataURL(image *schema.ImagePart) string {
	return "data:" + image.MIMEType + ";base64," + image.Base64Data
}

func assistantMessageParam(msg *schema.Message, names toolNameMaps) sdkopenai.ChatCompletionMessageParamUnion {
	if len(msg.ToolCalls) == 0 {
		return sdkopenai.AssistantMessage(msg.Content)
	}

	paramMsg := sdkopenai.ChatCompletionAssistantMessageParam{}
	if msg.Content != "" {
		paramMsg.Content.OfString = param.NewOpt(msg.Content)
	}
	for _, call := range msg.ToolCalls {
		name := call.Name
		if mapped, ok := names.toOpenAI[call.Name]; ok {
			name = mapped
		}
		paramMsg.ToolCalls = append(paramMsg.ToolCalls, sdkopenai.ChatCompletionMessageToolCallUnionParam{
			OfFunction: &sdkopenai.ChatCompletionMessageFunctionToolCallParam{
				ID: call.ID,
				Function: sdkopenai.ChatCompletionMessageFunctionToolCallFunctionParam{
					Name:      name,
					Arguments: string(call.Arguments),
				},
			},
		})
	}
	return sdkopenai.ChatCompletionMessageParamUnion{OfAssistant: &paramMsg}
}

func mapTools(defs []schema.ToolDefinition) (toolNameMaps, []sdkopenai.ChatCompletionToolUnionParam, error) {
	names := toolNameMaps{
		toOpenAI:   make(map[string]string, len(defs)),
		fromOpenAI: make(map[string]string, len(defs)),
	}
	tools := make([]sdkopenai.ChatCompletionToolUnionParam, 0, len(defs))
	for _, def := range defs {
		name := openAIToolName(def.Name)
		if existing, ok := names.fromOpenAI[name]; ok && existing != def.Name {
			return toolNameMaps{}, nil, fmt.Errorf("llm: tool names %q and %q both map to %q", existing, def.Name, name)
		}
		names.toOpenAI[def.Name] = name
		names.fromOpenAI[name] = def.Name

		parameters := shared.FunctionParameters{}
		if len(def.Parameters) > 0 {
			if err := json.Unmarshal(def.Parameters, &parameters); err != nil {
				return toolNameMaps{}, nil, fmt.Errorf("llm: tool %q parameters: %w", def.Name, err)
			}
		}

		tools = append(tools, sdkopenai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
			Name:        name,
			Description: param.NewOpt(def.Description),
			Parameters:  parameters,
		}))
	}
	return names, tools, nil
}

func openAIToolName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 64 {
			break
		}
	}
	if b.Len() == 0 {
		return "tool"
	}
	return b.String()
}

type toolNameMaps struct {
	toOpenAI   map[string]string
	fromOpenAI map[string]string
}
