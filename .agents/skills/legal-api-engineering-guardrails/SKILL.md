---
name: legal-api-engineering-guardrails
description: Shared Legal API engineering workflow for diagnosis, feature slicing, prototype use, architecture decisions, and handoff. Use with every Legal API subsystem skill before or during code changes, especially when a task is ambiguous, bug-like, high-risk, or cross-cutting.
---

# legal-api-engineering-guardrails

## When To Use This Skill
Use this skill with the owning subsystem skill for every non-trivial Legal API task.

It is not a replacement for subsystem ownership. It supplies the shared execution discipline that every role should apply before editing, while editing, and before sign-off.

## Core Operating Rules
- Start from [../../../AGENTS.md](../../../AGENTS.md) and the owning area guide.
- Classify every request as `bug` or `feature` before implementation.
- Prefer the smallest vertical slice that proves behavior through the public interface.
- Use existing project vocabulary. If `CONTEXT.md`, `CONTEXT-MAP.md`, or `docs/adr/` exists, read the relevant entries before naming new concepts or revisiting architecture.
- Create or update docs only when the decision is durable and directly useful. Do not add ceremony for routine implementation details.

## Bug Workflow
1. Build a fast feedback loop before fixing:
   - focused Go test, frontend test, build check, HTTP request, browser flow, compose verify, or a narrow throwaway harness
   - if the failure is nondeterministic, raise its reproduction rate with repeated runs or a stress loop
2. Reproduce or clearly state why the exact user-visible failure cannot be reproduced.
3. Rank 3 to 5 falsifiable hypotheses for complex bugs before changing code.
4. Instrument only where it tests a hypothesis. Tag temporary logs with a unique `[DEBUG-*]` prefix and remove them before finishing.
5. Add a regression test at the nearest correct seam when one exists.
6. Re-run the original feedback loop and the closest regression-prone neighbor.

## Feature Workflow
1. Define the new contract or workflow first:
   - backend request/response shape
   - frontend route, state, and API-wrapper impact
   - worker, vector, or deploy runtime assumptions
2. Implement one vertical slice at a time:
   - one behavior
   - one test or verification signal
   - minimal production code
3. Do not bulk-write tests for imagined behavior. Let each test or check respond to what the previous slice taught you.
4. Verify the happy path plus adjacent existing flows that share the touched contract.

## Prototype Rule
Use a prototype only when it answers a concrete question faster than production edits.

- Keep it clearly marked as throwaway.
- Put it near the area it informs.
- Make it runnable with one existing project command when possible.
- Avoid persistence unless persistence is the question being tested.
- Delete it or fold the result into production code once the question is answered.

## Architecture And Decision Rule
- Prefer deep modules: small interface, useful behavior hidden behind it.
- Apply the deletion test before adding an abstraction. If deleting the module just moves the same complexity to callers, it is not earning its keep.
- Treat the interface as the test surface.
- Record an ADR only when the decision is hard to reverse, surprising without context, and the result of a real trade-off.

## Handoff Rule
If work stops before completion, leave a concise handoff with:
- task classification and lead skill
- changed or inspected paths
- current hypothesis or implementation state
- exact verification already run
- remaining risks and next command to run
