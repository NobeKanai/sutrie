package sutrie

import (
	"container/list"
	"math/bits"
	"sort"
)

type SuccinctTrie struct {
	bitmap bitset
	leaves bitset
	nodes  []byte
}

// BuildSuccinctTrie builds a static trie which supports only searching on it
func BuildSuccinctTrie(dict []string) *SuccinctTrie {
	sort.Strings(dict)

	ret := &SuccinctTrie{
		nodes: make([]byte, 1),
	}

	zeroIdx := 1 // well this is actually one index cause that's easier
	queue := list.New()
	queue.PushBack(bfsNode{0, len(dict), 0})

	for queue.Len() > 0 {
		cur := queue.Front().Value.(bfsNode)
		queue.Remove(queue.Front())

		ret.bitmap.setBit(zeroIdx, true)
		zeroIdx++

		// make sure has child
		next := cur.l
		for next < cur.r && len(dict[next]) <= cur.depth {
			next++
		}

		for i := next; i < cur.r; {
			r := i + 1
			for r < cur.r && dict[i][cur.depth] == dict[r][cur.depth] {
				r++
			}

			ret.nodes = append(ret.nodes, dict[i][cur.depth])

			// touch bottom, this is a leaf
			if len(dict[i]) == cur.depth+1 {
				ret.leaves.setBit(len(ret.nodes)-1, true)
			}

			// now add the next level node
			queue.PushBack(bfsNode{i, r, cur.depth + 1})
			i = r
			zeroIdx++
		}
	}

	ret.bitmap.setBit(zeroIdx, true)

	return ret
}

// Search uses the match function to search through the trie level by level and returns true only if the last
// node it stays on is a leaf. In the match function:
//
// orderedCandicates: is the byte sequence represented by all the children of the current node of the trie,
// it is ordered, you can even do a binary search on it, although it is not necessary at all
// (because its length is at most 256). In order to save the search overhead,
// the orderedCandicates here is not a copy of a safe and modifiable value.
// You must be careful NOT to modify any of its values!
//
// prevIsLeaf: if the last matching node is a leaf node during the search, the value of prevIsLeaf will be true.
//
// return value: the index of the matching byte, or -1 if you want stop the search
func (t *SuccinctTrie) Search(match func(orderedCandicates []byte, prevIsLeaf bool) int) bool {
	node := 0 // current node

	for {
		firstChild := t.bitmap.selects(node+1) - node
		if firstChild >= len(t.nodes) {
			break
		}

		afterLastChild := t.bitmap.selects(node+2) - node - 1
		idx := match(t.nodes[firstChild:afterLastChild], t.leaves.getBit(node))

		if idx == -1 {
			break
		}

		node = firstChild + idx
	}

	return t.leaves.getBit(node)
}

// SearchPrefix searches the trie for the prefix of the key and returns the last index that does not match.
// When the match is a complete match, the return value is equal to the length of the key, and similarly,
// when the return value is 0, it means that there is no match at all.
// For example, suppose there is an entry "xx.yy" in the trie,
// when searching for "xx.yy.zz" or "xx.yy" it will return 5, when searching for "xx" or "bb" it will return 0
func (t *SuccinctTrie) SearchPrefix(key string) int {
	i := 0
	lastUnmatch := 0
	t.Search(func(orderedCandicates []byte, prevIsLeaf bool) int {
		if prevIsLeaf {
			lastUnmatch = i
		}

		if i >= len(key) {
			return -1
		}

		for j := 0; j < len(orderedCandicates); j++ {
			if orderedCandicates[j] == key[i] {
				i++
				return j
			}
		}

		return -1
	})

	return lastUnmatch
}

type bfsNode struct {
	l, r  int
	depth int
}

// --- bitset ---

type bitset struct {
	bits  []uint64
	ranks []int32
}

func (b *bitset) setBit(pos int, value bool) {
	for pos>>6 >= len(b.bits) {
		b.bits = append(b.bits, 0)
	}
	if value {
		b.bits[pos>>6] |= uint64(1) << (pos & 63)
	} else {
		b.bits[pos>>6] &^= uint64(1) << (pos & 63)
	}

	b.ranks = nil
}

func (b *bitset) getBit(pos int) bool {
	if pos>>6 >= len(b.bits) {
		return false
	}

	return b.bits[pos>>6]&(uint64(1)<<(pos&63)) > 0
}

func (b *bitset) initRanks() {
	b.ranks = make([]int32, len(b.bits)+1)
	for i := 0; i < len(b.bits); i++ {
		n := bits.OnesCount64(b.bits[i])
		b.ranks[i+1] = b.ranks[i] + int32(n)
	}
}

// rank does not include last bit
func (b *bitset) rank(pos int) int {
	if b.ranks == nil {
		b.initRanks()
	}

	if pos>>6 >= len(b.ranks)-1 {
		return int(b.ranks[len(b.ranks)-1])
	}

	return int(b.ranks[pos>>6]) + bits.OnesCount64(b.bits[pos>>6]&(uint64(1)<<(pos&63)-1))
}

func (b *bitset) selects(pos int) int {
	l, r := 0, len(b.bits)<<6-1
	for l < r {
		mid := (l + r + 1) >> 1
		if b.rank(mid) < pos {
			l = mid
		} else {
			r = mid - 1
		}
	}

	return l
}
