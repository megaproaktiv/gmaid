#!/bin/bash
# compare-gantt.sh — render all gantt charts and report aspect-ratio
# displacement vs mermaid references (reference-gantt/).
cd "$(dirname "$0")"
go run . >/dev/null 2>&1
total=0
rows=""
for p in reference-gantt/*.png; do
  base=$(basename "$p" .png)
  mine="output/$base.png"
  [ -f "$mine" ] || continue
  rd=$(sips -g pixelWidth -g pixelHeight "$p" 2>/dev/null | awk '/pixelWidth/{w=$2}/pixelHeight/{h=$2}END{printf "%.3f", w/h}')
  md=$(sips -g pixelWidth -g pixelHeight "$mine" 2>/dev/null | awk '/pixelWidth/{w=$2}/pixelHeight/{h=$2}END{printf "%.3f", w/h}')
  err=$(awk "BEGIN{d=($rd)-($md); if(d<0)d=-d; printf \"%.3f\", d}")
  total=$(awk "BEGIN{printf \"%.3f\", $total+$err}")
  rows="$rows$err $base ref=$rd mine=$md\n"
done
echo -e "$rows" | grep . | sort -rn
echo "GANTT_TOTAL_ERR=$total"
