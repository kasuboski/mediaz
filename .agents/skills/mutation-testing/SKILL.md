---
name: mutation-testing
description: Mutation-test Go packages to assess test rigor and strengthen tests from surviving or uncovered mutants.
---

# Mutation feedback

Work from the repository root on one explicit package (`./pkg/library`, not `./...`).
Run formatting, Go tests and JSON queries through `nix develop --command`;
use its `gofmt` and `jq` rather than assuming host tools are installed.

1. **Measure.** Run:
   ```sh
   nix develop --command make mutate PKG=./pkg/library
   ```
   Substitute the target package. Save the printed artifact path as the baseline.
   Start with the concise findings; query `report.json` for details/comparisons.
   A nonzero exit can mean findings, not a broken command; read the classification
   rather than stopping at Make's exit code.
2. **Diagnose.** Read the affected implementation and tests. Account for every
   surviving mutant and uncovered location:
   - **Survived:** code was exercised but the mutation went undetected. Find an
     input or assertion that distinguishes the intended behavior.
   - **Uncovered:** add a scenario reaching that behavior, including error paths.
   - **Equivalent:** explain the invariant making the mutation unobservable;
     leave it visible rather than padding tests to chase the score.
   - **Incomplete:** inspect logs. A completed report with nonviable mutants can
     still contain actionable findings; timeouts or missing reports are not kills.
     For setup/status troubleshooting, read [runner docs](../../../docs/mutation-testing.md).
3. **Strengthen.** Extend the existing behavior tests in their usual `*_test.go`
   files; name new tests/files for the behavior, not mutation testing. Add focused
   cases with contract-derived expected results. Use realistic inputs and assert
   returned structs, errors or observable effects; use the repo's existing
   fixtures/mocks where appropriate. Keep correct
   production behavior and runner settings/exclusions unchanged.
4. **Verify.** Format changed Go tests with `nix develop --command gofmt -w <files>`
   and run package tests; both must succeed before rerunning the mutation command.
   Compare mutation locations/operators, not just percentages. Repeat until each actionable
   finding is fixed or explicitly left open with a reason. Run the ordinary Go suite
   after the final test changes:
   ```sh
   nix develop --command go test ./... -count=1
   ```
5. **Report.** List test changes, baseline/final artifact paths, mutation outcomes,
   and reasons for remaining survivors, uncovered code or incomplete results.
   Claim improvement only when the rerun demonstrates it; report budget/tool limits
   rather than claiming a clean run.
