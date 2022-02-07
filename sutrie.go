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

// BuildSuccinctTrie builds a static tree which supports only Search on it
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

// Search searchs key in trie, the first return value will be true if the key is matched in full
// and the second return value will be true if the key is matched by the prefix only
// the third return value will be index after the last prefix match
// For example, when "xx.yy" in trie, Search("xx.yy.zz") would return false, true, 5
// Note that when the first return value (exact match) is true, the second value (prefix match) is also true
func (t *SuccinctTrie) Search(key string) (bool, bool, int) {
	node := 0 // current node
	isPrefix := false
	lastUnmatch := 0

	for i := 0; i < len(key); i++ {
		firstChild := t.bitmap.selects(node+1) - node
		if firstChild >= len(t.nodes) {
			return false, true, i // is prefix
		}

		if t.leaves.getBit(node) {
			isPrefix = true
			lastUnmatch = i
		}

		afterLastChild := t.bitmap.selects(node+2) - node - 1
		bs := t.binarySearchTrieNodes(key[i], firstChild, afterLastChild)
		if bs == -1 {
			return false, isPrefix, lastUnmatch // no next
		}

		node = bs
	}

	return t.leaves.getBit(node), isPrefix, lastUnmatch
}

func (t *SuccinctTrie) binarySearchTrieNodes(c byte, l, r int) int {
	for l < r {
		mid := (l + r) >> 1
		if t.nodes[mid] >= c {
			r = mid
		} else {
			l = mid + 1
		}
	}
	if t.nodes[l] != c {
		return -1
	}
	return l
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
