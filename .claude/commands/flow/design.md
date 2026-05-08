Design the technical approach for the current milestone or issue.

1. **Read context**
   - List flow/paths/ — see which golden paths are already defined
   - Read docs/architecture.md
   - Read relevant source files for the area being changed

2. **Draft the design**
   Walk through the code that needs to exist. Show the shape:
   - New types, interfaces, functions — with signatures
   - How data flows through the system end to end
   - What calls what, in what order
   - Where existing code changes and how

   Where things aren't obvious, present options with trade-offs.
   Flag risks: what could go wrong, what's uncertain, what couples badly.

   Format: prose + code sketches. Short. No walls of text.

3. **Golden paths**
   Look at what's in flow/paths/. For each part of the design, ask:
   is there an existing path that fits here?

   Where there's a gap, decide together:
   - If the pattern is common and reusable → design a new golden path, write flow/paths/<name>.md following TEMPLATE.md
   - If it's specific to this milestone → design a one-off solution

   This is a discussion. Don't create paths unilaterally.

4. **Test plan**
   What gets tested, through which interface, what observable output confirms it works.
