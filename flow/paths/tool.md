# Tool

A named capability the agent can invoke during a loop turn.

## Principles
- One file per tool, named after what it does (`timenow.go`, not `tool.go`)
- Schema is inferred from the input type — no hand-written JSON
- `Run()` never returns a Go error — all outcomes go through the envelope
- Tools are registered at wiring time; no dynamic registration at runtime

## Typed tool (default)

For stateless tools. Schema is inferred from `I`; output is wrapped in `Envelope[O]`.

```go
type myInput struct {
    Query string `json:"query" jsonschema:"the search query"`
}

func NewMyTool() (tools.Tool, error) {
    return tools.New[myInput, string](
        "my.tool",
        "One sentence: what it returns, when to use it, what not to confuse it with.",
        func(ctx context.Context, in myInput) (string, error) {
            result, err := doWork(in.Query)
            if err != nil {
                return "", err  // becomes {"status":"error","message":"..."}
            }
            return result, nil  // becomes {"status":"ok","result":"..."}
        },
    )
}
```

## Stateful tool

For tools that hold injected dependencies (handlers, clients, stores). Implement `tools.Tool` directly; use `tools.OK` / `tools.Fail` for output and `tools.SchemaFor` for the schema.

```go
type myInput struct {
    Query string `json:"query" jsonschema:"the search query"`
}

type MyTool struct {
    client SomeClient
    schema json.RawMessage
}

func NewMyTool(client SomeClient) (*MyTool, error) {
    schema, err := tools.SchemaFor[myInput]()
    if err != nil {
        return nil, err
    }
    return &MyTool{client: client, schema: schema}, nil
}

func (t *MyTool) Definition() tools.Definition {
    return tools.Definition{
        Name:        "my.tool",
        Description: "One sentence: what it returns, when to use it, what not to confuse it with.",
        Parameters:  t.schema,
    }
}

func (t *MyTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
    var input myInput
    if err := json.Unmarshal(args, &input); err != nil {
        return tools.Fail(fmt.Sprintf("invalid args: %s", err))
    }
    result, err := t.client.Query(ctx, input.Query)
    if err != nil {
        return tools.Fail(err.Error())
    }
    return tools.OK(result)
}
```

## Wiring

```go
// Both patterns wire the same way
myTool, err := NewMyTool(client)
if err != nil { ... }
registry, err := tools.NewRegistry(myTool)
```

## Envelope

All tool output reaches the model as a JSON envelope:

```json
{"status": "ok",    "result": ...}
{"status": "error", "message": "..."}
```

## Writing a good Description
The model reads this to decide when and whether to call the tool. Be short and precise:
- Say what it returns, not just what it is ("Returns X" not "A tool that…")
- Call out what it does NOT do if the model might assume otherwise
- Name the failure mode if there is one ("Returns an error if the target is unreachable")
