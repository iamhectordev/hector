Design a missing golden path and add it to flow/paths/.

A golden path is a canonical pattern that must exist before it is used in a slice.
The name of the path to design is provided as an argument or from context.

1. Read existing files in `flow/paths/` and `docs/architecture.md` for context
2. Discuss the pattern with the user — what it covers, how it works, constraints
3. Once agreed, write `flow/paths/<name>.md`:
   - Principles
   - Outline with code snippet
   - Example pointer: `_not yet defined_` until a real example exists in code
4. Commit: `docs(paths): define <name> golden path`
