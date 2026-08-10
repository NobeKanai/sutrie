# sutrie

`sutrie` is an immutable, memory-efficient byte trie for read-heavy workloads. It stores the tree topology as succinct bitsets and keeps child edges in sorted order, which makes prefix traversal predictable without allocating per-node maps.

The package is a good fit for large, mostly-static dictionaries such as reversed domain names, routing prefixes, protocol keywords, and content filters. It is intentionally a lookup structure: there are no insertion or deletion operations after construction.

## Requirements

- Go 1.26.5 or newer

Install the module with:

```bash
go get github.com/nobekanai/sutrie@latest
```

## Build a trie

```go
package main

import (
	"fmt"

	"github.com/nobekanai/sutrie"
)

func main() {
	keys := []string{"hat", "is", "it", "a"}
	root := sutrie.BuildSuccinctTrie(keys).Root()

	fmt.Println(root.Search("hat").Leaf()) // true
	fmt.Println(root.Search("ha").Leaf())  // false
	fmt.Println(root.SearchPrefix("hatt")) // 3
}
```

`Search` returns a node for the complete byte string. Check both `Exists` and `Leaf` when a lookup may be absent or may end at an internal prefix. `SearchPrefix` returns the length of the longest stored entry that is a prefix of the key, or zero when no stored entry matches. The returned node is a value handle into the immutable trie and is safe to copy for concurrent read-only use.

## Construction contracts

`BuildSuccinctTrie(dict)` sorts `dict` in place using `sort.Strings` and then builds the trie. This is the convenient entry point when the input order is not important:

```go
input := []string{"z", "a", "m"}
trie := sutrie.BuildSuccinctTrie(input)
// input is now []string{"a", "m", "z"}.
_ = trie
```

`BuildSuccinctTrieSorted(dict)` is the lower-overhead entry point for callers that already maintain sorted input. The slice must be in ascending lexicographic order according to `sort.Strings`; the function does not validate or reorder it. Passing unsorted input violates the function contract and can produce an invalid trie.

Both builders ignore empty strings and collapse duplicate entries. `SuccinctTrie.Size` reports the number of unique, non-empty entries. The trie operates on bytes, so strings are traversed as their UTF-8 byte representation rather than as Unicode code points.

## Traversal

```go
root := sutrie.BuildSuccinctTrie([]string{"car", "cat", "dog"}).Root()

fmt.Println(root.Children())              // "cd"
fmt.Println(root.Search("car").Leaf())   // true
fmt.Println(root.Search("ca").Exists())  // true, but it is not a leaf
fmt.Println(root.Search("missing").Exists()) // false
```

`Children` returns the sorted edge bytes for the current node. `Next` follows one edge and may return an invalid node; call `Exists` before traversing a result supplied by an optional path.

## Reversed domain matching

Reversing domain names turns suffix matching into a normal trie lookup. The following pattern stores reversed rules and walks labels from right to left:

```go
func reverse(s string) string {
	b := []byte(s)
	for left, right := 0, len(b)-1; left < right; left, right = left+1, right-1 {
		b[left], b[right] = b[right], b[left]
	}
	return string(b)
}

rules := []string{
	reverse("*.example.com"),
	reverse("google.*"),
}
trie := sutrie.BuildSuccinctTrie(rules)
```

Applications can then traverse `trie.Root()` one reversed label at a time and apply their wildcard policy. Keeping the policy outside `sutrie` makes the core structure useful for non-domain keys as well.

## Serialization

`Marshal(io.Writer)` and `Unmarshal(io.Reader)` use Go's `encoding/gob` format:

```go
var buf bytes.Buffer
if err := trie.Marshal(&buf); err != nil {
	panic(err)
}

var restored sutrie.SuccinctTrie
if err := restored.Unmarshal(&buf); err != nil {
	panic(err)
}
```

Treat serialized data as an application-level artifact. Validate its source and handle `Unmarshal` errors before serving lookups.

## Benchmarks

Run the deterministic unit tests and benchmarks with a temporary writable build cache when the default cache is unavailable:

```bash
go test ./...
go test -run '^$' -bench . -benchmem -count=5
```

Benchmarks read the first 100,000 non-comment entries from `domains.txt`, reverse each domain, and use the same corpus for every implementation. The comparison trie is [`github.com/derekparker/trie`](https://github.com/derekparker/trie) at `v0.0.0-20230829180723-39f4de51ef7d`; the map row is a hash-table baseline, not a prefix data structure. Setup and fixture parsing are excluded from timed lookup sections.

Reference output from Go 1.26.5 on an AMD Ryzen 3 5425U (Linux, `-benchtime=100ms -count=1`):

| Operation | Implementation | Time | Memory | Allocs |
| --- | --- | ---: | ---: | ---: |
| Build, unsorted input | sutrie | 51.0 ms/op | 7.9 MB/op | 77/op |
| Build, sorted input | sutrie | 19.9 ms/op | 6.3 MB/op | 76/op |
| Build | derekparker/trie | 513.5 ms/op | 211.0 MB/op | 2,231,442/op |
| Exact lookup, mixed hits/misses | sutrie | 514.1 ns/op | 0 B/op | 0/op |
| Exact lookup, mixed hits/misses | derekparker/trie | 598.3 ns/op | 49 B/op | 0/op |
| Exact lookup, mixed hits/misses | map[string]struct{} | 34.1 ns/op | 0 B/op | 0/op |

These numbers are a positioning reference, not a promise: CPU, Go release, corpus, and benchmark duration materially affect results. The useful trade-off is that `sutrie` provides compact immutable storage and prefix traversal, while a map is faster for exact membership and does not answer prefix queries. Use `BuildSuccinctTrieSorted` when sorting can be done once upstream; the construction benchmark shows the resulting savings.

The `domains.txt` fixture is sourced from [1Hosts Xtra domains.txt](https://raw.githubusercontent.com/badmojr/1Hosts/refs/heads/master/Xtra/domains.txt). It is over 20 MB and is kept as an external input rather than embedded in the package; refresh it from the source when updating benchmark data.

## License

See [LICENSE](LICENSE).
