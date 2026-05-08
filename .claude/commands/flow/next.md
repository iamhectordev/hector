Show what to work on next in the current milestone.

1. **Find the milestone**
   If not provided, fetch open milestones and ask which one to look at.

2. **Fetch issues**
   gh issue list --milestone "<title>" --json number,title,state
   For each open issue, check its blockers:
   gh api repos/{owner}/{repo}/issues/{number}/blocked-by

3. **Show the picture**
   Categorise and display:

   ready    #4  Implement session store
   ready    #7  Add config field for DB path
   blocked  #5  Wire store to agent  ← blocked by #3
   done     #1  Set up waffle bus

4. **Suggest**
   Pick the top unblocked issue by creation order.
   Ask if the user wants to start it — if yes, hand off to /flow:tdd.
