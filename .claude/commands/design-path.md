Design a missing golden path and add it to docs/golden-paths.md.

A golden path is a canonical pattern that must exist before it is used in a slice.
The name of the path to design is provided as an argument or from context.

1. Read `docs/golden-paths.md` and `docs/architecture.md` for context
2. Discuss the pattern with the user — what it covers, how it works, constraints
3. Once agreed, write the entry to `docs/golden-paths.md`:
   - Name (as a heading)
   - Description
   - Rules and constraints
   - Example pointer: `_not yet defined_` until a real example exists in code
4. Commit: `docs(golden-paths): define <name> golden path`
