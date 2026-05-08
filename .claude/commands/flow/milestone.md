Shape a feature into a deliverable GitHub milestone.

Takes an optional hint (e.g. "slack", "sessions") or no argument for open exploration.

1. **Load context**
   - Read `flow/tree.yaml` — find features matching the hint, note their status
   - Fetch open GitHub milestones: `gh api repos/{owner}/{repo}/milestones`
   - If relevant milestones found, fetch their issues too

2. **Surface the picture**
   Show: relevant tree features and their status, existing milestones and their issues.

3. **Converge**
   Ask: what do you want to deliver next?
   A good milestone is small, demoable, and valuable on its own.
   If scope feels large, push back and suggest splitting.

4. **Define it via conversation**
   - Problem Statement
   - Solution
   - User Stories
   - Non-functional Requirements (performance, reliability, security — only what's real)
   - Out of Scope (only if there's a genuine risk of scope creep or misreading)

5. **Create the milestone**
   Title: plain name, no prefix.
   gh api repos/{owner}/{repo}/milestones --method POST \
     --field title="<title>" --field description="<description>"
