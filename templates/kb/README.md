# {{PRODUCT}}: reverse engineering knowledge base

Start with `STATUS.md`. It says where the study stands and what comes next.

## How to read

- Every claim carries a tag: `[obs EV-###]` seen in the product, `[net EV-###]` network capture, `[doc SRC-###]` public documentation, `[inf]` inference with the question `Q-###` that would confirm it.
- Build only on `obs`, `net` and `doc`. Treat `inf` as a risk.
- IDs: `F` feature, `S` screen, `C` component, `FL` flow, `R` rule, `BR` production barrier, `E` entity, `B` finding, `Q` question, `SRC` source, `EV` evidence, `L` ledger row.

## Where things are

| Need | File |
|---|---|
| What the product does | `inventory/features.md`, `glossary.md` |
| What each screen looks like and does | `screens/`, `components/`, `design/tokens.md` |
| How the product behaves | `rules/`, `flows/`, `topics/` |
| What the data looks like | `data-model/`, `api/endpoints.md` |
| What to build | `clone/prd.md`, `clone/parity.md`, `clone/implications.md` |
| What could stop the launch | `clone/barriers.md` |
| How far the study went | `inventory/coverage.md`, `STATUS.md` |
| What was changed in the product | `sandbox/ledger.csv` |
