# gmaid — Project Memory (session re-entry doc)

Last updated: 2026-10-13. This file lets a fresh AI session resume work without
re-deriving context. Read it fully before changing code.

## 1. What this is

A **pure-Go** application (no cgo, no browser) that parses Mermaid diagrams and
renders them to PNG. Three diagram types are supported, dispatched by the first
keyword of the file:
- **flowchart** — hand-rolled parser + Sugiyama layered layout + gg rasterizer
  (packages `model/ parser/ layout/ render/`). This was the original work.
- **sequenceDiagram** — separate parser + timeline layout + gg rasterizer
  (packages `seqmodel/ seqparser/ seqlayout/ seqrender/`). Added WITHOUT
  touching the flowchart packages (user constraint).
- **gantt** — separate parser (with date resolution) + time-scale layout + gg
  rasterizer (packages `gmodel/ gparser/ glayout/ grender/`). Added WITHOUT
  touching the flowchart or sequence packages.

- Module: `github.com/megaproaktiv/gmaid` (see `go.mod`), Go 1.27.2. Renamed
  from bare `gmaid` on 2026-10-13 so `go install github.com/megaproaktiv/gmaid@latest`
  resolves (see §11). All internal imports use the full path.
- Working dir: `/Users/gernotglawe/letsbuild/2026/gmaid`
- Dependency: `github.com/fogleman/gg v1.3.0` (+ freetype + x/image), all pure Go.

## 2. How to build / run / verify

```bash
go build ./...
go vet ./...
go test ./...                 # parser + layout tests
go run .                      # renders all flowchart files in ./testdata -> ./output
go run . path/to/file.mmd     # render specific file(s) or dir(s)
go run . -scale 3 -out ./png testdata   # flags: -scale (default 2), -out (default "output"), -v
./compare.sh                  # renders all + prints aspect-ratio displacement vs mermaid refs
```

- Output PNGs go to `./output/` by default (NOT testdata — user requirement).
- `mmdc` (mermaid CLI) is installed at `~/.bun/bin/mmdc`; used to make references.

## 3. Package layout (packages are at PROJECT ROOT — user forbade `internal/`)

- `model/model.go` — data model: Graph, Node, Edge, Subgraph, Style, Shape,
  Direction, EdgeLineStyle, ArrowType. NOTE enum ordering: `ArrowNormal=0`,
  `ArrowNone=1` (so arrow==0 means a normal arrowhead). Shapes: Rect=0, Round,
  Stadium, Subroutine, Cylinder, Circle, Diamond, Hexagon, Parallelogram,
  ParallelogramAlt, Trapezoid, TrapezoidAlt.
- `parser/` — `parser.go` (line dispatch, headers, subgraphs, direction,
  style), `statement.go` (edge-chain splitting + operator classification +
  `splitAmp` quote/bracket-aware `&` split), `node.go` (shape detection, label
  cleaning, style props, header/direction helpers). `parser_test.go` tests the
  two examples + parses all 17 flowcharts.
- `layout/` — the engine:
  - `types.go` — Config (+ `DefaultConfig`), Point, NodeBox, SubBox, EdgePath,
    Diagram.
  - `sugiyama.go` — group struct; pipeline `solve()` = breakCycles (DFS reverse
    back-edges) -> assignRanks (Kahn longest-path) + `stackIsolated()` ->
    insertDummies -> ordering (barycenter, 8 iters) -> assignCoords. Has
    `labelExtent` field (rank-axis space reserved for edge labels).
  - `coords.go` — assignCoords: rank/cross axis mapping; **flow-dependent rank
    gap** (`rankSep *= 1.4` when horizontal); 24-iter mean-barycenter alignment
    of up+down neighbors; chain-straightening (6 passes); overlap resolution.
  - `engine.go` — Engine, recursive subgraph layout (subgraphBlock), solveGroup,
    edge routing (routeChain clips to real NodeBox via clipBox),
    `subgraphChildrenIsolated()` + `perpendicular()`.
  - `translate.go` — diagram/subgraph coordinate translation.
  - `layout_test.go` — smoke test on the two examples.
- `render/` — `render.go` (Theme, Renderer, Render = bg -> subgraphs -> edges ->
  nodes -> edge labels), `shapes.go` (node/subgraph shape paths + style
  application), `edges.go` (Catmull-Rom **spline** edges, arrowheads/circle/
  cross, dotted/thick/invisible, labels), `color.go` (parseHex #rgb/#rrggbb/
  #rrggbbaa + named).
- `textmetrics/textmetrics.go` — shared font loading (gofont goregular/gobold)
  + string measuring so layout sizes match rendered text. `SplitLines` splits
  labels on `\n`.
- `main.go` — CLI: collectFiles, renderFile, outputPath, **isFlowchart**.

## 4. Mermaid feature coverage (verified across 17 testdata flowcharts)

- Directions: TD/TB, LR, BT, RL (BT/RL via rank mirroring).
- Shapes: `[rect]`, `(round)`, `([stadium])`, `[[subroutine]]`, `[(cylinder)]`,
  `((circle))`, `{diamond}`, `{{hexagon}}`, `[/para/]`, `[\para\]`, trapezoids.
- Edges: `-->`, `---`, `-->|lbl|`, `-- text -->`, `-.->`, `-.->|lbl|`, `==>`
  (thick), `~~~` (invisible, influences layout but not drawn), `--o`, `--x`,
  multi-dash min-length (`MinLen`), chaining `A --> B --> C`, `A & B --> C`.
- Subgraphs: titles, nested, per-subgraph `direction`.
- `style NODE fill:/stroke:/stroke-width:/color:/stroke-dasharray` honored.
- `click`, `%%` comments, `classDef`/`class`/`linkStyle` safely ignored.
- Labels: `<br/>`, `<br>`, `\n`, quotes/backticks, HTML entities decoded.

## 5. CRITICAL constraints from the user (do not violate)

1. **No `internal/` package** — all packages at project root.
2. **Output to `./output/`**, never write PNGs into `testdata/`.
3. **Render ONLY flowcharts**: `isFlowchart()` requires the first non-empty,
   non-comment line's FIRST WORD (via `strings.Fields`) to equal exactly
   `flowchart`. The `graph` alias and all other diagram types are EXCLUDED.
   Verified excludes 32 sequenceDiagram, 2 gitGraph, 2 gantt files.
4. **Edges are splines** (Catmull-Rom), not straight polylines.
5. **No Chrome/headless/mmdc in the app** — mmdc is only a dev-time reference
   tool, never a runtime dependency.
6. Lane alignment across subgraphs is "ok" (user accepted it) — do NOT spend
   effort forcing cross-subgraph row alignment.

## 6. Layout fidelity status (metric + current config)

Metric = Σ |ref_aspect_ratio − mine_aspect_ratio| across 17 files, via
`./compare.sh` (compares `output/*.png` to `reference/*.png`; references are
regenerated with mmdc — see §8). Lower is better.

- History: 16.569 (start of tuning) -> 13.479 -> 10.844 -> 10.792 -> **7.391**.
- Current `DefaultConfig`: NodeHPad=18, NodeVPad=9, MinNodeW=40, MinNodeH=36,
  RankSep=105, NodeSep=28, SubPad=22, SubTitleH=26, FontSize=15, LineHeight=19,
  CharW=8.2, Margin=24. Plus `rankSep *= 1.4` for horizontal flows in coords.go.
- 14/17 files are within 0.3 displacement. 503 = 0.001 (near-perfect).

Current per-file displacement (worst first):
```
1.839 700-a2a-elevenlabs-01            ref=6.588 mine=4.749
1.839 630-a2a-elevenlabs-01            ref=6.588 mine=4.749
0.882 880-runbook-adding-a-new-flow-01 ref=0.797 mine=1.679
0.764 504-adr-raw-event-s3-archival-01 ref=2.058 mine=2.822
0.633 880-runbook-adding-a-new-flow-02 ref=8.809 mine=9.442
0.297 430-...                          ref=0.590 mine=0.887
0.287 840-development-backend-01        ref=1.571 mine=1.284
... (rest < 0.3) ...
0.001 503-adr-data-warehouse-integration-01 ref=2.069 mine=2.070
TOTAL_ERR=7.391
```

## 7. KNOWN ISSUES / OPEN WORK (ranked by impact)

### I1. Extreme LR chains too tall (630 / 700-a2a-elevenlabs) — BIGGEST error
- ref aspect ratio ≈ 6.59 (very flat/wide); mine ≈ 4.75 (too tall).
- Cause: disconnected components (main Caller→... chain vs the KMS key /
  Security profile feeder band) are spread across empty cross-axis rows.
  `resolveOverlaps` is PER-RANK, so there is no cross-rank vertical compaction;
  empty vertical bands remain between the two component bands.
- Main chains ARE now straight (chain-straightening works). The height is the
  inter-band gap, not staircasing.
- Possible fix: add a global cross-axis compaction that removes empty bands
  spanning all ranks, or lay out weakly-connected components and pack them
  tightly along the cross axis. Must not regress the 14 good files — measure
  with ./compare.sh after every change.

### I2. 880-runbook-01 TD spine drifts rightward (err 0.88)
- ref ar 0.80 (tall/narrow); mine 1.68 (too wide).
- Cause: branching diamonds (degree>2) pull their parent toward the MEAN of
  their fanned-out children, which extends rightward; drift propagates up the
  spine. Chain-straightening only triggers for strict 1:1 links, so it doesn't
  catch diamond spines.
- Tried & REJECTED: median-based alignment (both as the main pass and as a final
  refinement) — it made the TOTAL metric WORSE (7.391 -> ~8.3) and worsened
  880-01. Do NOT reintroduce median alignment without proving a net gain.
- Possible fix: priority/Brandes-Köpf-style alignment that picks a primary
  child to keep colinear with its parent; or a weak global anchor toward the
  layer centroid. Validate aggregate metric before keeping.

### I3. 504 LR slightly too wide (err 0.76)
- The `rankSep *= 1.4` horizontal multiplier over-widens LR graphs that have
  few/no edge labels. A smarter approach would scale rank gap by actual label
  presence per gap rather than a flat 1.4x for all horizontal flows.

### I4. TD vs metric tension (design note)
- Pure aspect-ratio optimization favors large RankSep, which makes TD diagrams
  look airy/sparse (observed at RankSep=140). Current 105 + 1.4x horizontal is
  a compromise balancing the metric against TD visual compactness. If you push
  the metric lower, re-check TD visuals (esp. 430) — do not let them get sparse.

### I5. Metric is a proxy
- Aspect-ratio displacement is a cheap structural proxy, not true node-position
  displacement. It can be gamed. For real fidelity, consider parsing mermaid SVG
  node coordinates and comparing positions — not yet implemented.

## 8. Reproducing the comparison harness

```bash
# (re)generate mermaid references:
mkdir -p reference
for f in testdata/*.mmd; do
  head -1 "$f" | grep -qE '^\s*flowchart' || continue
  base=$(basename "$f" .mmd)
  mmdc -i "$f" -o "reference/$base.png" -b white
done
./compare.sh      # prints per-file err (sorted) + TOTAL_ERR
```
`reference/` and `output/` are regenerable artifacts (safe to delete; not source).

## 9. Gotchas learned the hard way

- `-- no -->` inline edge labels: must be matched as ONE operator carrying the
  label, else "no"/"yes" become phantom nodes. Handled in statement.go regexes
  (reSolidLabel/reThickLabel/reDotLabel) tried BEFORE bare connectors.
- `&` inside a quoted label (e.g. `WP2["Email & AI Routing"]`) must NOT split
  the node — `splitAmp` is bracket/quote-aware.
- Isolated subgraph members (edges only cross outside the subgraph) must stack
  PERPENDICULAR to the PARENT flow (vertical column for an LR parent). See
  `subgraphChildrenIsolated` + `perpendicular` in engine.go. This fixed 480
  (err 2.38 -> 0.04) and is why 503 subgraphs stack vertically.
- Arrowhead direction follows the spline's end tangent (edges.go strokeSpline
  returns the end tangent).

## 10. Suggested next steps for a new session

1. Attack I1 (630/700) with cross-axis component compaction; measure with
   ./compare.sh; keep only if TOTAL_ERR drops and no good file regresses.
2. Then I2 (880-01) with priority alignment.
3. Keep every change gated on `go build && go vet && go test && ./compare.sh`
   and a spot visual check of 430 (TD) and 503 (LR subgraphs).


---

# SEQUENCE DIAGRAMS (added 2026-10-12)

## S1. Packages (all NEW; flowchart packages were NOT modified)

- `seqmodel/seqmodel.go` — Diagram, Participant (ID/Label/Kind), Event union
  (EvMessage, EvNote, EvBlockStart, EvBlockElse, EvBlockEnd, EvActivate,
  EvDeactivate), ArrowStyle{Dotted,Head}, HeadType (Arrow/Open/Cross/None/Bi),
  NotePlacement (Over/LeftOf/RightOf). Autonumber bool. EnsureParticipant,
  Index(id).
- `seqparser/` — `seqparser.go` (header, autonumber, participant/actor `as`
  alias, Note over/left of/right of, blocks loop/alt/opt/par/critical/break/
  rect/box + else/and/option + end, activate/deactivate, %% comments);
  `message.go` (splitArrow longest-first for -> --> ->> -->> -x --x -) --)
  <<->> <<-->>, +/- activation suffix, self-messages, cleanText, color prefix).
  `seqparser_test.go` parses all 32 + a detailed example.
- `seqlayout/` — `types.go` (Config, Actor, Message, NoteBox, Block, Divider,
  Diagram), `engine.go` (Engine.Layout). Timeline layout: actor box widths from
  bold label; **label-proportional inter-column gaps** (gapBetween[i] = max over
  messages spanning boundary i of (labelWidth+24)/numBoundaries, floored at
  cfg.ActorGap); self-loops reserve selfExtra to the right; walk events top→
  bottom assigning Y; block frames via a stack (finalizeBlock spans all columns
  + depth inset); notes clamped to left margin; canvas size from max extents.
- `seqrender/` — `seqrender.go` (Theme, Renderer, Render: lifelines → blocks →
  notes → messages → actor boxes top+bottom mirrored), `draw.go` (messages:
  straight + self-loop arcs, arrowheads filled/open/cross/bi, dotted dashes,
  labels above line, autonumber purple circles with white number; notes yellow
  box; block frame + label tab + dashed else/and dividers).

## S2. Dispatch (main.go — the only shared file touched)

- `diagramKind(src)` → kindFlowchart | kindSequence | kindUnknown via
  `firstKeyword` (exact first word: "flowchart" or "sequenceDiagram").
- `renderFile` branches on kind; the flowchart branch is byte-for-byte the old
  path (flowchart metric stayed exactly 7.391 before/after). collectFiles now
  accepts both kinds. Non-supported diagrams (gitGraph, gantt) are skipped.
- Both types render into ./output/ in one `go run .` (49 files = 17 + 32).

## S3. Sequence feature coverage (verified on 32 testdata files)

autonumber, participant X as Label, Note over X[,Y], messages ->/-->/->>/-->>
with solid/dotted + arrow/open/cross/bi heads, self-messages (loop arc), alt/
else/end blocks. Parser also supports actor stick figures, loop/opt/par/
critical/break/rect/box + activate/deactivate (not present in testdata).

## S4. Fidelity metric + config (sequence)

Metric: `./compare-seq.sh` = Σ |ref_ar − mine_ar| over 32 files vs
`reference-seq/` (regenerate with mmdc, see S6). 10-iteration history:
- Baseline 28.376 (ALL 32 too TALL — mine ar < ref uniformly; mermaid sequence
  diagrams are ~2x wider than tall relative to mine).
- Iter 1–2: ActorGap 40→205, MsgGap 40→34 → 3.175 (swept; min at ~205).
- Iter 6–8: **label-proportional column gaps** (replaced flat gap) + retune
  ActorGap→218 → 2.659. Residual errors now SMALL and BIDIRECTIONAL (worst
  ~0.17), i.e. no systematic bias remains — just per-file label-width noise.
- Final: SEQ_TOTAL_ERR = **2.659** (91% below baseline).
- `seqlayout.DefaultConfig`: Margin=12, ActorW=75, ActorH=44, ActorPadX=16,
  ActorGap=218 (base/min inter-column gap), TopGap=38, MsgGap=34, SelfMsgGap=36,
  NoteGap=12, NotePadX=14, NotePadY=10, BlockPadTop=22, FontSize=14,
  LineHeight=17, LabelFont=13.

## S5. KNOWN ISSUES / OPEN WORK (sequence)

- SI1. ActorGap=218 base is large; it is a floor under the label-proportional
  gaps. Diagrams with very short labels get slightly over-wide; those with long
  labels slightly under-wide. Residual is small (~0.08/file avg). A fully
  label-driven gap with a smaller floor regressed the metric (short-label files
  collapsed), so the floor stays. Could revisit with per-boundary label maxima
  weighted by message count.
- SI2. Block frames span ALL participants; mermaid narrows an alt/loop frame to
  just the involved actors. Cosmetic; not yet implemented. See finalizeBlock.
- SI3. Notes over the leftmost actor are CLAMPED to the left margin (not shifted
  with a wider canvas as mermaid does). Shifting the whole diagram right instead
  regressed the metric 2.659→3.533 because that opening note recurs in many
  files, so clamping was chosen. Minor: a very wide leftmost note sits flush to
  the margin rather than centered on its actor.
- SI4. Activations (activate/deactivate and +/- suffixes) are parsed but NOT yet
  drawn as activation bars. No testdata uses them.
- SI5. Metric is the same aspect-ratio proxy caveat as the flowchart side.

## S6. Reproduce sequence references + compare

```bash
mkdir -p reference-seq
for f in testdata/*.mmd; do
  head -1 "$f" | grep -qE '^\s*sequenceDiagram' || continue
  base=$(basename "$f" .mmd)
  mmdc -i "$f" -o "reference-seq/$base.png" -b white
done
./compare-seq.sh      # per-file err (sorted) + SEQ_TOTAL_ERR
```
`reference-seq/` is a regenerable artifact (safe to delete).

## S7. Verification gate for ANY change

`go build ./... && go vet ./... && go test ./... && ./compare.sh && ./compare-seq.sh`
Flowchart metric MUST stay 7.391 (proves flowchart untouched); sequence metric
should stay ≤ 2.659 or improve. Spot-check 600-email-contact-flow-02 (alt block
+ note + self-messages) and 590-architecture-handling-08 (opening note).


---

# GANTT CHARTS (added 2026-10-12)

## G1. Packages (all NEW; flowchart + sequence packages NOT modified)

- `gmodel/gmodel.go` — Chart{Title, DateFormat, AxisFormat, TickEvery, TickUnit,
  Sections, Tasks, Min, Max}, Section{Name, Tasks}, Task{ID, Label, Tag
  (TagNormal/Active/Done/Crit), Milestone, Start, End, Section}. Task.SetRaw/
  RawStart/RawEnd stash unresolved spec strings for the resolver.
- `gparser/` — `gparser.go` (header `gantt`, title, dateFormat, axisFormat,
  tickInterval `<n><unit>`, section, task line `Label : [tags,] [id,]
  <start>, <end|dur>`; tags done/active/crit/milestone parsed first; excludes/
  weekend/todayMarker/click/vert ignored). `dates.go` resolveDates: goLayout
  converts dayjs dateFormat->Go reference layout; durations ms/s/m/h/d/w/M/y;
  `after <id...>` (latest end), `until <id>` (that task's start); sequential
  default start = previous task's End; sets Chart.Min/Max. `gparser_test.go`.
- `glayout/` — `glayout.go` (Config, Bar, SectionBand, Tick, Diagram, Engine).
  Linear date->x scale over [Min,Max] -> [ChartLeft,ChartRight]; one row per
  task (rowH = BarH+BarGap); section bands span full width (Index for color
  cycle); bars by Start/End, label inside if it fits else beside; milestones =
  diamond at midpoint(Start,End) with label beside; ticks via alignDown +
  advance per TickUnit; today marker X when now in [Min,Max]. `axis.go`
  formatAxis = strftime subset (%Y %y %m %d %e %H %M %S %b %B %a %A %%).
- `grender/` — section bands (cycled palette) -> gridlines -> section names ->
  bars/milestones -> axis labels -> red today line -> centered title. drawBar
  roundRect colored by tag; drawMilestone diamond.

## G2. Dispatch (main.go)

- `kind` enum gained `kindGantt`; `diagramKind` maps first word "gantt".
- `renderers` struct bundles all three engine+renderer pairs; `renderFile`
  switches on kind. Flowchart and sequence branches are byte-identical to before
  (both metrics unchanged: 7.391 and 2.659).
- `go run .` renders 51 files total (17 flowchart + 32 sequence + 2 gantt).

## G3. Gantt feature coverage (verified on 2 testdata files)

title, dateFormat (YYYY-MM-DD), axisFormat (%d.%m, %Y), tickInterval (1week,
1month), section, tasks with tags (done/active/crit/milestone incl. combos like
`milestone,done`), explicit start dates, durations (7d/10d/0d), implicit
sequential starts. Parser also supports `after`/`until` (not in testdata).
Not implemented: excludes, weekends, compact mode, vert markers (ignored).

## G4. Fidelity metric + config (gantt)

Metric: `./compare-gantt.sh` = sum |ref_ar - mine_ar| over 2 files vs
`reference-gantt/`. GANTT_TOTAL_ERR = 0.038 — the aspect ratio matched the
mermaid references almost exactly on the FIRST render (220-02 ref 2.154 /
mine 2.175; 220-01 ref 4.558 / mine 4.575). The 10 iterations therefore focused
on VISUAL fidelity, not proportions.
- `glayout.DefaultConfig`: Margin=8, TitleH=40, LeftPad=100, RightPad=120,
  BarH=20, BarGap=4, AxisH=34, TopPad=12, FontSize=12, SectionFont=13,
  MinChartW=560.
- `grender.DefaultTheme`: Bands = [#E6E6FA lavender, #F7F7FF near-white,
  #FCFCE3 light-yellow] cycled by section index; BarNormal #8A90DD, BarActive
  #6A6AD6, BarDone #BFC7D5 (grey), BarCrit #FF3737 (red), Grid #D0D0D0, Today
  red.

## G5. GOTCHA — Go color.RGBA is ALPHA-PREMULTIPLIED

The single biggest fix during iteration: section bands first used
`color.RGBA{128,128,192,40}` (semi-transparent). Go's `image/color.RGBA` is
**alpha-premultiplied**, so RGB values must be <= alpha; `{128,128,192,40}` is
an invalid/opaque DARK color and rendered as saturated purple, not a light
tint. Fix: use SOLID light RGB (alpha 255). Never use alpha < max with raw RGB
in this codebase — precompute the tint over white instead.

## G6. KNOWN ISSUES / OPEN WORK (gantt)

- GI1. Section band colors cycle a fixed 3-color palette by index; mermaid's
  `numberSectionStyles` config isn't read. Fine for testdata.
- GI2. `excludes` / weekends ignored, so tasks spanning excluded days are not
  extended as mermaid does. No testdata uses them.
- GI3. Milestone/outside labels use the regular font (gofont has no bundled
  italic); mermaid italicizes milestone labels. Cosmetic.
- GI4. Chart width fixed (MinChartW); mermaid sizes to content. Aspect ratio
  still matched because both axes scale together.
- GI5. `after`/`until` implemented but untested against a reference.

## G7. Reproduce gantt references + compare

```bash
mkdir -p reference-gantt
for f in testdata/*.mmd; do
  head -1 "$f" | grep -qE '^gantt' || continue
  base=$(basename "$f" .mmd)
  mmdc -i "$f" -o "reference-gantt/$base.png" -b white
done
./compare-gantt.sh
```

## G8. Verification gate (now covers all THREE types)

`go build ./... && go vet ./... && go test ./... && ./compare.sh &&
./compare-seq.sh && ./compare-gantt.sh`
Flowchart MUST stay 7.391, sequence <= 2.659, gantt <= 0.038. Spot-check
220-project-timeline-02 (3 sections, bars by tag, milestones, today line) and
220-project-timeline-01 (Legend: done/active/crit swatches).


---

# RELEASE / DISTRIBUTION / SECURITY (added 2026-10-13)

## 11. Module path + go install

- `go.mod` module path is now `github.com/megaproaktiv/gmaid` (was bare
  `gmaid`). ALL internal imports were rewritten from `"gmaid/<pkg>"` to
  `"github.com/megaproaktiv/gmaid/<pkg>"` (sed across all *.go). The readme and
  anonflow.go dev tool were updated too.
- Why: `go install <path>@<version>` requires the module path in go.mod to match
  the repo import path, or the proxy rejects it. The published v0.3.0 tag still
  had `module gmaid`, so installs failed.
- Install command (pinned, now in readme):
  `go install github.com/megaproaktiv/gmaid@v0.4.0`
- Go 1.27.2 minimum is intentional/documented as a SECURITY floor (older
  toolchains lack fixes gmaid depends on). Stated in readme + go.mod `go 1.27.2`.

## 12. Git state after 2026-10-13 session

- Branch `main`, remote `git@github.com:megaproaktiv/gmaid.git`.
- Commits pushed this session:
  - `4879fe7` rename module path + rewrite imports.
  - `c43b914` docs: readme install command + Go 1.27.2 security note.
  - `d53f0d0` ci: run govulncheck before builds.
  - (+ this memory update commit.)
- Tag `v0.4.0` was created at `4879fe7` and pushed. NOTE: the readme/govulncheck
  commits came AFTER the tag, so they are NOT in v0.4.0. If you want them in a
  release, cut a new tag (e.g. v0.4.1) rather than force-moving v0.4.0.
- **v0.5.0 (2026-10-13): first gitGraph release.** Because v0.4.0 was already
  published (module rename), the gitGraph feature shipped as the next minor
  v0.5.0. changelog.md gained 0.5.0 (gitGraph) and a back-filled 0.4.0 (module
  rename / govulncheck) entry. Released with GoReleaser (`goreleaser release
  --clean`) using the existing `.goreleaser.yaml` (which now runs go mod tidy,
  go test, govulncheck before building linux/darwin/windows x amd64/arm64).
- **v0.5.5 (2026-10-13): GMAID_SCALE env var.** main.go resolveScale(): explicit
  -scale flag wins (detected via flag.Visit), else GMAID_SCALE if a positive
  float, else default 2.0; invalid/non-positive env warns and falls back.
  Applies to all 4 renderers (shared *scale). Documented in readme "Output
  resolution (DPI)" section with the 1.5/2/3 -> ~330/450/675 DPI table. Released
  with GoReleaser as v0.5.5.

## 13. govulncheck wired into builds

- `Taskfile.yaml` (NOTE: actual filename is `Taskfile.yaml`, though readme/
  header comment call it `Taskfile.dev.yml`): new `vulncheck` task; `build` now
  has `deps: [vulncheck]` so it scans before compiling.
- `.goreleaser.yaml`: added to `before.hooks` after `go test ./...`.
- Both invoke `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` (no global
  install needed, CI-friendly). Last run: "No vulnerabilities found."

## 14. Sophos CryptoGuard "ransomware" alert — FALSE POSITIVE (resolved)

- 2026-10-13 ~01:21 Sophos flagged
  `~/Library/Caches/go-build/9f/9f20786...-d/gmaid` as ransomware.
- That path is the Go BUILD CACHE; the file is a normal Mach-O arm64 executable
  whose content-hash matches its cache filename (untampered, a legit `gmaid`
  build from 00:54).
- Source audit found ZERO malicious capabilities: no net/http/net, no
  os/exec/exec.Command/syscall.Exec, no crypto/aes|cipher, no
  os.Remove/RemoveAll. Only 3 deps (fogleman/gg, golang/freetype,
  golang.org/x/image). The ONLY os.WriteFile in the tree is in anonflow.go,
  which is `//go:build ignore` and NOT in the binary. govulncheck clean.
- Root cause: behavioral heuristic — a fresh unsigned native binary in a cache
  dir that rapidly reads many .mmd and writes many .png (batch render of 51
  diagrams) pattern-matches ransomware encrypting a folder. It only ever reads
  inputs and writes NEW png files; never modifies/deletes originals.
- Fix/recommendation: add Sophos exclusion for `~/Library/Caches/go-build/`
  (and optionally `~/go/bin`). Optionally codesign+notarize release binaries via
  goreleaser to reduce heuristic flags on distributed artifacts (NOT yet
  configured). `go clean -cache` safely clears the flagged cache file.


---

# GITGRAPH DIAGRAMS (added 2026-10-13)

## GG1. Packages (all NEW; flowchart/sequence/gantt packages NOT modified)

- `gitmodel/gitmodel.go` — Graph{Branches, Commits, MainBranch; New(mainName),
  AddBranch, Branch, HasBranch}. Branch{Name, Index, Order} (Index/Order =
  creation order, drives lane + color). Commit{ID, AutoID, Tag, Kind, Branch,
  Seq, Parent, MergeFrom}. CommitKind = Normal/Merge/Reverse/Highlight/
  CherryPick. Parent and MergeFrom are indices into Commits (-1 = none). AutoID
  marks ids that mermaid does NOT draw (merge + bare-commit auto ids).
- `gitparser/gitparser.go` — Parse(src). Header `gitGraph` (tolerates
  `gitGraph:` / `gitGraph LR:` via HasPrefix). Directives: commit [id:/tag:/
  type:], branch NAME, checkout|switch NAME, merge NAME [tag:/type:],
  cherry-pick id:. Default branch "main". current branch tracked; branchTip map
  gives each commit its Parent; merge commits get MergeFrom = source branch tip
  and Kind=CommitMerge; auto ids are sequential ints with AutoID=true.
  parseCommitOpts/extractOpt read quoted ("...") or bare (type:) values.
  `gitparser_test.go`: 3 tests (all testdata parse sane; exact git-100
  structure; option extraction) — all pass.
- `gitlayout/gitlayout.go` — horizontal lanes, one row per branch by Branch.
  Order; laneY(order)=Margin+TopPad+order*LaneGap; commitX(seq)=Margin+labelW+
  FirstCommit+seq*CommitSep (x advances by GLOBAL seq, not per-branch).
  LaneLabel = colored box on left (width = max branch-name width, floored at
  LabelBoxW). CommitDot carries ShowID=!AutoID. Connectors: each commit->parent
  and merge->source; same-lane = straight, cross-lane = rounded elbow. Canvas:
  W=commitX(maxSeq)+CommitSep+Margin; H=laneY(nBranch-1)+BottomPad+Margin.
- `gitrender/gitrender.go` — gg rasterizer. Draw order: connectors (thick
  Radius*0.9 rounded) -> commit dots -> tags -> rotated commit ids -> branch
  label boxes. Dots: normal=filled circle, merge=ringed (color/white/color-core),
  highlight=square, reverse=crossed. Tag = light-yellow rounded box + bullet +
  connector line above the commit. Commit ids drawn rotated 45deg BELOW the dot,
  and ONLY when ShowID (auto/merge ids suppressed, matching mermaid).

## GG2. Dispatch (main.go — only shared file touched)

- `kind` enum gained `kindGit`; `diagramKind` maps first word "gitGraph".
- `renderers` struct gained gitEng/gitRnd; `renderTo` has a `case kindGit`.
- Flowchart/sequence/gantt branches are byte-identical to before. collectFiles
  already includes any kind != kindUnknown, so gitGraph files are auto-picked.
- `go run .` now renders 53 files (17 flowchart + 32 sequence + 2 gantt + 2 git).

## GG3. gitGraph feature coverage (verified on 2 testdata files)

commit (bare/id/tag/type), branch, checkout/switch, merge (+tag), default main
branch. Parser ALSO supports cherry-pick, type: REVERSE/HIGHLIGHT, switch alias
(not in testdata). Orientation is always the mermaid default (horizontal lanes).

## GG4. Fidelity metric + config (gitGraph)

Metric: `./compare-git.sh` vs `reference-git/` (regenerate with mmdc, see GG6).
This script is MORE ACCURATE than the ar-only harnesses (per the user's request):
it is self-contained (renders ONLY gitGraph via `gmaid -i`, mmdc-style) and
reports TWO numbers — aspect-ratio displacement AND scale-normalized size error.
gmaid renders at -scale 2 by default; the script de-scales mine by SCALE
(default 2, override `SCALE=3 ./compare-git.sh`) so size is logical-pixel.
- Result: **GIT_AR_ERR=0.032** (0.016/file) and **GIT_SIZE_ERR=0.010**
  (de-scaled mine≈625x397 vs ref 625x393 — width exact, +4px height).
  0.016/file ar beats the per-file averages of flowchart (~0.43), seq (~0.08)
  and matches gantt (~0.019). Not chasing the last 4px: it is the descender
  safety margin for the rotated "object expedition" commit id.
- `gitlayout.DefaultConfig`: Margin=10, LabelBoxW=118, LabelBoxH=28, LabelPadX=12,
  LaneGap=87, CommitSep=49, FirstCommit=36, TopPad=34, BottomPad=82, Radius=9,
  FontSize=13, LabelFont=15, TagFont=12.
- `gitrender` branch palette (cycled by creation index, sampled from mmdc PNG):
  #0000E0 blue(main), #D0D000 yellow, #90E000 green, #0070E0 bright-blue, then
  orange/purple/teal/pink for >4 branches. Tag box #FFFFCC, id text #333333.

## GG5. KNOWN ISSUES / OPEN WORK (gitGraph)

- GGI1. Merge/branch connector elbows turn near the CHILD column; mermaid's
  risers sometimes sit at the source column. Visually equivalent on testdata;
  not pixel-matched.
- GGI2. Only the default horizontal orientation is drawn; `gitGraph TB:`
  (vertical) is parsed-tolerant but laid out horizontally.
- GGI3. cherry-pick / REVERSE / HIGHLIGHT render paths exist but are untested
  against a reference (no testdata exercises them).
- GGI4. Same aspect-ratio-proxy caveat as the other types, mitigated here by the
  added size metric.

## GG6. Reproduce gitGraph references + compare

```bash
mkdir -p reference-git
for f in testdata/*.mmd; do
  head -1 "$f" | grep -qE '^\s*gitGraph' || continue
  base=$(basename "$f" .mmd)
  mmdc -i "$f" -o "reference-git/$base.png" -b white
done
./compare-git.sh          # per-file ar + size err, GIT_AR_ERR + GIT_SIZE_ERR
```
`reference-git/` is a regenerable artifact (safe to delete).

## GG7. PRE-EXISTING METRIC DRIFT (important for a fresh session)

As of 2026-10-13 the flowchart metric reads 7.801 (memory records 7.391) and
sequence reads 21.474 (memory records 2.659). This drift is NOT from the
gitGraph work: it was PROVEN pre-existing by stashing all gitGraph changes and
re-running on clean HEAD — identical 7.801 / 21.474. The gitGraph change set
touched only main.go dispatch + the four new git* packages; zero flowchart/
sequence/gantt source changed (git diff --stat confirms). The drift comes from
`reference*/` PNGs having been regenerated with a different mmdc version than
when 7.391/2.659 were recorded. Do NOT attribute it to git support, and do NOT
"fix" seq/flowchart layout to chase it without first re-establishing the
reference baseline.


---

# GANTT EXTENSIONS + CSS (added 2026-10-26)

Added WITHOUT touching flowchart/sequence/git packages. Gantt metric stayed
0.038 throughout (verified with ./compare-gantt.sh). Only gmodel/ gparser/
glayout/ grender/ main.go changed, plus a NEW gcss/ package.

## GX1. CSS styling of gantt bars (`gmaid -C style.css`)

- NEW package `gcss/` (gcss.go + color.go + gcss_test.go). Parses a tiny CSS
  subset and, given a task id, returns the winning fill/stroke.
- Mermaid renders each task bar as `<rect id="my-svg-<taskid>">`. gcss emulates
  that id ("my-svg-" + taskID) and matches ATTRIBUTE SELECTORS:
  `[id="x"]` `[id^="x"]` `[id$="x"]` `[id*="x"]` (leading `rect`/`id` optional).
  Declarations supported: `fill:` and `stroke:` (others ignored). `!important`
  honored (outranks non-important); later source order wins on ties.
- COMMA-SEPARATED SELECTOR LISTS are supported (e.g.
  `rect[id*="-pm"], rect[id*="-lead_"] { fill:... }`). This was a bug the first
  cut missed — each comma part becomes its own rule sharing the body. Without
  it the aggregated "Project lead" row (ids lead_11..) stayed default-colored.
- Wiring: main.go `-C` flag (mmdc-compatible) loads the sheet and calls
  `grender.Renderer.SetStyle(*gcss.Sheet)`. drawBar/drawMilestone/drawSummary
  override fill/stroke from the sheet by `b.Task.ID`.
- Color parsing: #rgb/#rrggbb/#rrggbbaa (alpha PREMULTIPLIED per G5) + a few
  named colors. Validated against the real conrad plan.css (wp4_ai->#d62728,
  engc_->#2ca02c, lead_->#1f77b4, etc.).
- Also added `-c` flag: accepted + IGNORED (mmdc config file compatibility).

## GX2. `<br>` line breaks + p-with-overline (p̄) in gantt

- `gparser.normalizeBreaks()` converts `<br>`,`<br/>`,`<br />`,`\n` to real
  newlines; applied to BOTH section names and task labels. Section titles can
  now be multi-line (e.g. `section WP1<br/>Email channel<br/>80 PD`).
- grender draws multi-line section names AND bar labels (split via
  textmetrics.SplitLines, line height = fontSize+3, vertically centered).
- COMBINING MACRON (U+0304) / OVERLINE (U+0305) have NO glyph in the Go builtin
  font (truetype Index==0 -> tofu box). FIX: `grender.drawLabel()` strips the
  combining mark and draws a short horizontal bar itself over the preceding
  base glyph (measured via face.Measure of the substring). So "0,7p̄" renders
  as p with a drawn overline. ALL gantt label drawing goes through drawLabel.

## GX3. Dependency arrows (FS/SS/FF/SF) — gmaid extension

- Mermaid gantt has NO arrow syntax (open feature request mermaid#7300). gmaid
  adds a `link` directive:
    `link <fromId> <toId>`          (implicit FS)
    `link <fromId> FS|SS|FF|SF <toId>`
    `link <fromId> --> <toId>`      (--> treated as FS)
  German aliases accepted: EA/AA/EE/AE and Ende-Anfang etc.
- model: gmodel.Link{From,To,Kind}, Chart.Links. parser: parseLink/linkKind.
- layout: glayout.LinkPath{Pts []{X,Y}} computed in Layout via a taskID->Bar
  map; `linkPath()` picks source/target edges by kind (FS end->start, SS
  start->start, FF end->end, SF start->end) and routes an orthogonal polyline
  (stub out of source, vertical run, into target edge).
- render: grender.drawLink draws the polyline (collapsing duplicate points) +
  a filled arrowhead on the final segment. Theme.Link color (#333).

## GX4. Summary / Sammelvorgang tasks (MS-Project rollup bar) — extension

- Tag `summary` on a task: `Phase 1 :summary, p1, 2026-01-01, 2026-03-31`.
- model: Task.Summary. render: grender.drawSummary = slim solid spine bar +
  DOWNWARD-POINTING filled triangles at both ends (the MS-Project bracket
  look). Honors CSS fill override. Label drawn beside it (outside).

## GX5. Same-line tasks (share one row) — extension

- Prefix a task label with `&` OR add the `sameline` tag to render it on the
  SAME row as the previous task instead of advancing a row. Lets e.g. a role's
  weekly/monthly sums sit on ONE line:
    `W1 3 :w1, ...`
    `&W2 9 :w2, ...`   (& => same row as w1)
- parser: strips leading `&`, sets Task.SameLine (also `sameline` tag).
- layout: when SameLine && not first in section, reuse previous row Y and do
  NOT advance rowIdx (so the section band height accounts only for real rows).

## GX6. TEXT ALWAYS ON TOP (render order)

- grender.Render now draws in strict layers: bands -> gridlines -> section
  names -> BAR SHAPES (drawBar/drawSummary/drawMilestone draw shapes only) ->
  dependency arrows -> BAR LABELS (new drawBarLabel pass) -> axis -> today ->
  title. Labels were split out of the shape functions so no later bar or arrow
  can overdraw text. drawBarLabel handles normal/milestone/summary label cases.

## GX7. testdata examples (stored at user request, 2026-10-26)

In ./testdata (both .mmd and rendered .png committed as feature references):
- gantt-200-css-colors.mmd (+ .css)  — CSS id-selector coloring
- gantt-210-dependencies.mmd          — FS/SS dependency arrows
- gantt-220-summary.mmd               — summary rollup bars + milestone
- gantt-230-br-macron.mmd             — <br/> multi-line + p̄ overline
- gantt-240-sameline.mmd              — & same-line weekly sums
NOTE: this intentionally overrides the older "never write PNGs into testdata"
rule (G/§2) for these feature-reference images, per explicit user instruction.

## GX8. Verification gate (unchanged commands)

`go build ./... && go vet ./... && go test ./... && ./compare.sh &&
./compare-seq.sh && ./compare-gantt.sh`
New unit tests: gcss/gcss_test.go (selectors, !important, source order) and
gparser/gparser_ext_test.go (link/summary/sameline/<br>). Gantt metric 0.038.
NOTE: the current testdata flowchart/seq FILESET differs from the older memory
baselines (files renamed to flowchart-NNN / the seq set changed), so
TOTAL_ERR/SEQ_TOTAL_ERR now read differently than the historical 7.391/2.659 —
this is a fileset change, NOT a regression (flowchart/seq/git CODE untouched;
confirmed via `git diff --stat`: only gmodel/gparser/glayout/grender/main.go +
new gcss/).
