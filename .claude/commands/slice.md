Design and create a slice for the current feature.
The feature must already be picked and confirmed by the user.

1. **Read context**
   Read files in `flow/paths/`, `docs/architecture.md`, and the feature node via `./flow/flow feature <slug>`.

2. **Think**
   Reason through the design openly:
   - What exactly needs to be built
   - Which golden paths apply
   - What files and signatures change
   - How it will be validated
   - What could go wrong or be missed

3. **Check golden paths**
   For each golden path needed, check if it exists in `flow/paths/`.
   List them:
   - [path name](../flow/paths/<name>.md) ✓
   - [path name](../flow/paths/<name>.md) ✗ — missing

   If any are missing: run `/design-path` for each before continuing.

4. **Present the design** to the user for confirmation:

   ```
   ## Slice: <slug>

   **Feature:** `<feature-slug>`

   ### Thinking
   <reasoning>

   ### Golden paths
   - [cli-command](../flow/paths/cli.md) ✓
   - [db-migration](../flow/paths/db-migration.md) ✗

   ### What it does
   ### Boundaries
   ### Files and signatures
   ### Validation plan
   ### DoD
   ### Edge cases
   ```

5. **Create the slice file** once the user confirms.
   Write to `flow/slices/<feature-slug>.md`.
   Run `./flow/flow status <feature-slug> designing`.
   Commit: `feat(<domain>): slice <feature-slug>`
