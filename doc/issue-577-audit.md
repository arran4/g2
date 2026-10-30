# Issue #577: guidance reconciliation and scoped implementation audit

Reviewed on 2026-10-30 for [PR #578](https://github.com/arran4/g2/pull/578).
Implementation evidence below refers to `main` commit
`4e23a1e5abbbd3003f2d84f4b4c19d4b02d60994`, merged into the existing PR branch.
This PR changes guidance and records findings; it does not change application
code, public APIs, tests or fixtures. This report is evidence for the separate
audit acceptance item in [#577](https://github.com/arran4/g2/issues/577), not a
claim that the issue or repository-wide compliance is complete.

## Acceptance and reconciliation

- Read the root `AGENTS.md`, `readme.md`, canonical `doc/g2.1.md`, relevant
  implementation/tests, release generation hook and CI checks. A tracked-file
  search found no nested `AGENTS.md` files.
- Retain #578's deliberate error handling, non-mutating candidate generation,
  generated-file ownership and explicit audit obligation. Incorporate #579's
  useful environment/process boundary and readable fixture guidance, including
  `fs.FS`, `fstest.MapFS`, txtar and focused OS/environment exceptions.
- Preserve [merged PR #574](https://github.com/arran4/g2/pull/574)'s comprehensive
  Unix manual-page policy verbatim: long references are acceptable; conventional
  sections are encouraged; size alone does not justify splitting pages. Retain
  CLI inspection, canonical man source, README ownership, anonymization,
  repository neutrality and the ephemeral `g2 dev` documentation exception.
- Avoid a contradiction with existing commands: `manifest verify --fix` and
  `--clean` explicitly authorize writes. Verification without write flags must
  remain non-mutating. Logging should explain operations and intentional skips
  without repeatedly logging the same returned error.
- [#572](https://github.com/arran4/g2/issues/572) already owns incremental parser,
  command, process-state and HTTP fixture cleanup, including the CWD regression
  test mentioned in its comments. Do not open another fixture-cleanup issue or
  replace real boundary tests wholesale.
- No command behavior changed, so `readme.md` and `doc/g2.1.md` remain unchanged.
  An extra contributor link would not materially improve this guidance-only
  change. Generated README files must not be edited by hand.

## Findings with implementation and test evidence

### Error context and caller-relevant identity

The pkg-desc-index file adapter wraps opening failures with `%w`
([pkg_desc_index.go:88–99](../pkg_desc_index.go#L88-L99)); its CLI recognizes
`fs.ErrNotExist` through `errors.Is`
([cmd/g2/pkg_desc_index.go:118–128](../cmd/g2/pkg_desc_index.go#L118-L128)).
The CLI's typed exit error has `Unwrap` and is classified using `errors.As`
([cmd/g2/main.go:22–36](../cmd/g2/main.go#L22-L36),
[148–157](../cmd/g2/main.go#L148-L157)). The Portage adapter has a useful
`errors.Is` assertion for its unavailable sentinel
([cmd_runner_test.go:46–55](../cmd_runner_test.go#L46-L55)); generic execution and
malformed-output tests check diagnostics at lines 58–80. These are reasons to
preserve existing contracts, rather than wrap every return mechanically.

Observed diagnostic gap: USE_EXPAND scanner failures propagate unchanged through
the directory adapter, without identifying the affected `.desc` file
([use_expand_desc.go:51–53](../use_expand_desc.go#L51-L53),
[98–109](../use_expand_desc.go#L98-L109)). A diagnostic probe using an injected
reader failure through `ParseUseExpandDescFS` confirmed `errors.Is` identity is
preserved but the returned message is only `injected read failure`, without
`broken.desc`. The same probe confirmed the wrapped pkg-desc-index missing-file
error still matches `fs.ErrNotExist`. This is a diagnostic improvement candidate,
not evidence of corrupt output or broken caller classification.

Coverage gap: the manifest commit-failure test injects `os.ErrPermission` but
asserts its text instead of `errors.Is`
([cmd/g2/manifest_verify_transactional_test.go:297–315](../cmd/g2/manifest_verify_transactional_test.go#L297-L315)).
USE_EXPAND injected failures generally assert only non-nil errors
([use_expand_desc_test.go:459–481](../use_expand_desc_test.go#L459-L481),
[541–547](../use_expand_desc_test.go#L541-L547)). Passing these tests does not
establish an error-identity or contextual-message contract for those paths.

### Candidate generation, explicit writes and failure preservation

Manifest verification reads inputs and rejects uncertain source resolution
before entering the explicit fix/clean paths
([cmd/g2/main.go:391–451](../cmd/g2/main.go#L391-L451)). Missing entries are
downloaded into an in-memory candidate; only a modified candidate is written
at the final commit point
([455–492](../cmd/g2/main.go#L455-L492)). The writer uses a sibling temporary
file, closes it, renames it, and cleans up its temporary resource on failure
([manifestutil.go:24–65](../manifestutil.go#L24-L65)).

Existing tests cover uncertainty, truncated responses and a successful first
download followed by a failed later download, checking unchanged Manifest bytes
([cmd/g2/manifest_verify_transactional_test.go:97–178](../cmd/g2/manifest_verify_transactional_test.go#L97-L178)).
They also cover final commit failure, exact final entries and repeated results
at lines 287–365. Real filesystem and HTTP integration are justified here because
the contract includes download failure and on-disk commit behavior. Byte equality
alone does not prove that no transient write was attempted; these tests do not
establish every possible OS failure or crash-durability guarantee.

Cache integration uses in-memory filesystems and mutation spies to check
unsupported metadata and missing Portage capability without creating cache
directories or touching existing entries
([cmd/g2/cache_integration_test.go:443–521](../cmd/g2/cache_integration_test.go#L443-L521)).
Additional tests cover discovery/layout failure and denied existing-cache reads
([cmd/g2/cache_test.go:144–232](../cmd/g2/cache_test.go#L144-L232)). Cache verification
separates explicit policy skips from errors
([cmd/g2/cache.go:149–180](../cmd/g2/cache.go#L149-L180),
[222–230](../cmd/g2/cache.go#L222-L230)). This is bounded evidence, not a claim
that all generation or dry-run paths have been audited.

### Fixture hygiene: existing #572 ownership

USE_EXPAND already exposes reader and `fs.FS` seams
([use_expand_desc.go:25](../use_expand_desc.go#L25),
[77–81](../use_expand_desc.go#L77-L81)), with in-memory parser/tree tests
([use_expand_desc_test.go:15–94](../use_expand_desc_test.go#L15-L94),
[128–181](../use_expand_desc_test.go#L128-L181)). Reuse these in a coherent #572
cleanup; preserve the file/directory adapter and real permission tests.

Two observed test defects belong in that cleanup:
`TestParseUseExpandDesc_Error` has no action/assertion
([use_expand_desc_test.go:297–301](../use_expand_desc_test.go#L297-L301)), and
`TestParseUseExpandDescFS_Error` discards its result without asserting anything
([359–372](../use_expand_desc_test.go#L359-L372)). The latter uses `MapFS` mode
bits which do not exercise OS permission denial. Actual injected ReadDir, Open
and reader failure tests exist at lines 459–481 and 541–547; the empty tests do
not mean those entire paths are untested. The real permission test tolerates
success when permissions cannot be enforced
([485–506](../use_expand_desc_test.go#L485-L506)); a pass alone is not universal
permission-denial evidence.

The overlay update regression changes global CWD and uses the wall clock
([cmd/g2/overlay_ebuild_test.go:12–71](../cmd/g2/overlay_ebuild_test.go#L12-L71)).
Keep a real relative-layout assertion while extracting incidental date/content
setup incrementally, as #572 already requests. Declarative HTTP fixture routing
is another existing #572 candidate
([pipeline/evaluator_test.go:20–81](../pipeline/evaluator_test.go#L20-L81),
[cmd/g2/pipeline_test.go:13–56](../cmd/g2/pipeline_test.go#L13-L56)). No new
fixture-cleanup ticket was created.

### Confirmed separate manual-generation defect

The canonical source starts with `# NAME`
([doc/g2.1.md:1–5](g2.1.md#L1-L5)). Running the release hook's converter
([.goreleaser.yml:4–6](../.goreleaser.yml#L4-L6)) succeeds but generates:

```roff
.nh
.TH NAME
g2 \- Gentoo overlay and repository maintenance utilities
```

The output lacks the manual name/section in `.TH` and has no `.SH NAME` section.
The source is byte-for-byte identical to current `main`, so this predates #578;
successful converter exit is not sufficient semantic validation. An open-issue
search for `repo:arran4/g2 is:issue man page` returned no matches during review.

Propose a separate focused fix for approval: give the maintained source a proper
converter title identifying `g2(1)` while preserving its NAME section and all
reference content; add a generated-output regression requiring the correct
`.TH` and `.SH NAME` directives. Do not commit generated roff or shorten/split
the manual to work around the header. This PR records the defect without
silently expanding its implementation scope or opening an unapproved ticket.

## Verification performed

Environment: Go 1.26.0, linux/amd64, non-root UID 1000. `portageq` is unavailable.
All commands below ran on the reconciled branch; no standalone g2 binary was
built. Temporary probes/output/logs stayed under `/tmp`.

| Check | Result |
| --- | --- |
| `go test ./...` | Passed across all packages. |
| `go vet ./...` | Passed with no diagnostics. |
| `go test ./... -count=1 -v -run 'Test(ParseUseExpandDesc\|WriteUseExpandDesc\|WriteFileUseExpandDesc\|OSExecPortageContext\|CmdVerifyTransactional\|Cache.*\|GenerateCache.*\|DoCache.*\|GenerateSite.*\|OverlayEbuildMove_Regression_Issue478)'` | Passed; uncached run of the audited test groups. Packages without matching tests reported that explicitly. No skip or permission-bypass log was observed in this run. |
| `go run ./cmd/g2 -h` | Printed the real top-level command tree, preceded by `Unknown command -h`; exited 0. Used for inspection, not proof of help UX correctness. |
| `go run ./cmd/g2 overlay site generate -h`; `go run ./cmd/g2 manifest verify -h`; `go run ./cmd/g2 cache verify -h`; `go run ./cmd/g2 site serve -h` | Passed; inspected actual generation, write, mode and server flags. |
| `go run ./cmd/g2 overlay site generate -work-mode temp -out /tmp/g2-pr578-site testdata/test_overlay` | Passed using committed local fixtures, without a remote checkout. |
| Serve that output with Python `ThreadingHTTPServer` on loopback; GET `/`, `/repos/test-overlay/`, `/news/`, `/news/index.rss`, `/news/index.atom` | All returned HTTP 200 and nonempty output; RSS and Atom parsed as XML. No `file://` directory navigation was used. No browser/visual claim is made. |
| SHA-256 snapshots before/after site generation | All 156 files under `testdata` and `templates` unchanged. This checks contents, not every possible metadata side effect. |
| Temporary Go diagnostic probe described above | Confirmed reader error identity, missing USE_EXPAND filename context and wrapped missing-index classification. No probe code or fixtures committed. |
| `go run github.com/cpuguy83/go-md2man/v2@latest -in doc/g2.1.md -out /tmp/g2-pr578.1` | Converter v2.0.7 exited 0. Semantic header check failed as described above; confirmed pre-existing on unchanged canonical source. |
| Compare the comprehensive man-policy paragraph with `origin/main:AGENTS.md`; `git diff --check` | Passed; #574 policy preserved verbatim and patch whitespace clean. |

No build/test environment failure blocked the Go checks. Real Gentoo/Portage
acceptance was not attempted because the capability is absent; mocked success
does not establish active-profile behavior. That broader work already belongs to
[#564](https://github.com/arran4/g2/issues/564), with a manual acceptance procedure
in [readme.md:389–408](../readme.md#L389-L408). No remote-network site integration,
cross-platform/race suite, complete API audit or crash-consistency audit is
claimed.

## Follow-up and readiness

1. Continue fixture/process-state cleanup and meaningful negative assertions
   under #572, one cluster at a time, preserving genuine OS boundary coverage.
2. Under #577, consider a separate bounded USE_EXPAND diagnostic change: first
   add a failing regression requiring file/operation context and preserved
   injected error identity, then wrap at the adapter boundary. Keep missing-file
   behavior and public signatures unchanged. Approval is needed before opening
   a new implementation workstream; this is not a repository-wide wrapping task.
3. Approve/schedule the separate manual-header fix above. Retain #564 ownership
   for Portage/profile validation. No duplicate issues were opened.
4. Review this report as the separate #577 acceptance item and decide the scope
   and ownership of remaining work. Leave #577 open; guidance alone and this
   scoped report do not establish complete resolution.

Recommendation: #578 can leave draft as a guidance/review-report PR once current
CI is acceptable; the recorded pre-existing defects are separate work, not
unfinished edits in this PR. #579 can be closed as superseded: its useful rules
are reconciled here, while its `Fixes #577` claim and loss of #574 guidance must
not be carried forward. Neither readiness nor closing #579 authorizes merging
#578 or closing #577. This update does not change PR/issue states or enable
automatic merging.
