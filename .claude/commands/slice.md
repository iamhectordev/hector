Design and create a slice for the current feature.
The feature must already be picked and confirmed by the user.

1. **Read context**
   Read `docs/golden-paths.md`, `docs/architecture.md`, and the feature node via `./plan/flow feature <slug>`.

2. **Think**
   Reason through the design openly:
   - What exactly needs to be built
   - Which golden paths apply
   - What files and signatures change
   - How it will be validated
   - What could go wrong or be missed

3. **Check golden paths**
   For each golden path needed, check if it exists in `docs/golden-paths.md`.
   List them:
   - [path name](../docs/golden-paths.md#anchor) ✓
   - [path name](../docs/golden-paths.md#anchor) ✗ — missing

   If any are missing: run `/design-path` for each before continuing.

4. **Present the design** to the user for confirmation:

   ```
   ## Slice: <slug>

   **Feature:** `<feature-slug>`

   ### Thinking
   <reasoning>

   ### Golden paths
   - [cli-command](../docs/golden-paths.md#cli-command) ✓
   - [db-migration](../docs/golden-paths.md#db-migration) ✗

   ### What it does
   ### Boundaries
   ### Files and signatures
   ### Validation plan
   ### DoD
   ### Edge cases
   ```

5. **Create the slice file** once the user confirms.
   Write to `plan/slices/<feature-slug>.md`.
   Run `./plan/flow status <feature-slug> designing`.
   Commit: `feat(<domain>): slice <feature-slug>`
