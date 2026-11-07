This file contains reusable behavioural rules as relate to projects.

- The user requires correct and accurate solutions. Do not guess missing data. Instead, immediately report the missing data and stop work. The user will add the missing data. If you can't pull a source or reference, let the user know and they will pull it for you.

- Always write a plan and execute it. Do not conjure next steps out of thin air. If a plan is complete, report the plan complete and stop work.

- There can be only one live plan. Consolidate spent plans into the persistent record (`CLAUDE.md` or similar). Be sure not to lose significant decisions, traps, tricks or lessons learned. Remove old, obsolete plans.

- The periodically-read instructions file (`CLAUDE.md` or similar) must contain a rule to re-read this file (`RULES.md`) before starting work, to automatically check for behaviour-policy changes.

- You may create one and only one file in the plans directory. Once created, that file is your plan file for this project. Touch no other files in the plans directory, as they belong to unrelated projects which would be disrupted.

- `BOOTSTRAP.md` has the following structure:
   1. Current Goal & Next 3 Steps
   2. State of Play (Key Logic, Architecture, Decisions)
   3. Dependency Map & Version Log
   4. Golden Code Blocks
   5. Tested & Passing Status Confirmation

- The plan file is forward work only. Work which has landed goes in `BOOTSTRAP.md` §2, rules and traps go in `CLAUDE.md` (or equivalent), general reusable rules for AI behaviour go in `RULES.md`. Nothing is recorded in the plan that is not still to do — if it needs keeping, it belongs in one of those others.

- You have a signing key. Create signed commits **at will** but *never* push unless directly asked to push. The user is the boss of pushing.

- Commits are signed with the agent key, passed per-commit; never `--no-gpg-sign`, and never touch the host's signing config. A commit message says what was found, as well as what was done.

- Code should be no more than 70 columns wide with 8-character tabs. Functions should be no longer than 40 lines. These rules aid readability of functions for humans. This applies to **code**, but not documentation.

- Long text strings are exempt from the 70-columns rule. Keep text strings terse while not sacrificing meaning. They need to be searchable, so don't break them up. It might help to put them in separate modules for language translations, where that's appropriate.

- If you find out anything about other projects, don't mention them in this project. All projects are independent, and you might be leaking something private.

- The user needs to know what's going on, but be **very brief** to avoid inflating the prompt. When committing, output only the commit one-liner and hash. That will be sufficient to prompt the user to examine `git log`.

The following rules are guidance about token efficiency.

- Reduce waste. Tokens spent problem-solving or providing meaningful output are not wasted.

- When chat sessions get long (>20 responses), generate a handoff prompt for the user to start a new chat session. Plans should be generated with anticipated handoff breaks and remind the user to handoff.

- Keep chat responses succinct, since they're always fed back. Example: Don't put whole git commit messages into the chat. The hash and summary are plenty. I can read the whole message in `git log` or on GitHub after the push.

- Once the problem is sufficiently understood and the solution is verified to the required standard, stop investigating.

- When deciding whether another action is worthwhile, consider whether it is likely to reduce important uncertainty, uncover a better solution or verify correctness. If none applies, avoid the action.

- Prefer concise reasoning and tool usage, but investigate as much as necessary to solve the task reliably.

- Prefer targeted retrieval when practical, but inspect additional context whenever it could materially improve the solution.

- Prefer the simplest solution that adequately solves the problem, but do not reject a more sophisticated solution when the problem genuinely requires it.

- When uncertainty is significant, spend tokens reducing that uncertainty rather than guessing.

- Optimize for successful completion and solution quality first; token efficiency is a secondary objective.

- Avoid duplicating information already in context. Prefer to provide an explanatory comment and link to the previous response or the persistent record.

- Avoid repeated attempts that are unlikely to provide new information. When an approach fails, learn from the result before retrying. Use alternative approaches when the current approach is failing or when another approach has a reasonable chance of producing a better result.

- Do not impose an arbitrary limit on the number of investigative steps. Continue while additional investigation has a reasonable expected benefit.

- Keep tool calls and responses focused on information relevant to the task. Do not spend tokens narrating routine actions.
