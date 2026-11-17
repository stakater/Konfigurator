# <Story code> — <short title>

<!--
The planning doc is a CONTEXT CAPSULE (method.md, DX.1): a cold session with no
memory and no conversation history must be able to execute this story from this
file alone, without re-scouting the codebase. Fill every section; write "none"
rather than deleting one — an explicit "none" is information, an absent section
is a question. Delete this comment block in the real doc.
-->

Story: `docs/roadmap.md` <Story code> (status must be 👍 Approved before this
doc is implemented). Driver: <the feedback, audit finding, or need that
prompted it — cite ids/links>.

## Why (one paragraph)

<What breaks or is missing today, for whom, and why now. If the story came
from an audit or review, name the incident/finding ids.>

## Governing constraints

<The ADRs and CLAUDE.md invariants this change must respect, by number, with
one clause each on how they bite here. E.g. "Invariant 2: the render
context is read from the informer cache — no new hand-rolled mirror.">

## Design

<The change, at seam level: which modules, which files, which interfaces —
exact paths. New types/fields shown as snippets. Wiring: what the composition
root connects. State explicitly what is NOT changing.>

## Tests (red first)

<The failing tests to write BEFORE implementing: test file, seam, and the
behavior each pins — including the negative/regression cases that must stay
green. If a live incident motivated the story, reproduce it as a fixture.>

## Out of scope

<What this story deliberately does not do, and where that work lives instead
(a follow-up story code, an ADR to write first, or "accepted limitation" with
the honest consequence stated).>

## Done means

<Restate the roadmap Done line, plus anything mechanical: make verify green,
CRDs/chart regenerated if the API changed, docs updated (which ones), review
finding id(s) closed in docs/reviews/ if this is a review fix.>
