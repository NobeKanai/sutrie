package sutrie

import (
	"encoding/gob"
	"io"
	"math/bits"
	"sort"
)

type SuccinctTrie struct {
	bitmap bitset
	leaves bitset
	nodes  []byte
	size   int
}

type Node struct {
	firstChild int
	Children   []byte
	Leaf       bool
}

// BuildSuccinctTrie builds a static trie which supports only searching on it
func BuildSuccinctTrie(dict []string) *SuccinctTrie {
	sort.Strings(dict)

	ret := &SuccinctTrie{
		nodes: make([]byte, 1),
	}

	type bfsNode struct {
		l, r  int
		depth int
	}

	zeroIdx := 1 // well this is actually one index cause that's easier
	queue := []bfsNode{{0, len(dict), 0}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

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

			queue = append(queue, bfsNode{i, r, cur.depth + 1})
			i = r
			zeroIdx++
		}
	}

	ret.bitmap.setBit(zeroIdx, true)
	ret.bitmap.initRanks()
	return ret
}

// Root returns root node of trie
func (t *SuccinctTrie) Root() Node {
	firstChild := t.bitmap.selects(1)
	if firstChild >= len(t.nodes) {
		return Node{
			Children: nil,
			Leaf:     false,
		}
	} else {
		afterLastChild := t.bitmap.selects(2) - 1
		return Node{
			firstChild: firstChild,
			Children:   t.nodes[firstChild:afterLastChild],
			Leaf:       false,
		}
	}
}

// Next moves to children of node n. childIdx is the index of n.Children, which should point
// to the byte you want to move to.
func (t *SuccinctTrie) Next(n Node, childIdx int) Node {
	if childIdx >= len(n.Children) {
		panic("index outbound")
	}

	node := n.firstChild + childIdx

	firstChild := t.bitmap.selects(node+1) - node
	if firstChild >= len(t.nodes) {
		return Node{
			Children: nil,
			Leaf:     true,
		}
	} else {
		afterLastChild := t.bitmap.selects(node+2) - node - 1
		return Node{
			firstChild: firstChild,
			Children:   t.nodes[firstChild:afterLastChild],
			Leaf:       t.leaves.getBit(node),
		}
	}
}

// SearchPrefix searches the trie for the prefix of the key and returns the last index that does not match.
// When the match is a full match, the return value is equal to the length of the key, and similarly,
// when the return value is 0, it means that there is no match at all.
// For example, suppose there is an entry "xx.yy" in the trie,
// when searching for "xx.yy.zz" or "xx.yy" it will return 5, when searching for "xx" or "bb" it will return 0
func (t *SuccinctTrie) SearchPrefix(key string) (lastUnmatch int) {
	cur := t.Root()
	for i := 0; i < len(key); i++ {
		for k, c := range cur.Children {
			if c == key[i] {
				cur = t.Next(cur, k)
				if cur.Leaf {
					lastUnmatch = i + 1
				}
				goto CONTINUE
			}
		}
		break
	CONTINUE:
	}

	return
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

func (v *SuccinctTrie) Marshal(writer io.Writer) error {
	w := wrapSuccinctTrie{v.bitmap.bits, v.leaves.bits, v.nodes, v.size}

	enc := gob.NewEncoder(writer)
	return enc.Encode(w)
}

func (v *SuccinctTrie) Unmarshal(reader io.Reader) error {
	w := wrapSuccinctTrie{}

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

	v.bitmap.initRanks()
	return nil
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
