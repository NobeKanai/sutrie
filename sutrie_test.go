package sutrie

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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

	// 4,5,127,128
	assert.Equal(t, 1, bs.rank(5))
	assert.Equal(t, 2, bs.rank(6))
	assert.Equal(t, 3, bs.rank(128))
	assert.Equal(t, 4, bs.rank(129))
	assert.Equal(t, 4, bs.rank(100000))

	assert.Equal(t, 0, bs.selects(0))
	assert.Equal(t, 4, bs.selects(1))
	assert.Equal(t, 5, bs.selects(2))
	assert.Equal(t, 127, bs.selects(3))
	assert.Equal(t, 128, bs.selects(4))
	assert.Equal(t, -1, bs.selects(5))
}

func TestBuildSuccinctTrie(t *testing.T) {
	dict := []string{"hat", "is", "it", "a"}
	trie := BuildSuccinctTrie(dict)

	assert.Equal(t, []byte{0, 'a', 'h', 'i', 'a', 's', 't', 't'}, trie.nodes)
	assert.Equal(t, "11110100101100010", fmt.Sprintf("%08b", trie.bitmap.bits[0]))

	assert.True(t, trie.leaves.getBit(1))
	assert.False(t, trie.leaves.getBit(2))
	assert.False(t, trie.leaves.getBit(3))
	assert.False(t, trie.leaves.getBit(4))
	assert.True(t, trie.leaves.getBit(5))
	assert.True(t, trie.leaves.getBit(6))
	assert.True(t, trie.leaves.getBit(7))
}

func TestSearchOnSuccinctTrie(t *testing.T) {
	dict := []string{"hat", "is", "it", "a"}
	trie := BuildSuccinctTrie(dict)

	exact, prefix := trie.Search("hat")
	assert.True(t, exact)
	assert.True(t, prefix)

	exact, prefix = trie.Search("istttt")
	assert.False(t, exact)
	assert.True(t, prefix)

	exact, prefix = trie.Search("ti")
	assert.False(t, exact)
	assert.False(t, prefix)
}

func loadLocalDomains() (ret []string) {
	bytes, err := os.ReadFile("domains.txt")
	if err != nil {
		panic(err)
	}

	for _, domain := range strings.Split(string(bytes), "\n") {
		ret = append(ret, domain)
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
	trie := BuildSuccinctTrie(domains)

	given := []string{
		"xxx.twitter.com",
		"bilibili.com",
		"example.top",
		"blog.example.top",
		"cdn.ark.qq.com",
		"google.com",
		"img.yandex.com",
		"fuuxkxkfjsdfsdf.hinatarin.com",
		"www.example.com",
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		trie.Search(given[i%9])
	}
}
