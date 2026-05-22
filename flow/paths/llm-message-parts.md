# LLM message parts

Provider-neutral structured content for model-facing messages.

## Principles
- `pkg/llm/schema.Message` owns the persisted transcript shape
- `Content` remains the text fallback for providers without structured input support
- `Parts` is ordered and provider-neutral; provider adapters map parts to their native wire format
- Attachment IDs link prompt XML nodes to structured payloads
- Provider-specific details such as data URLs stay inside provider adapters

## Outline
```go
msg := schema.UserMessageWithParts(xml, []schema.MessagePart{
    schema.TextPart(xml),
    schema.TextPart(`<image_data id="F123"/>`),
    schema.NewImagePart("F123", base64Data, "image/png"),
})
```
