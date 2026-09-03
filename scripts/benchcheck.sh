#!/usr/bin/env bash
#
# Compare a benchmark run against the committed baseline with benchstat and
# fail when any benchmark regressed past the tolerance.
#
# Usage: bash scripts/benchcheck.sh <benchmark-output.txt>
#
# Rows benchstat marks "~" (p >= 0.05, statistically insignificant) are ignored,
# so ordinary runner noise does not fail the check. Refresh the baseline with
# `task benchmark:baseline`.

set -euo pipefail

BASELINE=tests/benchmarks/baseline.txt
BENCHSTAT=golang.org/x/perf/cmd/benchstat@v0.0.0-20260825160852-19be9d8e6c70

# Maximum allowed slowdown vs baseline, in percent.
TIME_TOLERANCE_PCT=15  # sec/op: shared CI runners are noisy
ALLOCS_TOLERANCE_PCT=5 # allocs/op: near-deterministic, keep it tight

new=${1:?usage: $0 <benchmark-output.txt>}

# -row .name drops the GOMAXPROCS suffix so a baseline from a 4-core runner
# still lines up with a run on a different core count.
# ponytail: it also pools sub-benchmarks (ProviderTransformations/*) into one
# row; switch to .fullname if per-sub-benchmark granularity is ever needed.
go run "$BENCHSTAT" -row .name "$BASELINE" "$new"

go run "$BENCHSTAT" -row .name -format csv "$BASELINE" "$new" 2>/dev/null |
	awk -F, -v t_time="$TIME_TOLERANCE_PCT" -v t_allocs="$ALLOCS_TOLERANCE_PCT" '
    # Unit header row of each table: ",sec/op,CI,sec/op,CI,vs base,P".
    $3 == "CI" {
      unit = $2
      limit = (unit == "sec/op") ? t_time : (unit == "allocs/op") ? t_allocs : 0
      next
    }
    # Data row: name,base,CI,new,CI,delta,P. Only positive, significant deltas matter.
    limit && $1 != "geomean" && $6 ~ /^\+/ {
      pct = substr($6, 2) + 0
      if (pct > limit) {
        printf "REGRESSION: %s %s %s vs baseline (limit +%d%%)\n", $1, unit, $6, limit
        failed = 1
      }
    }
    END {
      if (failed) exit 1
      print "benchcheck: no regression beyond +" t_time "% sec/op / +" t_allocs "% allocs/op"
    }
  '
