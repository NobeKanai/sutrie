package sutrie

import (
	"bufio"
	"os"
	"sort"
	"strings"
	"testing"

	derektrie "github.com/derekparker/trie"
)

const (
	domainsSourceURL    = "https://raw.githubusercontent.com/badmojr/1Hosts/refs/heads/master/Xtra/domains.txt"
	benchmarkDomainCap  = 100_000
	benchmarkQueryCount = 10_000
)

// Benchmark data is a bounded sample so the suite remains useful on a laptop.
// The complete domains.txt fixture is intentionally kept in the repository for
// reproducible, larger experiments.
func benchmarkDomains(tb testing.TB) []string {
	tb.Helper()

	file, err := os.Open("domains.txt")
	if err != nil {
		tb.Skipf("domains.txt is not available: %v (source: %s)", err, domainsSourceURL)
	}
	defer file.Close()

	domains := make([]string, 0, benchmarkDomainCap)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1<<20)
	for scanner.Scan() && len(domains) < benchmarkDomainCap {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		domains = append(domains, reverseBytes(line))
	}
	if err := scanner.Err(); err != nil {
		tb.Fatalf("reading domains.txt: %v", err)
	}
	if len(domains) == 0 {
		tb.Fatal("domains.txt contains no benchmark domains")
	}
	return domains
}

func reverseBytes(value string) string {
	bytes := []byte(value)
	for left, right := 0, len(bytes)-1; left < right; left, right = left+1, right-1 {
		bytes[left], bytes[right] = bytes[right], bytes[left]
	}
	return string(bytes)
}

func BenchmarkBuildSuccinctTrie(b *testing.B) {
	domains := benchmarkDomains(b)
	sortedDomains := append([]string(nil), domains...)
	sort.Strings(sortedDomains)
	b.ReportMetric(float64(len(domains)), "keys")

	b.Run("sort-and-build", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			input := append([]string(nil), domains...)
			BuildSuccinctTrie(input)
		}
	})

	b.Run("sorted-input", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			BuildSuccinctTrieSorted(sortedDomains)
		}
	})

	b.Run("derekparker-trie", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			trie := derektrie.New()
			for _, domain := range domains {
				trie.Add(domain, nil)
			}
		}
	})
}

func BenchmarkSearchExact(b *testing.B) {
	domains := benchmarkDomains(b)
	sutrieRoot := BuildSuccinctTrie(append([]string(nil), domains...)).Root()
	mapIndex := make(map[string]struct{}, len(domains))
	derek := derektrie.New()
	for _, domain := range domains {
		mapIndex[domain] = struct{}{}
		derek.Add(domain, nil)
	}

	queries := make([]string, 0, benchmarkQueryCount*2)
	for i := 0; i < benchmarkQueryCount; i++ {
		queries = append(queries, domains[i%len(domains)])
		candidate := "missing." + domains[i%len(domains)]
		for {
			if _, exists := mapIndex[candidate]; !exists {
				break
			}
			candidate = "x." + candidate
		}
		queries = append(queries, candidate)
	}

	b.ReportMetric(float64(len(domains)), "keys")
	b.Run("sutrie", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			want := i%2 == 0
			if got := sutrieRoot.Search(queries[i%len(queries)]).Leaf(); got != want {
				b.Fatalf("Search returned %t, want %t", got, want)
			}
		}
	})
	b.Run("derekparker-trie", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			want := i%2 == 0
			_, got := derek.Find(queries[i%len(queries)])
			if got != want {
				b.Fatalf("Find returned %t, want %t", got, want)
			}
		}
	})
	b.Run("map", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			want := i%2 == 0
			_, got := mapIndex[queries[i%len(queries)]]
			if got != want {
				b.Fatalf("map returned %t, want %t", got, want)
			}
		}
	})
}

func BenchmarkSearchPrefix(b *testing.B) {
	domains := benchmarkDomains(b)
	root := BuildSuccinctTrie(append([]string(nil), domains...)).Root()
	queries := []string{
		"com",
		"moc.",
		"moc.elgoog",
		"moc.elgoog.www",
		"missing.example",
	}

	b.ReportMetric(float64(len(domains)), "keys")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if root.SearchPrefix(queries[i%len(queries)]) < 0 {
			b.Fatal("SearchPrefix returned a negative length")
		}
	}
}

func BenchmarkSearchExactParallel(b *testing.B) {
	domains := benchmarkDomains(b)
	root := BuildSuccinctTrie(append([]string(nil), domains...)).Root()

	b.ReportMetric(float64(len(domains)), "keys")
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if !root.Search(domains[i%len(domains)]).Leaf() {
				b.Fatal("parallel search missed an inserted key")
			}
			i++
		}
	})
}
