# Mutation testing for Mediaz

Research snapshot: 2026-11-04. Repository: `846db86e06938a138ff305f19f076ebf36e987b3`. Scope: Go tests; no frontend mutation testing evaluated. No CI, product code, dependencies, or repository tests changed.

The local command is now implemented; see [usage and safeguards](mutation-testing.md).
The observations below describe the original research run against unpatched upstream
v0.6.0. The Nix-packaged command additionally patches build-failure classification.

## Recommendation

**Pilot pinned Gremlins v0.6.0 as an advisory check; do not immediately make repository-wide mutation testing a required gate.** It works with Mediaz's Go 1.26 toolchain and has useful coverage filtering, parallelism, changed-line selection and native thresholds. However, this investigation reproduced timeout instability and a CLI argument bug. Harden the runner and verify its results before requiring it.

Start with `pkg/pagination` and `pkg/machine`, then selected business-rule files in `pkg/manager`. Run broader handwritten-code scans weekly or manually. Consider a required, narrowly scoped PR check only after observing stable runs and manageable survivor review. Mutation testing demonstrates sensitivity to the faults its operators generate; it does not prove complete correctness or replace integration tests, race detection, fuzzing or review.

## Tool options

These are mainly developer CLIs, not libraries to add to the application's runtime dependencies. Popularity was not quantitatively ranked.

| Tool | Evidence and strengths | Trade-offs / recommendation |
| --- | --- | --- |
| [Gremlins](https://github.com/go-gremlins/gremlins) | Apache-2.0; latest release v0.6.0 (2025-12-06), default-branch activity in March 2026. Coverage prepass, workers, package-local execution, git changed-line scope, JSON, efficacy and coverage thresholds. Release requires Go 1.25. | Best starting CLI **with the safeguards below**. No documented persistent mutation-result cache. Still 0.x; pin versions. Authors explicitly warn large modules can take hours. |
| [Avito go-mutesting](https://github.com/avito-tech/go-mutesting) | Maintained MIT fork; v2.3.1 (2025-12-26), branch activity October 2026. Broad AST operators, annotations, custom executor, JSON/HTML. Release declares Go 1.25.5. | Useful if operator breadth/custom execution matters. No native workers, changed-line selector, coverage-directed execution or score gate found in inspected CLI; needs more wrapper work. Mutates source in place: use disposable files. Main declares Go 1.27, so avoid unpinned installation. |
| [Original go-mutesting](https://github.com/zimmski/go-mutesting) | MIT; extensible AST mutators and custom executors. | Latest inspected default-branch commit and release are June 2021; old x/tools. Not archived, but not my choice for a new Go 1.26 rollout. |
| [Gooze](https://github.com/gooze-dev/gooze) | MIT; v1.4.2 (2026-11-03). Overlay execution, result/build caching, workers, diff mode, sharding, thresholds. | Worth a second spike if scale becomes a problem. Main declares Go 1.27.3; verify pinned release and toolchain integration. Cache invalidation, fixtures and subprocess builds need auditing. Not benchmarked here. |
| [gomu](https://github.com/sivchari/gomu) | MIT; v0.3.0 (2026-10-17). Incremental history, workers, reports, gates; main declares Go 1.26. | Promising newer alternative; incremental mode defaults on. Validate full-run behavior, cache correctness and build-tag handling. Not benchmarked here. |
| [Ooze](https://github.com/gtramontina/ooze) | MIT; Go-test-native library, custom mutators and threshold support. | Requires adding a mutation-tagged test harness/dependency. Default command runs the full suite per mutant. Less attractive than a standalone CLI for the first rollout. |

Dates and Go declarations are snapshot observations, not compatibility guarantees. Release and main may differ. Sources: official repositories above; [Gremlins release API](https://api.github.com/repos/go-gremlins/gremlins/releases/latest), [Avito release module](https://github.com/avito-tech/go-mutesting/blob/v2.3.1/go.mod), [Gooze CLI documentation](https://gooze.dev/docs/cli/run/). Only Gremlins was executed locally.

## What was actually measured

Environment: Linux amd64, 12 reported logical CPUs, Nix Go **1.26.10**, Gremlins **v0.6.0**, two mutation workers. Existing dependency/build caches were available. Tool installation/download and Nix startup are **not** included in the timings below. No paid service or model inference is required to run these tools.

Ordinary complete Go suite: `go test ./... -count=1` passed in **10.23 s** wall time. Individual package execution reported manager 6.382 s and indexer 8.542 s; packages overlap, so these are not additive wall times.

Valid mutation measurements used `GOFLAGS=-count=1`, `--workers 2`, `--timeout-coefficient 20`, and **no `--test-cpu`**:

| Target | Full tracked-source copy: wall time | Repeated minimal-copy runs | Killed | Survived | Uncovered | Timeouts / nonviable |
| --- | ---: | --- | ---: | ---: | ---: | --- |
| `pkg/machine` | 1.004 s | 0.729 / 0.773 s | 1 | 0 | 0 | 0 / 0 |
| `pkg/pagination` | 4.608 s | 5.493 / 4.454 s | 18 | 2 | 5 | 0 / 0 |

The minimal copy contained the unchanged `go.mod`, `go.sum`, and these two package directories. Full-copy results came from a disposable `git archive HEAD` extraction. Repeated minimal-copy runs had identical mutation locations/statuses, not identical durations/output ordering. This is a small warm-cache pilot, **not** a cold GitHub Actions benchmark or a whole-repository mutation run.

A full-source **dry run** of `pkg/manager` took 10.638 s and found **917 runnable + 176 uncovered = 1,093 mutation candidates** under default operators. It did not execute mutants. This package alone makes a blanket full-suite PR gate risky. A naive 917 × 6.382 s / 2 calculation is about 49 minutes, but this is only an illustrative cost model: fail-fast kills can be quicker, while compilation, resource contention and timeouts add cost. Measure actual CI distributions before promising a budget.

### Direct evidence that assertions matter

Two experiments changed tests **only in `/tmp`**, not the repository:

1. **Weakened state-machine assertions:** removed the assertions on `ToState`'s returned error, leaving calls and other assertions intact. Ordinary tests still passed with **100% statement coverage**. The sole conditional-negation mutant changed from **killed to survived**. Mutation testing caught a weakness coverage did not.
2. **Strengthened pagination tests:** the existing tests let two boundary mutants survive:
   - `pagination.go:9`: changing the overflow guard `>` to `>=` wrongly rejects the last safe offset. A case with `Page=math.MaxInt/2+1`, `PageSize=2`, expected offset `math.MaxInt-1` kills it.
   - `pagination.go:17`: changing `PageSize > 0` to `>= 0` allows division by zero when there are items but page size is zero. A zero-size/nonempty-total metadata case kills it.
   Adding these two table cases in the temporary copy changed **18 killed / 2 survived** to **20 killed / 0 survived**. Five uncovered mutants in the generic `Page` helper remained. Its behavior may be exercised by other packages, but the package-local scan does not credit that.

This is useful actionable evidence, not a claim that all other tests or code are sound. A 100% machine score is based on just one supported mutant.

## Reproducibility hazards found during the pilot

### 1. Do not use Gremlins v0.6.0 `--test-cpu`

The [release executor](https://github.com/go-gremlins/gremlins/blob/v0.6.0/internal/engine/executor.go) constructs `fmt.Sprintf("-cpu %d", m.testCPU)` as **one argv item**, rather than separate flag/value arguments or `-cpu=1`. It classifies command exit 1 as KILLED.

With `--test-cpu 1`, even the deliberately weakened state-machine test appeared to kill its mutant. Running the generated argument manually in the minimal module failed with `no Go files` at the module root and exit 1. Omitting the option correctly reported that mutant as LIVED. Early trial results using that flag were invalid and discarded. In particular, an initial 20/20 pagination result was a false positive; the corrected unchanged-test result is **18/20**.

Do not trust a green score until a known-surviving sentinel is actually reported as surviving. This is a confirmed local/source-level finding, not an upstream issue filed or a fix implemented here.

### 2. Default timeout calibration was unstable

Repeated default pagination runs reported **20 timeouts**, not assertion kills. The coverage prepass became very fast with warm caches; the executor's timeout is baseline elapsed time multiplied by a coefficient (default 3). That can be too short for recompilation/testing mutants.

Using `GOFLAGS=-count=1` to avoid test-result reuse and coefficient 20 produced the repeatable valid outcomes above. This setting is an experimental starting point, not a universal safe value. Go's build cache remains enabled; disabling test-result caching is not the same as disabling build caching. CI load can still change outcomes. Treat unexpected timeouts as inconclusive, not proof of assertion quality.

### 3. Diff scope is not complete test-impact analysis

`--diff REF` selects mutations on changed production lines, not every unchanged behavior affected by changed tests or dependencies. Test-only PRs need a selected-package scan without diff filtering. Do not quietly skip them.

The [v0.6.0 diff implementation](https://github.com/go-gremlins/gremlins/blob/v0.6.0/internal/diff/diff.go) treats an empty parsed diff as all positions changed: guard no-relevant-change PRs in the wrapper. Fetch base history/ref; this repo already checks out with `fetch-depth: 0`. Handle no candidates explicitly as “not applicable”, not evidence of 100% quality.

### 4. Interpret the denominators

[Release report implementation](https://github.com/go-gremlins/gremlins/blob/v0.6.0/internal/report/report.go):

- Efficacy = `killed / (killed + survived)`.
- Mutator coverage = `(killed + survived) / (killed + survived + uncovered)`.
- Timeouts, skipped and nonviable mutants are not included in those formulas.

Pagination's existing tests therefore score **90% efficacy and 80% mutator coverage**, not a single universal “90% correct” score. Even after the two temporary test improvements, 100% efficacy coexists with five uncovered mutants. Require visibility into all statuses and absolute counts, not just a percentage. Equivalent mutants require human judgment and narrowly documented exclusions, not meaningless assertions.

## How easy, fast, deterministic and cheap is this?

- **Easy to try:** one pinned tool installation and one command; no application dependency or external service needed. Reliable CI integration needs more work than the quick start suggests.
- **Fast for small packages:** seconds in this pilot. **Not proven fast for manager or the whole repository.** Each tested mutant triggers another build/test invocation; full-suite integration mode magnifies the cost.
- **Deterministic in intent, not guaranteed in outcomes:** mutation operators are reproducible for fixed source/config, but load-sensitive timeouts, timing-dependent tests and races can change classifications. Mediaz already has timer/scheduler/HTTP tests; ordinary suite success is not a flake audit. JSON order can vary; compare normalized `(file,line,column,type,status)` entries.
- **Cheap in licensing/service cost:** reviewed tools are open source and run locally. Compute time, CI quota, parallel-worker memory/disk and survivor review remain real costs. No exact dollar figure is justified without runner rates and measured CI runtimes.

## Proposed rollout and gate policy

1. **Advisory pilot:** pinned tool and Go environment; normal tests must pass first. Explicitly exclude mocks, generated clients under `gen`, sqlc output, and `*.gen.go`. This matches existing coverage exclusions in `.github/workflows/ci.yml`; additionally exclude any generated files identified by headers, even if their paths differ. Start with the two packages measured here. No blanket percentage gate.
2. **Validate the harness:** maintain known-killed and known-surviving sentinels; verify compile/setup failures cannot silently count as assertion kills. Never use the broken flag above. Record logs and JSON, normalize statuses, bound job runtime and worker count, and rerun unexpected timeouts once before reporting infrastructure instability. Do not repeatedly retry until green.
3. **Observe PRs for 1–2 weeks:** production changes use diff scope; test-only changes run the affected selected packages, falling back to the small whole allowlist when impact is unclear. Skip irrelevant changes explicitly. Separate new survivors, uncovered mutations, timeouts and nonviable counts in the report. Tool/operator/exclusion changes reset the baseline comparison.
4. **Then add a narrow required gate:** require explanation or a useful regression test for new, non-equivalent survivors/uncovered mutations on selected changed code. Start from reviewed package baselines, not an arbitrary global 100% demand. A wrapper is needed for “no new survivors”; native percentage thresholds alone are not that policy. A reasonable acceptance budget is p95 under ~5 minutes; that is a proposed target, not a measured result.
5. **Broader weekly/manual scan:** expand into manager acquisition/admission, quality inheritance, discovery and storage transactions; retain a bounded allowlist initially. Package-local execution may miss manager/server tests of storage helpers: assess `--integration` and `--coverpkg` deliberately rather than counting these as absent tests. Preserve `go test -race` separately. Schedule broad scans only after measuring cost; no schedule was created here.
6. **Reconsider tool choice if safeguards dominate:** evaluate Gooze/gomu on the same source/operator scope with cold/full and warm/incremental runs. Test fixture/dependency/toolchain cache invalidation. Do not assume marketing speed claims transfer to Mediaz.

The current CI publishes coverage successfully regardless of percentage; it is informational, not an existing coverage-quality threshold. Mutation results should initially be another distinct check/artifact, not silently turn that check into a repository-wide blocking requirement.

## Reproduce the small pilot

Run on a disposable copy if experimenting. From the repository's Nix environment (installation is separate from measured run time):

```sh
nix develop --command bash -c \
  'GOBIN=/tmp/mediaz-mutation-bin go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0'

nix develop --command go test ./... -count=1

nix develop --command bash -c \
  'GOFLAGS=-count=1 /tmp/mediaz-mutation-bin/gremlins unleash ./pkg/pagination \
    --workers 2 --timeout-coefficient 20 --output /tmp/pagination-mutants.json'
```

Repeat with `./pkg/machine`. Preserve any existing `GOFLAGS` when turning this into a maintained wrapper. No threshold is set in this advisory example: surviving mutants alone do not cause a failed quality gate. Native `--threshold-efficacy` and `--threshold-mcover` are available, but should only be selected after validating status accuracy and reviewing baselines.

## Further primary sources

- [Gremlins v0.6.0 CLI flags](https://github.com/go-gremlins/gremlins/blob/v0.6.0/cmd/unleash.go), [module requirements](https://github.com/go-gremlins/gremlins/blob/v0.6.0/go.mod), [README / release policy](https://github.com/go-gremlins/gremlins/blob/v0.6.0/README.md).
- [Avito executor](https://github.com/avito-tech/go-mutesting/blob/9ce5f731e32140feacf391dc683ff8bd32c3aa41/cmd/go-mutesting/main.go), [CLI configuration](https://github.com/avito-tech/go-mutesting/blob/9ce5f731e32140feacf391dc683ff8bd32c3aa41/internal/models/options.go).
- [Original latest branch commit API](https://api.github.com/repos/zimmski/go-mutesting/commits?per_page=1).
- [Gooze documentation](https://gooze.dev/docs/), [gomu documentation](https://github.com/sivchari/gomu), [Ooze documentation](https://github.com/gtramontina/ooze).

Local raw pilot logs/JSON are under `/tmp/mediaz-*` on the research machine; these are temporary, not committed artifacts. This document records the portable findings. No live media/download services were used.
