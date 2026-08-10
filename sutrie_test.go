package sutrie

import (
	"bytes"
	"fmt"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestBitset(t *testing.T) {
	var bs bitset

	bs.setBit(4, true)
	bs.setBit(567, true)

	if !bs.getBit(4) || !bs.getBit(567) {
		t.Fatal("set bits were not readable")
	}
	if bs.getBit(8) || bs.getBit(568) {
		t.Fatal("unset bits were reported as set")
	}

	bs.setBit(567, false)
	if bs.getBit(567) || !bs.getBit(4) {
		t.Fatal("clearing a bit changed the wrong bit")
	}

	bs.setBit(5, true)
	bs.setBit(127, true)
	bs.setBit(128, true)
	bs.init()

	for nth, want := range []int32{4, 5, 127, 128} {
		if got := bs.selects(int32(nth + 1)); got != want {
			t.Fatalf("select(%d) = %d, want %d", nth+1, got, want)
		}
	}
	if got := bs.selects(5); got != -1 {
		t.Fatalf("select past the end = %d, want -1", got)
	}
}

func TestNthSet(t *testing.T) {
	var value uint64 = 0b1010101011
	for _, test := range []struct {
		nth  uint8
		want uint8
	}{{nth: 0, want: 0}, {nth: 1, want: 1}, {nth: 3, want: 5}} {
		if got := nthSet(value, test.nth); got != test.want {
			t.Fatalf("nthSet(%d) = %d, want %d", test.nth, got, test.want)
		}
	}

	value = 1<<64 - 1
	for i := 0; i < 64; i++ {
		if got := nthSet(value, uint8(i)); got != uint8(i) {
			t.Fatalf("nthSet(all bits, %d) = %d, want %d", i, got, i)
		}
	}
}

func TestBuildSuccinctTrie(t *testing.T) {
	dict := []string{"hat", "is", "it", "a"}
	trie := BuildSuccinctTrie(dict)

	if want := []string{"a", "hat", "is", "it"}; !reflect.DeepEqual(dict, want) {
		t.Fatalf("BuildSuccinctTrie did not sort dict in place: got %q, want %q", dict, want)
	}
	if got, want := trie.nodes, string([]byte{0, 'a', 'h', 'i', 'a', 's', 't', 't'}); got != want {
		t.Fatalf("nodes = %q, want %q", got, want)
	}
	if got, want := fmt.Sprintf("%08b", trie.bitmap.bits[0]), "11110100101100010"; got != want {
		t.Fatalf("bitmap = %s, want %s", got, want)
	}

	for pos, want := range map[int32]bool{1: true, 2: false, 3: false, 4: false, 5: true, 6: true, 7: true} {
		if got := trie.leaves.getBit(pos); got != want {
			t.Fatalf("leaf bit %d = %t, want %t", pos, got, want)
		}
	}
	if trie.size != 4 {
		t.Fatalf("size = %d, want 4", trie.size)
	}

	root := trie.Root()
	if got := root.Size(); got != 3 {
		t.Fatalf("root size = %d, want 3", got)
	}
	if root.Leaf() {
		t.Fatal("root unexpectedly marked as a leaf")
	}

	first := root.next(root.firstChild)
	if first.Size() != 0 || !first.Leaf() {
		t.Fatalf("first child = %+v, want a leaf without children", first)
	}
}

func TestBuildSuccinctTrieSorted(t *testing.T) {
	dict := []string{"a", "hat", "is", "it"}
	before := append([]string(nil), dict...)
	trie := BuildSuccinctTrieSorted(dict)

	if !reflect.DeepEqual(dict, before) {
		t.Fatalf("BuildSuccinctTrieSorted changed dict: got %q, want %q", dict, before)
	}
	if trie.Size() != 4 || !trie.Root().Search("hat").Leaf() {
		t.Fatalf("sorted builder produced an invalid trie")
	}
}

func TestBuildEmptySuccinctTrie(t *testing.T) {
	for _, dict := range [][]string{nil, {}, {"", "", ""}} {
		trie := BuildSuccinctTrie(dict)
		if trie.Size() != 0 {
			t.Fatalf("BuildSuccinctTrie(%q).Size() = %d, want 0", dict, trie.Size())
		}
		if trie.Root().Children() != "" {
			t.Fatalf("empty trie has children: %q", trie.Root().Children())
		}
	}
}

func TestSearchPrefixOnSuccinctTrie(t *testing.T) {
	root := BuildSuccinctTrie([]string{"hat", "is", "it", "a"}).Root()

	for _, test := range []struct {
		key  string
		want int
	}{
		{key: "hat", want: 3},
		{key: "hatt", want: 3},
		{key: "iss", want: 2},
		{key: "ti", want: 0},
		{key: "a", want: 1},
	} {
		if got := root.SearchPrefix(test.key); got != test.want {
			t.Errorf("SearchPrefix(%q) = %d, want %d", test.key, got, test.want)
		}
	}
}

func TestSearchAndChildren(t *testing.T) {
	root := BuildSuccinctTrie([]string{"car", "cat", "dog"}).Root()

	if got := root.Children(); got != "cd" {
		t.Fatalf("root children = %q, want %q", got, "cd")
	}
	for _, test := range []struct {
		key  string
		leaf bool
	}{
		{key: "car", leaf: true},
		{key: "cat", leaf: true},
		{key: "ca", leaf: false},
		{key: "can", leaf: false},
		{key: "dog", leaf: true},
		{key: "dove", leaf: false},
	} {
		node := root.Search(test.key)
		if got := node.Exists() && node.Leaf(); got != test.leaf {
			t.Errorf("Search(%q) leaf = %t, want %t", test.key, got, test.leaf)
		}
	}
}

func TestDuplicateAndEmptyStringBehavior(t *testing.T) {
	root := BuildSuccinctTrie([]string{"", "", "abc", "abc", "ab"}).Root()

	if root.Search("").Leaf() {
		t.Fatal("empty string should not be represented as a leaf")
	}
	if !root.Search("abc").Leaf() || !root.Search("ab").Leaf() {
		t.Fatal("non-empty entries were not represented as leaves")
	}
	if got := root.trie.Size(); got != 2 {
		t.Fatalf("Size() = %d, want 2 unique non-empty entries", got)
	}
}

func TestRandomSearchOnSuccinctTrie(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	const count = 20_000

	dict := make([]string, count)
	exists := make(map[string]struct{}, count)
	for i := range dict {
		for {
			candidate := randomString(rng, 10+rng.Intn(11))
			if _, ok := exists[candidate]; ok {
				continue
			}
			dict[i] = candidate
			exists[candidate] = struct{}{}
			break
		}
	}

	root := BuildSuccinctTrie(dict).Root()
	for key := range exists {
		if !root.Search(key).Leaf() {
			t.Fatalf("Search(%q) did not find an inserted key", key)
		}
	}
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("missing-%08d", i)
		if root.Search(key).Leaf() {
			t.Fatalf("Search(%q) found a key that was not inserted", key)
		}
	}
}

func TestMarshalBinary(t *testing.T) {
	keys := []string{"hat", "is", "it", "a", "中文"}
	trie := BuildSuccinctTrie(append([]string(nil), keys...))

	var buf bytes.Buffer
	if err := trie.Marshal(&buf); err != nil {
		t.Fatalf("Marshal() failed: %v", err)
	}

	var decoded SuccinctTrie
	if err := decoded.Unmarshal(&buf); err != nil {
		t.Fatalf("Unmarshal() failed: %v", err)
	}
	if decoded.Size() != trie.Size() {
		t.Fatalf("decoded size = %d, want %d", decoded.Size(), trie.Size())
	}

	root := decoded.Root()
	for _, key := range keys {
		if !root.Search(key).Leaf() {
			t.Fatalf("decoded trie did not find %q", key)
		}
	}
	if got := root.SearchPrefix("iss"); got != 2 {
		t.Fatalf("decoded SearchPrefix(iss) = %d, want 2", got)
	}
}

func TestUnmarshalRejectsInvalidData(t *testing.T) {
	var trie SuccinctTrie
	if err := trie.Unmarshal(strings.NewReader("not a gob")); err == nil {
		t.Fatal("Unmarshal accepted invalid data")
	}
}

func FuzzBuildAndSearch(f *testing.F) {
	for _, seed := range []string{"a\x00ab\x00", "www.example.com\x00example.com", ""} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > 4096 {
			t.Skip()
		}
		keys := strings.Split(input, "\x00")
		expected := make(map[string]struct{}, len(keys))
		for _, key := range keys {
			if key != "" {
				expected[key] = struct{}{}
			}
		}

		root := BuildSuccinctTrie(keys).Root()
		for key := range expected {
			if !root.Search(key).Leaf() {
				t.Fatalf("Search(%q) did not find an inserted key", key)
			}
		}
	})
}

func randomString(rng *rand.Rand, length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789.-_"
	buf := make([]byte, length)
	for i := range buf {
		buf[i] = alphabet[rng.Intn(len(alphabet))]
	}
	return string(buf)
}

func TestSortedBuilderMatchesSortingBuilder(t *testing.T) {
	unsorted := []string{"z", "a", "alpha", "alphabet", ""}
	sorted := append([]string(nil), unsorted...)
	sort.Strings(sorted)

	got := BuildSuccinctTrieSorted(sorted)
	want := BuildSuccinctTrie(append([]string(nil), unsorted...))
	if !reflect.DeepEqual(got, want) {
		t.Fatal("sorted and sorting builders produced different tries")
	}
}
