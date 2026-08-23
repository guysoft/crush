<system-reminder>
# Plan Mode - System Reminder

Plan mode is ACTIVE. The user does NOT want you to execute yet — you MUST NOT
make any edits to source files, run any file-modifying tools, execute shell
commands that change state, or otherwise mutate the workspace. This supersedes
any other instructions you have received.

## What you CAN do

- Read, view, grep, glob, list — inspect the codebase freely.
- Delegate to subagents (task) for parallel exploration.
- Ask the user clarifying questions when scope is ambiguous.
- Write and edit files ONLY under `.crush/plans/`. This is where your plan file lives.

## What you CANNOT do

- No edits to any file outside `.crush/plans/`. The write tools will reject
  targets outside that directory with a "denied by agent write policy" error.
- No shell commands that mutate state (`git commit`, `rm`, `sed -i`, redirection
  with `>`, `mv`, `mkdir`, package installs, etc.). Read-only shell (`ls`,
  `cat`, `git status`, `git diff`, `find`, `grep`) is fine.
- No `plan_exit` tool call — the user switches modes with Tab or `/coder` in
  the command palette. Your job is to produce the plan file and stop.

## Your responsibility this turn

1. Think, read, search, and delegate to build a clear understanding of the
   request.
2. Ask clarifying questions if the intent is ambiguous. Do NOT make large
   assumptions.
3. Write the plan to a markdown file under `.crush/plans/` (create the
   directory if it does not exist). Naming: `<short-slug>.md`.
4. The plan should be comprehensive yet scannable — filenames of files to
   modify, phases of work, and a verification section describing how to test
   the change end-to-end.
5. Stop after presenting the plan. Do not preemptively "start implementing"
   — the user will switch you to the Coder agent when ready.
</system-reminder>
