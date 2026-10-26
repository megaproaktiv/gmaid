# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2026-10-26

### Added

- **Gantt CSS styling** via an external stylesheet (`gmaid -C style.css`,
  mmdc-compatible). New package `gcss/` parses a small CSS subset and colors
  task bars by id. Supports attribute selectors `[id=]`, `[id^=]`, `[id$=]`,
  `[id*=]` (matching the SVG id mermaid assigns, `my-svg-<taskid>`),
  comma-separated selector lists, `fill:`/`stroke:` declarations, `!important`,
  and source-order precedence. A `-c` config flag is accepted and ignored for
  mmdc compatibility.
- **`<br>` line breaks** in gantt section names and task labels (`<br>`,
  `<br/>`, `<br />`, `\n`), rendered as multiple lines.
- **Automatic word-wrap** of section names that do not fit the left column,
  with a configurable extra line spacing (`LineSpacingPct`, default +20%). Band
  height grows to fit the wrapped label and bars are vertically centered.
- **Combining macron / overline** (U+0304 / U+0305) rendered as a drawn
  overline above the base glyph (e.g. `p̄`), since the built-in font has no
  glyph for the combining marks.
- **Dependency arrows** (gmaid extension): `link <from> [FS|SS|FF|SF] <to>`
  (also `-->`), drawn as real orthogonal arrows with arrowheads.
- **Summary / rollup tasks** (`summary` tag): MS-Project-style summary bars
  (slim spine with downward triangle end-caps). Honors CSS fill override so a
  summary can keep any color.
- **Same-line tasks** (`&` label prefix or `sameline` tag): render a task on
  the same row as the previous one, so e.g. weekly/monthly sums share one line.
  Labels on shared rows stay centered over their own bar.
- Wider left text column and landscape-leaning default chart width for
  A4-landscape-friendly output.

### Fixed

- Canvas height no longer leaves large empty space when same-line tasks are
  used (height is computed from the real number of rows, not task count).
- Bar labels are drawn in a final pass after all bars and dependency arrows so
  text always stays on top of blocks.

### Tests

- `gcss/gcss_test.go` (selectors, `!important`, source order) and
  `gparser/gparser_ext_test.go` (link / summary / sameline / `<br>`).
- New gantt feature reference files in `testdata/` (`gantt-200`..`gantt-240`).

## [0.5.5] - 2026-10-13

### Added

- **`GMAID_SCALE` environment variable** to set the output resolution / scale
  factor without changing the command invocation. Precedence: an explicit
  `-scale` flag wins; otherwise `GMAID_SCALE` is used if set; otherwise the
  default of `2`. Invalid or non-positive values are ignored with a warning.
  Documented in the readme with DPI guidance (1.5 ≈ 330 DPI, 2 ≈ 450 DPI,
  3 ≈ 675 DPI).

## [0.5.0] - 2026-10-13

### Added

- **GitGraph (Git) diagram** rendering (`gitGraph`) — new packages
  `gitmodel/ gitparser/ gitlayout/ gitrender/`.
  - Parser for `commit` (with `id:`, `tag:`, `type:`), `branch`,
    `checkout`/`switch`, `merge` (with `tag:`/`type:`) and `cherry-pick`, with a
    default `main` branch and `%%` comments ignored.
  - Lane layout: one horizontal lane per branch in creation order, commits
    placed left to right by global sequence, with parent and merge connectors
    drawn as rounded elbows between lanes.
  - Renderer: colored branch label boxes, thick colored connectors, filled
    commit circles (ringed for merges, squares for highlights, crossed for
    reverses), tag boxes with bullets, and rotated commit ids (auto/merge ids
    suppressed to match mermaid).
  - `compare-git.sh` fidelity harness reporting both aspect-ratio and
    scale-normalized size error vs the `mmdc` references.
- `gitparser` unit tests covering the test data and option parsing.

## [0.4.0] - 2026-10-13

### Changed

- Module path renamed to `github.com/megaproaktiv/gmaid` so the tool is
  installable via `go install github.com/megaproaktiv/gmaid@<version>`.
- `govulncheck` now runs before every build (Taskfile and GoReleaser hooks).
- Documented the Go 1.27.2 minimum as a security requirement.

## [0.3.0] - 2026-10-13

### Added

- **Gantt chart** rendering (`gantt`) — new packages `gmodel/ gparser/
  glayout/ grender/`.
  - Parser for `title`, `dateFormat`, `axisFormat`, `tickInterval`, `section`
    and tasks with tags (`done`, `active`, `crit`, `milestone`), explicit dates,
    durations (`ms/s/m/h/d/w/M/y`), sequential defaults, and `after`/`until`.
  - Date resolution (dayjs-style `dateFormat` → Go layout) with chart range.
  - Time-scale layout: linear date→x axis, one row per task, section bands,
    tick gridlines, milestone diamonds and a today marker.
  - Renderer: centered title, alternating section bands, bars coloured by tag
    (active/crit/done/normal), milestone diamonds, date axis and red today line.
- Gantt anonymised test data (`testdata/gantt-100.mmd`, `testdata/gantt-110.mmd`).

## [0.2.0] - 2026-10-13

### Added

- **Sequence diagram** rendering (`sequenceDiagram`) — new packages
  `seqmodel/ seqparser/ seqlayout/ seqrender/`.
  - Parser for `autonumber`, `participant`/`actor` with `as` aliases, messages
    (`->`, `-->`, `->>`, `-->>`, `-x`, `--x`, `-)`, `--)`, `<<->>`) incl.
    self-messages and `+/-` activation, `Note over/left of/right of`, and
    blocks (`loop/alt/opt/par/critical/break/rect/box` with `else/and/option`).
  - Timeline layout: mirrored actor boxes, lifelines, label-proportional column
    spacing, autonumbered messages, notes and block frames.
  - Renderer: lavender actor boxes, lifelines, filled/open/cross arrowheads,
    rounded self-message loops, yellow notes, autonumber circles, block frames.
- 32 sequence diagrams in the test data.

### Changed

- Dispatch in `main.go` routes by the diagram's first keyword so sequence and
  flowchart pipelines coexist without touching the flowchart implementation.

## [0.1.0] - 2026-10-13

### Added

- **Flowchart** rendering (`flowchart`) — the original pure-Go pipeline in
  `model/ parser/ layout/ render/`. No cgo, no headless browser.
  - Parser for directions (`TD/TB/LR/BT/RL`), node shapes (rect, round,
    stadium, subroutine, cylinder, circle, diamond, hexagon, parallelogram,
    trapezoid), edges (solid/dotted/thick/invisible with arrow/open/circle/
    cross heads and labels), subgraphs with nested direction, and `style`
    directives. `click` and `%%` comments are ignored.
  - Sugiyama-style layered layout: cycle breaking, rank assignment, dummy-node
    routing, crossing reduction, coordinate assignment with chain straightening.
  - Renderer: node shapes, spline edges, arrowheads, subgraph boxes, per-node
    style fills/strokes/colours, PNG output via `gg`.
- `mmdc`-compatible CLI (`gmaid -i input.mmd -o output.png`) as a drop-in
  replacement, plus batch mode and a `Taskfile.dev.yml` (`build`, `render`).
- 17 flowcharts in the test data.

[Unreleased]: https://example.com/gmaid/compare/v0.5.5...HEAD
[0.5.5]: https://example.com/gmaid/releases/tag/v0.5.5
[0.5.0]: https://example.com/gmaid/releases/tag/v0.5.0
[0.4.0]: https://example.com/gmaid/releases/tag/v0.4.0
[0.3.0]: https://example.com/gmaid/releases/tag/v0.3.0
[0.2.0]: https://example.com/gmaid/releases/tag/v0.2.0
[0.1.0]: https://example.com/gmaid/releases/tag/v0.1.0
