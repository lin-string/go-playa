#!/bin/sh
set -eu

if [ "$#" -eq 0 ]; then
	set -- testdata/files/*.pdf
fi

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
binary=${TMPDIR:-/tmp}/go-playa-bench
repeats=${BENCH_REPEATS:-3}
task=${BENCH_TASK:-text-glyph-jsonl}
workers=${BENCH_WORKERS:-1}

go build -o "$binary" ./bench/go
printf '%s\n' "benchmark, repeats=$repeats files=$#" >&2

run_go() {
	printf '%s\n' "go[$i] task=$task workers=$workers pdf=$path" >&2
	"$binary" -task "$task" -workers "$workers" -jsonl "$path"
}

run_playa() {
	printf '%s\n' "playa[$i] task=$task workers=$workers pdf=$path" >&2
	uv run --project "$root/compat" python "$root/bench/run_playa.py" --task "$task" --workers "$workers" --jsonl "$path"
}

i=1
while [ "$i" -le "$repeats" ]; do
	for path in "$@"; do
		if [ "$((i % 2))" -eq 1 ]; then
			run_go
			run_playa
		else
			run_playa
			run_go
		fi
	done
	i=$((i + 1))
done
