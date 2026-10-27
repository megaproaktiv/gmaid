# gmaid

A **pure-Go** renderer for Mermaid diagrams — no cgo, no headless browser. It
parses `.mmd` files and rasterizes them straight to PNG, and works as a drop-in
replacement for `mmdc`:

```sh
gmaid -i input.mmd -o output.png     # mmdc-compatible single-file mode
gmaid                                 # batch: render all supported .mmd in ./testdata -> ./output
```

## Install

```sh
go install github.com/megaproaktiv/gmaid@v0.4.0
```

> **Requires Go 1.27.2 or newer.** This minimum is enforced for security
> reasons — older toolchains are missing fixes that `gmaid` depends on. Check
> your version with `go version`.

## Examples

The `examples/` directory holds one ready-to-render diagram of each supported
type, together with its rendered PNG. Each lives in its own subdirectory so you
can see the source `.mmd` next to the output:

| Diagram           | Source                               | Rendered output                      |
|-------------------|--------------------------------------|--------------------------------------|
| Flowchart         | `examples/flowchart/flowchart-100.mmd` | `examples/flowchart/flowchart-100.png` |
| Sequence diagram  | `examples/sequence/sequence-100.mmd`   | `examples/sequence/sequence-100.png`   |
| Git graph         | `examples/gitgraph/git-100.mmd`        | `examples/gitgraph/git-100.png`        |
| Gantt chart       | `examples/gantt/quest-ring.mmd`        | `examples/gantt/quest-ring.png`        |

Render any of them yourself with the single-file mode:

```sh
gmaid -i examples/flowchart/flowchart-100.mmd -o examples/flowchart/flowchart-100.png
gmaid -i examples/gantt/quest-ring.mmd        -o examples/gantt/quest-ring.png
```

The flowchart, sequence and git-graph examples are copied straight from the
anonymised `testdata/` set. The Gantt example (`examples/gantt/quest-ring.mmd`)
is an **enhanced, hand-built** chart that exercises three advanced features at
once, themed after the quest to destroy the One Ring:

- **Summary tasks (Sammelvorgänge):** the `summary` tag renders a rollup bar
  spanning a whole phase, e.g. the "Fellowship of the Ring" bar over the
  Fellowship section.
- **Same-line tasks (Vorgänge in einer Linie):** prefixing a task label with
  `&` packs it onto the same row as the preceding task. The "Carry the Ring
  (one line)" section places four legs (Amon Hen, Emyn Muil, Marshes, Black
  Gate) on a single row.
- **Dependencies drawn as vectors:** `link <from> <KIND> <to>` draws an arrow
  between tasks, where `KIND` is `FS`/`SS`/`FF`/`SF` (German aliases
  `EA`/`AA`/`EE`/`AE` also work). The example chains the whole quest with
  finish-to-start, start-to-start and finish-to-finish arrows.

All names are anonymised with *The Lord of the Rings* motifs (the Ring, Mordor,
Mount Doom, the fall of Barad-dur), matching the anonymisation used elsewhere in
the project.

Supported diagram types (dispatched by the first keyword):

| Keyword           | Packages                                      |
|-------------------|-----------------------------------------------|
| `flowchart`       | `model/ parser/ layout/ render/`              |
| `sequenceDiagram` | `seqmodel/ seqparser/ seqlayout/ seqrender/`  |
| `gantt`           | `gmodel/ gparser/ glayout/ grender/`          |
| `gitGraph`        | `gitmodel/ gitparser/ gitlayout/ gitrender/`  |

## Output resolution (DPI)

`gmaid` rasterizes at a configurable scale factor. The layout is computed in
logical pixels and multiplied by the scale, so a higher scale yields a larger,
sharper PNG at the same proportions.

Set the scale in either of two ways:

```sh
gmaid -scale 1.5 -i input.mmd         # command-line flag
GMAID_SCALE=1.5 gmaid -i input.mmd    # environment variable
```

Precedence: an explicit `-scale` flag always wins; otherwise `GMAID_SCALE` is
used if set; otherwise the default of `2`. An invalid or non-positive
`GMAID_SCALE` is ignored (with a warning) and the default applies.

`GMAID_SCALE` is the practical lever for hitting a target print DPI without
changing how you invoke the tool (handy in CI or a `Taskfile`). Rough guidance
for a typical diagram:

| `GMAID_SCALE` | Approx. DPI | Notes                                             |
|---------------|-------------|---------------------------------------------------|
| `1.5`         | ~330 DPI    | Comfortably above 300 DPI with a safety margin    |
| `2` (default) | ~450 DPI    | Conservative; ~55% fewer pixels than scale `3`    |
| `3`           | ~675 DPI    | Largest/sharpest; biggest files                   |

Exact pixel counts depend on the diagram's logical size; use the table as a
starting point and check the resulting PNG dimensions for your content.

## Why: speed

`mmdc` (the official Mermaid CLI) drives a headless Chromium through Puppeteer,
so every invocation pays Node + browser startup and SVG→PNG conversion. `gmaid`
is a single compiled binary that parses, lays out and rasterizes natively, so it
is dramatically faster — especially for the common "render a folder of diagrams"
workflow.

## Benchmark: gmaid vs mmdc

Measured on this project's `testdata/` (macOS, warm caches, `/usr/bin/time -p`,
best of 3 runs). `gmaid` is the compiled binary (`go build -o gmaid .`); both
tools invoked once per file via `-i` for an apples-to-apples comparison.

### Single file (one diagram, cold per-call startup)
  
  | Tool  | Time per render | Relative |
  |-------|-----------------|----------|
  | mmdc  | ~0.69 s         | 1×       |
  | gmaid | ~0.037 s        | **~18.9× faster** |
  
  ### Batch of 10 files (per-file `-i` invocation loop)
  
  | Tool  | Total (10 files) | Per file | Relative |
  |-------|------------------|----------|----------|
  | mmdc  | 7.36 s           | ~736 ms  | 1×       |
  | gmaid | 0.49 s           | ~49 ms   | **~15.0× faster** |

### gmaid batch mode (all 51 diagrams in one process)

Running `gmaid` once over the whole `testdata/` set (17 flowcharts +
32 sequence + 2 gantt = 51 diagrams) completes in **~3.15 s total** (~62 ms per
diagram amortized). The equivalent per-file `mmdc` loop over 51 files
extrapolates to roughly **35–38 s**.

**Headline: gmaid renders Mermaid diagrams roughly 15–19× faster than mmdc**,
with the gap widening in batch mode because gmaid amortizes startup across all
files in a single process while mmdc restarts Node + Chromium for every file.

> Numbers are environment-dependent (CPU, disk, Node/Chromium version). Re-run
> the measurement with `task -t Taskfile.dev.yml render` for the gmaid side and a
> per-file `mmdc -i … -o …` loop for the mmdc side.

## Build & dev tasks

```sh
go build -o gmaid .
task -t Taskfile.dev.yml build      # build the binary
task -t Taskfile.dev.yml render     # render every testdata/*.mmd via `gmaid -i`
```

## Test data note

The `testdata/` diagrams are anonymised:

- **Sequence diagrams** (`sequence-NNN.mmd`): actor/participant names use
  *The Hobbit* cast (see `testdata/actors.txt`); message/note text is replaced
  with random words from the opening of *The Hobbit* (see `testdata/actions.txt`).
- **Flowcharts** (`flowchart-NNN.mmd`): node labels, edge labels and subgraph
  titles are replaced with length-matched words from the same vocabulary, node
  IDs are anonymised, and `click` directives (URLs/tooltips) are stripped
  entirely. The `anonflow.go` tool (run with `go run anonflow.go <vocab> <files>`)
  performs this using the project's own flowchart parser to locate labels.

The anonymised text is intentionally nonsensical; word lengths are roughly
preserved so layouts stay representative.
