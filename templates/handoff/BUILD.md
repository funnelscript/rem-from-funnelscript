# Build brief: {{PRODUCT}} clone

This folder is a reverse engineering knowledge base of {{PRODUCT}}, made with [REM](https://github.com/alextnetto/res) and packed on {{DATE}}. Give it to a coding agent or a team to build a product that behaves the same way. Parts of it may be written in a language other than English.

## Read in this order

1. `STATUS.md`: what the study covered, what it did not, and why.
2. `clone/prd.md`: scope, launch blockers and build order.
3. `clone/barriers.md`: approvals, licenses and certifications to start before or while you build.
4. `data-model/`: `types.ts`, `erd.mmd` and one file per entity.
5. `screens/`, `components/`, `design/tokens.md`: the interface.
6. `rules/` and `clone/parity.md`: the behavior and its acceptance tests.
7. `api/endpoints.md`, `flows/`, `topics/`, `glossary.md`: supporting detail.

## How to build

- Follow the build order in `clone/prd.md`. Start the long approvals in `clone/barriers.md` on day one.
- Write each test in `clone/parity.md` as an automated test before you implement its behavior. A feature is done when its parity tests pass.
- Choose your own stack. `api/endpoints.md` describes the original product: reproduce the data and the behavior, not its URL scheme, unless you need compatibility.
- Statements tagged `[obs]`, `[net]` or `[doc]` are backed by evidence in `evidence/` or `sources.md`. Statements tagged `[inf]` are inferences with an open question in `questions.md`: confirm them with the owner or make them easy to change.
- Every file ends with "Not tested". Do not invent behavior for those items; list them as open.
- Use the screenshots in `evidence/` as a reference for layout and states.

## Do not

- Copy the original product's code, icons, illustrations, logos, brand names or long texts. Write new copy and draw new assets.
- Use the original product's name or trademarks.
- Ship a feature that `clone/barriers.md` marks as needing an approval or a license before that approval or license is in place.

## Report back

When you finish, list what you could not build from this knowledge base, with the file and section that was missing or unclear.
