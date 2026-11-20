# GLEP 42 News Corpus Research & Golden Fixtures (#610)

## Corpus Scope & Split
Evaluated **144** real GLEP 42 news files across 4 sources (split deterministically 70/30 by SHA-256 of `repo:filename` into **102 train** and **42 holdout** files):
- `gentoo-mirror/gentoo` (`metadata/news/`): 128 files (en: 120, pl: 5, ru: 3)
- `arran4/arrans_overlay` (`metadata/news/`): 11 files (en: 11)
- `anyc/steam-overlay` (`metadata/news/`): 1 file (en: 1)
- `g2` repository `testdata`: 4 files (en: 4)

Era distribution across the 144 files:
- `2015-2019`: 17 files
- `2020-2023`: 63 files
- `2024-2026`: 64 files
- Formats: `2.0`: 142 files; missing format header: 2 files

## Empirical Findings

### 1. Heading Dialects & Shell Comments
Across all 144 files:
- **Setext underline headings (`Title\n======` / `Title\n------`)**: 116 occurrences across **34 files** (train: 21, holdout: 13). Note edge case: a multi-line prose paragraph ending with `.` followed by `---` (e.g. `...still applies.\n---`) is a horizontal separator, not a heading.
- **Markdown `#` / `##` headings**: 5 distinct articles (e.g. `2021-10-18-libxcrypt-migration-stable`, `2022-12-27-alternatives-introduction` `.en`/`.ru`, `2023-01-01-hardening-fortify-assertions`, `2026-03-15-postfix-database-change`, `2026-03-31-varnish-renamed`), all after 2021.
- **Shell `#` comments / root prompts** (must NOT become headings): 10 articles contain unindented `# emerge ...`, `# eselect ...`, `# cd ...`, `# ln -s ...`, `# nano ...`, `# https://...`, or consecutive `# ...` comment lines — including mixed files that have both `## Section` headings AND `# emerge ...` root commands (`2021-10-18-libxcrypt-migration-stable`, `2022-12-27-alternatives-introduction`).
- **MediaWiki `== Heading ==`**: 1 article (`2026-02-11-sssd-2_12-keywording`), including asymmetrical `== openrc users ===`.
- **Rejection of fixed date cutoffs**: Neither `#` headings nor `==` headings follow a `<=2014` date split; Setext underlines appear from 2021–2026, `#` headings appear from 2021–2026, and `==` appears in 2026. Document-local structural evidence must decide dialect and per-line classification.

Evaluating the structural heading classifier across all **144** train + holdout articles yielded **0 false positives** and **0 false negatives** on `#` headings vs. shell comments:
- Reject lines with leading indentation (`>= 1` space for verbatim/code, preserving `*`/`-` list continuation).
- Reject lines matching shell commands/flags/URLs/assignments (`emerge`, `eselect`, `cd`, `cp`, `mv`, `rm`, `ln`, `sln`, `mkdir`, `grep`, `echo`, `nano`, `systemctl`, `rc-update`, `FEATURES=`, `/etc/`, `https://`, etc.).
- Reject `#` lines adjacent to another `#` line (comment block) or ending in `.` / `!`.
- Require a blank line (or start of body) before candidate `#` headings, and for single `#` require either a following blank line or a following list block (with `len <= 80`).

### 2. Footnotes & References
- **58 files** (train: 40, holdout: 18) contain **167** `[N] <url/text>` reference definitions; **61 files** contain inline `[N]` citations (including consecutive `[0][1][2][3]`).
- Definitions appear at paragraph boundaries (`[0] https://...` or `[1] https://... description`) outside code blocks. Unmatched references in prose/code must remain literal text.

### 3. Indentation & Code Blocks
- **70 files** use 2+ space indentation for code/verbatim blocks, while **5 files** (including `2026-03-31-varnish-renamed`) use **1-space** indentation (` varnishd -> vinyld`). GLEP 42 permits 1+ space indentation for non-list lines; preserving exact `RawLines` and treating 1+ space non-list lines carefully avoids losing verbatim formatting.

### 4. Autolinks (`#598`)
- **218** prose `http://`/`https://` URLs observed; **6** have trailing punctuation or unmatched closing parentheses (`https://bugs.gentoo.org/805077).`, `https://www.gentoo.org/support/)`).
- **399** prose `cat/pkg` matches and **9** backtick-quoted atoms observed, plus **44** unindented `emerge ...` command lines (train: 32, holdout: 12).
- Backtick spans (`` `cat/pkg` ``) and indented code blocks must NEVER be autolinked; prose atoms must only link when present in parsed overlay inventory.

## Golden Anonymized Fixtures
All fixtures under `testdata/news_corpus/cases/` use anonymized identities (`Jane Doe <jane@example.com>`, `example.com`) and map to expectations in `testdata/news_corpus/cases.json`.
