package sutrie

import (
	"bytes"
	"container/list"
	"encoding/gob"
	"math/bits"
	"sort"
)

type SuccinctTrie struct {
	bitmap bitset
	leaves bitset
	nodes  []byte
	size   int
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
				ret.size++
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

func (t *SuccinctTrie) search(node int, match func(children []byte, isLeaf bool, next func(int))) {
	firstChild := t.bitmap.selects(node+1) - node
	if firstChild >= len(t.nodes) {
		match([]byte{}, true, func(int) {})
		return
	}

	afterLastChild := t.bitmap.selects(node+2) - node - 1

	next := func(idx int) {
		t.search(firstChild+idx, match)
	}

	match(t.nodes[firstChild:afterLastChild], t.leaves.getBit(node), next)
	return
}

// Search uses the match function to traverse through the trie
// In the match function:
//
// children: is the byte sequence represents all the children of the current node of the trie,
// it is ordered, you can even do a binary search on it, although it is not necessary at all
// (because its length is at most 256). In order to save the search overhead,
// the children here is NOT a copy or, NOT a value that is allowed to be modified,
// You must be careful NOT to modify any of its values!
//
// isLeaf: if current node is a leaf node, the value of it will be true.
//
// next: call next(index of child) to move to that child node
func (t *SuccinctTrie) Search(match func(children []byte, isLeaf bool, next func(int))) {
	t.search(0, match)
}

// SearchPrefix searches the trie for the prefix of the key and returns the last index that does not match.
// When the match is a full match, the return value is equal to the length of the key, and similarly,
// when the return value is 0, it means that there is no match at all.
// For example, suppose there is an entry "xx.yy" in the trie,
// when searching for "xx.yy.zz" or "xx.yy" it will return 5, when searching for "xx" or "bb" it will return 0
func (t *SuccinctTrie) SearchPrefix(key string) int {
	i := 0
	lastUnmatch := 0
	t.Search(func(children []byte, isLeaf bool, next func(int)) {
		if isLeaf {
			lastUnmatch = i
		}

		if i >= len(key) {
			return
		}

		for k, c := range children {
			if c == key[i] {
				i++
				next(k)
				return
			}
		}
	})

	return lastUnmatch
}

// Size returns number of leaves in trie
func (t *SuccinctTrie) Size() int {
	return t.size
}

type wrapSuccinctTrie struct {
	BitmapBits []uint64
	LeavesBits []uint64
	Nodes      []byte
	Size       int
}

func (v *SuccinctTrie) MarshalBinary() ([]byte, error) {
	w := wrapSuccinctTrie{v.bitmap.bits, v.leaves.bits, v.nodes, v.size}

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(w); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (v *SuccinctTrie) UnmarshalBinary(data []byte) error {
	w := wrapSuccinctTrie{}

	reader := bytes.NewReader(data)
	dec := gob.NewDecoder(reader)
	if err := dec.Decode(&w); err != nil {
		return err
	}

	v.bitmap.bits = w.BitmapBits
	v.leaves.bits = w.LeavesBits
	v.bitmap.ranks = nil
	v.leaves.ranks = nil
	v.nodes = w.Nodes
	v.size = w.Size
	return nil
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
