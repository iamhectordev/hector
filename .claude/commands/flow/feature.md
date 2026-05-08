Interview the user to understand a new feature idea, then shape it into the tree.

1. **Interview**
   Ask until the feature is clear — what problem it solves, who feels it,
   what success looks like. One or two questions at a time. Conversational.

2. **Check the tree**
   Read `flow/tree.yaml`. Does this feature already exist under a different name?
   Conflict with something planned? Which domain does it belong to?
   Surface findings before going further.

3. **Shape it**
   Propose: slug, title, description, domain, priority, and the main user stories.
   If stories are piling up (approaching 10), flag it — the feature may need splitting.
   Confirm with the user.

4. **Add to tree**
   ./flow/flow add <parent> <slug> "<title>" "<description>"
   ./flow/flow story <slug> "<story>"  (repeat for each)
   Commit: feat(tree): add <slug>
