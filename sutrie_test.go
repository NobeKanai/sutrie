package sutrie

import (
	"bytes"
	"crypto/rand"
	"fmt"
	mrand "math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: find a better test approach

func TestBitset(t *testing.T) {
	bs := bitset{}

	bs.setBit(4, true)
	bs.setBit(567, true)

	assert.True(t, bs.getBit(4))
	assert.True(t, bs.getBit(567))
	assert.False(t, bs.getBit(8))
	assert.False(t, bs.getBit(568))

	bs.setBit(567, false)
	assert.False(t, bs.getBit(567))
	assert.True(t, bs.getBit(4))

	bs.setBit(5, true)
	bs.setBit(128, true)
	bs.setBit(127, true)

	bs.init()

	// 4,5,127,128
	assert.Equal(t, int32(4), bs.selects(1))
	assert.Equal(t, int32(5), bs.selects(2))
	assert.Equal(t, int32(127), bs.selects(3))
	assert.Equal(t, int32(128), bs.selects(4))
	assert.Equal(t, int32(-1), bs.selects(5))
}

func TestNthSet(t *testing.T) {
	var n uint64 = 0b1010101011

	assert.Equal(t, uint8(0), nthSet(n, 0))
	assert.Equal(t, uint8(1), nthSet(n, 1))
	assert.Equal(t, uint8(5), nthSet(n, 3))

	n = 1<<64 - 1

	for i := 0; i < 64; i++ {
		assert.Equal(t, uint8(i), nthSet(n, uint8(i)))
	}
}

func TestBuildSuccinctTrie(t *testing.T) {
	dict := []string{"hat", "is", "it", "a"}
	trie := BuildSuccinctTrie(dict)

	assert.Equal(t, string([]byte{0, 'a', 'h', 'i', 'a', 's', 't', 't'}), trie.nodes)
	assert.Equal(t, "11110100101100010", fmt.Sprintf("%08b", trie.bitmap.bits[0]))

	assert.True(t, trie.leaves.getBit(1))
	assert.False(t, trie.leaves.getBit(2))
	assert.False(t, trie.leaves.getBit(3))
	assert.False(t, trie.leaves.getBit(4))
	assert.True(t, trie.leaves.getBit(5))
	assert.True(t, trie.leaves.getBit(6))
	assert.True(t, trie.leaves.getBit(7))

	assert.Equal(t, 4, trie.size)

	node := trie.Root()
	assert.Equal(t, int32(3), node.afterLastChild-node.firstChild)
	assert.False(t, node.leaf)

	node = node.next(node.firstChild)
	assert.Equal(t, int32(0), node.afterLastChild-node.firstChild)
	assert.True(t, node.leaf)
}

func TestBuildEmptySuccinctTrie(t *testing.T) {
	dict := []string{}
	trie := BuildSuccinctTrie(dict)

	assert.Equal(t, 0, trie.Size())
}

func TestSearchPrefixOnSuccinctTrie(t *testing.T) {
	dict := []string{"hat", "is", "it", "a"}

	trie := BuildSuccinctTrie(dict).Root()

	lastUnmatch := trie.SearchPrefix("hat")
	assert.Equal(t, 3, lastUnmatch)

	lastUnmatch = trie.SearchPrefix("iss")
	assert.Equal(t, 2, lastUnmatch)

	lastUnmatch = trie.SearchPrefix("ti")
	assert.Equal(t, 0, lastUnmatch)
}

func randomString(length int) string {
	x := make([]byte, length)
	l, err := rand.Read(x)
	if l != length || err != nil {
		panic("failed generating random string")
	}
	return string(x)
}

func TestRandomSearchOnSuccinctTrie(t *testing.T) {
	const l = 100000
	dict := make([]string, l)
	exists := make(map[string]struct{})
	for i := 0; i < l; i++ {
		dict[i] = randomString(10 + mrand.Intn(11))
		exists[dict[i]] = struct{}{}
	}

	trie := BuildSuccinctTrie(dict).Root()

	for i := 0; i < l; i++ {
		assert.True(t, trie.Search(dict[i]).Leaf())
	}

	for i := 0; i < l; i++ {
		rs := randomString(10 + mrand.Intn(11))
		_, ok := exists[rs]
		assert.Equal(t, ok, trie.Search(rs).Leaf())
	}
}

func TestEmptyStringBehaviorSuccinctTrie(t *testing.T) {
	trie := BuildSuccinctTrie([]string{"", "", "abc"}).Root()

	assert.True(t, trie.Search("abc").Leaf())
	assert.False(t, trie.Search("").Leaf())
	assert.False(t, trie.Leaf())
	assert.Equal(t, 1, trie.Size())
}

func TestMarshalBinary(t *testing.T) {
	var buf bytes.Buffer

	dict := []string{"hat", "is", "it", "a", "中文"}
	trie := BuildSuccinctTrie(dict)

	err := trie.Marshal(&buf)
	if err != nil {
		assert.FailNow(t, "failed to marshal trie to binary")
	}

	var decTrie SuccinctTrie
	err = decTrie.Unmarshal(&buf)
	if err != nil {
		assert.FailNow(t, "failed to unmarshal binary to trie")
	}

	assert.Equal(t, 5, decTrie.size)

	root := trie.Root()

	lastUnmatch := root.SearchPrefix("hat")
	assert.Equal(t, 3, lastUnmatch)

	lastUnmatch = root.SearchPrefix("iss")
	assert.Equal(t, 2, lastUnmatch)

	lastUnmatch = root.SearchPrefix("ti")
	assert.Equal(t, 0, lastUnmatch)
}

func loadLocalDomains() (ret []string) {
	bytes, err := os.ReadFile("domains.txt")
	if err != nil {
		panic(err)
	}

	for i := 0; i < len(bytes); {
		j := i + 1
		for j < len(bytes) && bytes[j] != '\n' {
			j++
		}

		d := bytes[i:j]
		for i2, j2 := 0, len(d)-1; i2 < j2; i2, j2 = i2+1, j2-1 {
			d[i2], d[j2] = d[j2], d[i2]
		}

		ret = append(ret, string(d))
		i = j + 1
	}

	return
}

func BenchmarkBuildSuccinctTrie(b *testing.B) {
	domains := loadLocalDomains()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		BuildSuccinctTrie(domains)
	}
}

func BenchmarkSearchOnSuccinctTrie(b *testing.B) {
	domains := loadLocalDomains()
	trie := BuildSuccinctTrie(domains).Root()

	given := []string{
		"xxx.twitter.com",
		"bilibili.com",
		"example.top",
		"blog.example.top",
		"cdn.ark.qq.com",
		"google.com",
		"img.yandex.com",
		"fuuxkxkfjsdfsdf.ddddddd.com",
		"www.example.com",
		"a.b.c.d.e.f.g.h",
		"abc.def",
		"a.b.c.d.e.f.google.com",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.SearchPrefix(given[i%12])
	}
}

func BenchmarkRandomSearchOnSuccinctTrie(b *testing.B) {
	const l = 1000000
	dict := make([]string, l)
	dict2 := make([]string, l)
	exists := make(map[string]struct{})
	for i := 0; i < l; i++ {
		dict[i] = randomString(10 + mrand.Intn(11))
		exists[dict[i]] = struct{}{}
	}

	for i := 0; i < l; i++ {
		dict2[i] = randomString(10 + mrand.Intn(11))
		if _, ok := exists[dict2[i]]; ok {
			i--
		}
	}

	trie := BuildSuccinctTrie(dict).Root()

	b.Run("sutrie-exists", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if !trie.Search(dict[i%l]).Leaf() {
				b.FailNow()
			}
		}
	})

	b.Run("sutrie-nonexist", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if trie.Search(dict2[i%l]).Leaf() {
				b.FailNow()
			}
		}
	})

	b.Run("map-exists", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, ok := exists[dict[i%l]]; !ok {
				b.FailNow()
			}
		}
	})

	b.Run("map-nonexist", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, ok := exists[dict2[i%l]]; ok {
				b.FailNow()
			}
		}
	})
}
