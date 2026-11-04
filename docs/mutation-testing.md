# Mutation feedback for Go tests

Use mutation testing while writing or improving tests: it deliberately changes
production code and reports which changes the tests failed to detect. Passing
ordinary tests or high coverage alone do not establish assertion quality.

## Run it

```sh
nix develop --command make mutate PKG=./pkg/pagination
```

Inside `nix develop`, just run `make mutate PKG=./pkg/pagination`.
Choose **one explicit package**, not `./...`. Test-only changes should still run
against the production package they exercise. The command scans the whole selected
package, not only changed lines, and runs that package's tests, not every dependent
package's tests.

Nix supplies Go, pinned Gremlins v0.6.0, Make, Bash, jq, coreutils and Git. No global
`go install` or application dependency is needed. The first shell entry may build
Gremlins; its source and vendored dependencies have fixed hashes in
`nix/gremlins.nix`. A small local patch distinguishes build failures from test
failures using Go's structured test events: an uncompilable mutant must not count
as an assertion kill. `flake.lock` remains unchanged.

## Read the feedback

- **SURVIVED**: tests still passed after a mutation. The report shows its operator,
  source location and original source line. Ask which input or meaningful assertion
  would distinguish correct behavior from the mutation, improve the test, and rerun.
- **UNCOVERED**: package-local tests did not exercise this mutation location. Add a
  relevant case, or check whether coverage lives in another package. Multiple
  operators at the same uncovered location are listed once in the console.
- **INCOMPLETE**: baseline/test/tool failure, time budget exhausted, missing or
  malformed report, unsupported status, or timed-out/nonviable/skipped mutants. This
  is not evidence that assertions caught those mutations. Inspect the logs.
- **CLEAN**: every generated candidate was killed. This is evidence of sensitivity
  to those mutations, not proof of correctness or of every test's usefulness.

Do not change correct production behavior or weaken the tool's settings just to
clear findings. Equivalent mutations can be explained rather than chased with
meaningless assertions. Expected results should follow the behavioral contract,
not repeat the implementation under test.

Gremlins JSON contains mutation operator and location, **not an exact patch**.
The runner does not invent a before/after diff. Reports and logs are retained in
unique ignored directories:

```text
.artifacts/mutation/pkg/pagination/run.XXXXXX/
  baseline.log    # package discovery / ordinary tests
  package.json    # selected package and active Go files
  config.yaml    # explicit empty config; personal config is not inherited
  gremlins.log   # complete upstream output, including kills
  report.json    # machine-readable results, when generated
```

## Defaults and exit status

Ordinary tests must pass before mutations run. The runner preserves existing
`GOFLAGS` and appends `-count=1` so test results are not reused; Go's build cache
remains enabled. It uses two workers and timeout coefficient 20. The known-broken
upstream `--test-cpu` flag is not used. Personal `GREMLINS_*` variables/configuration
are ignored to prevent accidental dry runs, diff filtering or operator changes.

Package discovery, ordinary tests and mutation execution share a **300-second**
budget, with up to five seconds to kill lingering processes. Override explicitly
for a larger package:

```sh
MUTATE_TIMEOUT=600 nix develop --command make mutate PKG=./pkg/manager
```

Large packages can have hundreds of mutants. Running out of time yields incomplete
feedback, not a clean result. No automatic retries or random sampling are used.

Direct script exit codes are:

- `0`: clean.
- `1`: surviving or uncovered mutants need attention.
- `2`: incomplete/error; no quality conclusion (also used for no candidates).

GNU Make returns its own failure code (`2`) for a failed recipe, so read the
printed classification rather than using Make's status to distinguish findings
from errors. For automation needing the distinction, invoke
`nix develop --command bash scripts/mutate.sh ./pkg/pagination` directly.

Mocks, `gen` and `sqlcdb` packages are rejected. Within a package, subdirectories,
`*.gen.go`, generated headers before the package declaration, and files inactive for the
current Go build context are excluded. Baseline tests still compile required
generated dependencies normally. Symlinked package paths, recursive patterns,
nested modules and module-root scope are intentionally unsupported.

## Check the runner itself

```sh
nix develop --command make mutate-selftest
```

This runs in a disposable module without external dependencies. A deliberately
weak test must leave mutations alive; boundary assertions must kill them, including
on a warm repeat. It also checks generated/inactive/child exclusions, isolation
from personal Gremlins settings, invalid scope, a real uncompilable string-arithmetic
mutation, and incomplete/error handling using controlled reports and a simulated
slow tool. It does not modify repository tests.
Run this after changing/upgrading the tool or runner.

The sentinel guards against common false-green mistakes, not every upstream
misclassification. Gremlins classifies subprocess exit codes; a future toolchain or
mutation can still expose limitations. Keep raw logs and investigate suspicious
all-killed results rather than treating a percentage as unquestionable truth.

No CI workflow or required percentage threshold is added. The purpose is a quick,
actionable local feedback loop. For tool comparisons and pilot findings, see
[the research report](mutation-testing-research.md).
