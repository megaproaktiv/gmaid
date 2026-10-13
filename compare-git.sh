#!/bin/bash
# compare-git.sh — gitGraph fidelity metric vs mermaid references
# (reference-git/). Unlike the aspect-ratio-only harnesses, this reports BOTH
# the aspect-ratio displacement and the relative size (width+height) error so
# the number reflects true proportions, not just the w/h ratio.
#
# It renders ONLY the gitGraph testdata (via `gmaid -i`, mmdc-style, isolated
# per file) so the metric is self-contained and does not depend on rendering
# the other diagram types.
cd "$(dirname "$0")"

# gmaid renders at -scale (default 2.0); mmdc references are scale 1. Normalise
# mine by SCALE so the size metric measures true proportions, not the DPI
# multiplier. Override with: SCALE=3 ./compare-git.sh (match your render scale).
SCALE="${SCALE:-2}"

go build -o /tmp/gmaid-git . 2>/dev/null || { echo "build failed"; exit 1; }

ar_total=0
sz_total=0
rows=""
for f in testdata/*.mmd; do
  head -1 "$f" | grep -qE '^[[:space:]]*gitGraph' || continue
  base=$(basename "$f" .mmd)
  ref="reference-git/$base.png"
  mine="output/$base.png"
  [ -f "$ref" ] || continue
  /tmp/gmaid-git -i "$f" -o "$mine" >/dev/null 2>&1 || continue

  read rw rh < <(sips -g pixelWidth -g pixelHeight "$ref"  2>/dev/null | awk '/pixelWidth/{w=$2}/pixelHeight/{h=$2}END{print w, h}')
  read mw mh < <(sips -g pixelWidth -g pixelHeight "$mine" 2>/dev/null | awk '/pixelWidth/{w=$2}/pixelHeight/{h=$2}END{print w, h}')
  # de-scale mine to logical pixels
  mw=$(awk "BEGIN{printf \"%.1f\", $mw/$SCALE}")
  mh=$(awk "BEGIN{printf \"%.1f\", $mh/$SCALE}")

  ar=$(awk "BEGIN{r=$rw/$rh; m=$mw/$mh; d=r-m; if(d<0)d=-d; printf \"%.3f\", d}")
  # relative size error: mean of |dw|/rw and |dh|/rh
  sz=$(awk "BEGIN{dw=($mw-$rw)/$rw; if(dw<0)dw=-dw; dh=($mh-$rh)/$rh; if(dh<0)dh=-dh; printf \"%.3f\", (dw+dh)/2}")

  ar_total=$(awk "BEGIN{printf \"%.3f\", $ar_total+$ar}")
  sz_total=$(awk "BEGIN{printf \"%.3f\", $sz_total+$sz}")
  rows="$rows$ar ar | $sz sz | $base  ref=${rw}x${rh} mine≈${mw}x${mh}\n"
done
echo -e "$rows" | grep . | sort -rn
echo "GIT_AR_ERR=$ar_total   GIT_SIZE_ERR=$sz_total  (scale=$SCALE)"
