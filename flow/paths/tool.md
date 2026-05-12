# Tool

A named capability the agent can invoke during a loop turn.

## Principles
- One file per tool, named after what it does (`timenow.go`, not `tool.go`)
- `Definition()` returns the name, description, and input schema as `json.RawMessage`
- `Run()` is pure — no side effects on shared state; errors return as `(string, error)`
- Tools are registered at wiring time; no dynamic registration at runtime

## Outline
```go
type MyTool struct{}

func (MyTool) Definition() tools.Definition {
    return tools.Definition{
        Name:        "my.tool",
        Description: "One sentence: what it does, when to use it, and what not to confuse it with.",
        Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
    }
}

func (MyTool) Run(ctx context.Context, args json.RawMessage) (string, error) {
    // unmarshal args, do work, return string result
    return result, nil
}
```

## Wiring
```go
// in cli action
catalog := agent.NewCatalog(tools.MyTool{})
agent.NewLoop(completer, catalog)
```

## Writing a good Description
The model reads this to decide when and whether to call the tool. Be short and precise:
- Say what it returns, not just what it is ("Returns X" not "A tool that…")
- Call out what it does NOT do if the model might assume otherwise
- Name the failure mode if there is one ("Returns an error if the target is unreachable")
