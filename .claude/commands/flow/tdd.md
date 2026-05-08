Implement an issue using test-driven development.

Takes an issue number or reads from context.

1. **Read the issue**
   gh issue view <number>
   Understand what needs to be built and how it will be tested.
   Read the design in the issue body if present.

2. **Read context**
   - Read the golden paths listed in the issue
   - Read the source files that will change

3. **Red**
   Write the test first. It must fail for the right reason.
   Test through the public interface — no internals.

4. **Green**
   Write the minimum code to make it pass. No more.

5. **Refactor**
   Clean up without breaking the test. Follow existing patterns.

6. **Repeat**
   One behaviour at a time until the acceptance criteria are met.

7. **Done**
   gh issue close <number>
