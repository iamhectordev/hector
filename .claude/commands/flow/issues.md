Break the current milestone into vertical GitHub issues.

1. **Read context**
   - Fetch the milestone and its existing issues
   - List flow/paths/, read the ones relevant to this milestone

2. **Slice vertically**
   Each issue should be end-to-end, independently testable, and small enough
   to implement in one focused session. Issues can cover features, infrastructure,
   or test harness work — whatever unblocks delivery.

3. **Preview**
   Show the proposed list before creating anything:

   #1  Title                    paths: module ✓  listener ✓
   #2  Title                    paths: store ✗ → needs path design issue first
   #3  Design "store" path      blocker for #2

4. **Design where needed**
   For issues that aren't obvious, draft the design inline — flow, signatures,
   trade-offs, test plan. Write it into the issue body. Skip it where the work is clear.

5. **Confirm then create**
   Wait for user confirmation on the full list.
   Create blocker issues first, then wire dependencies:

   gh issue create --milestone "<title>" --title "..." --body "..."
   gh api repos/{owner}/{repo}/issues/{number}/blocked-by \
     --method POST --field blocked_by_issue_number=<n>
