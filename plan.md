1. The user wants me to benchmark the ebuild parsing.
2. We have `ebuild_bench_test.go`. I should run the benchmark to see how it performs with and without `EnableWeakEbuildContent`.
3. To benchmark it, I can add a benchmark in `ebuild_bench_test.go` or just run `go test -bench=BenchmarkParseEbuild ./cmd/g2/...` no wait, `ParseEbuild` is in the root `g2` package.
So `go test -bench=BenchmarkParseEbuild .`
4. Let's see what benchmarks exist in `ebuild_bench_test.go`.
