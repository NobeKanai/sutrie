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
	nodes  string
	size   int
}

type Node struct {
	trie           *SuccinctTrie
	firstChild     int32
	afterLastChild int32
	leaf           bool
}

// BuildSuccinctTrie constructs an immutable, succinct prefix tree/trie data structure.
// You can traverse the tree from root node, but you cannot modify it.
func BuildSuccinctTrie(dict []string) *SuccinctTrie {
	sort.Strings(dict)

	ret := &SuccinctTrie{}

	type bfsNode struct {
		l, r  int32
		depth int32
	}

	zeroIdx := 1 // well this is actually one index cause that's easier
	queue := newQueue[bfsNode](max(1, len(dict)))
	queue.push(bfsNode{0, int32(len(dict)), 0})
	nodes := make([]byte, 1)

	for queue.size() > 0 {
		cur := queue.pop()

		ret.bitmap.setBit(zeroIdx, true)
		zeroIdx++

		// make sure has child
		next := cur.l
		for next < cur.r && len(dict[next]) <= int(cur.depth) {
			next++
		}

		for i := next; i < cur.r; {
			r := i
			for b := (cur.r - i) >> 1; b >= 1; b >>= 1 {
				for r+b < cur.r && dict[i][cur.depth] == dict[r+b][cur.depth] {
					r += b
				}
			}
			r++

			nodes = append(nodes, dict[i][cur.depth])

			// touch bottom, this is a leaf
			if len(dict[i]) == int(cur.depth+1) {
				ret.leaves.setBit(len(nodes)-1, true)
				ret.size++
			}

			queue.push(bfsNode{i, r, cur.depth + 1})
			i = r
			zeroIdx++
		}
	}

	ret.nodes = string(nodes)
	ret.bitmap.setBit(zeroIdx, true)
	ret.bitmap.init()

	return ret
}

// Root returns root node of trie
func (t *SuccinctTrie) Root() Node {
	firstChild := t.bitmap.selects(1)
	if firstChild < 0 {
		return Node{
			leaf: false,
			trie: t,
		}
	} else {
		afterLastChild := t.bitmap.selects(2) - 1
		return Node{
			firstChild:     firstChild,
			afterLastChild: afterLastChild,
			leaf:           false,
			trie:           t,
		}
	}
}

// Exists returns the validity of the current node.
func (n Node) Exists() bool {
	return n.trie != nil
}

// Size returns the number of child nodes of the current node.
func (n Node) Size() int {
	return int(n.afterLastChild - n.firstChild)
}

// Leaf returns whether the current node is a leaf node, that is, whether the current node corresponds to a complete entry.
func (n Node) Leaf() bool {
	return n.leaf
}

// Children function returns a string of the sorted bytes corresponding to the edges of the current node’s child nodes in the trie.
func (n Node) Children() string {
	return n.trie.nodes[n.firstChild:n.afterLastChild]
}

func (n Node) next(node int32) Node {
	if node >= n.afterLastChild || node < 0 {
		return Node{}
	}

	firstChild := n.trie.bitmap.selects(node+1) - node
	if firstChild < 0 {
		return Node{
			leaf: true,
			trie: n.trie,
		}
	} else {
		afterLastChild := n.trie.bitmap.selects(node+2) - node - 1
		return Node{
			firstChild:     firstChild,
			afterLastChild: afterLastChild,
			leaf:           n.trie.leaves.getBit(node),
			trie:           n.trie,
		}
	}
}

// Next returns the next node corresponding to the byte b in the trie from the current node.
// Note that the returned node may be invalid. You can call Exists to determine its validity.
func (n Node) Next(b byte) Node {
	return n.next(n.trie.indexByte(n.firstChild, n.afterLastChild, b))
}

// Search is simply a wrapper around the Next function.
// It iterates through each byte in the string s within the trie,
// and returns the final node (note that the node may be a null node).
func (n Node) Search(s string) Node {
	for i := 0; i < len(s) && n.Exists(); i++ {
		n = n.Next(s[i])
	}
	return n
}

func (t *SuccinctTrie) indexByte(l, r int32, b byte) int32 {
	r--
	for r-l >= 15 {
		k := (l + r) >> 1
		if t.nodes[k] == b {
			return k
		} else if t.nodes[k] > b {
			r = k - 1
		} else {
			l = k + 1
		}
	}

	for i := l; i <= r; i++ {
		if t.nodes[i] == b {
			return i
		}
	}
	return -1
}

// SearchPrefix searches the trie for the prefix of the key and returns the last index that does not match.
// When the match is a full match, the return value is equal to the length of the key, and similarly,
// when the return value is 0, it means that there is no match at all.
// For example, suppose there is an entry "xx.yy" in the trie,
// when searching for "xx.yy.zz" or "xx.yy" it will return 5, when searching for "xx" or "bb" it will return 0
func (cur Node) SearchPrefix(key string) (lastUnmatch int) {
	for i := 0; i < len(key); i++ {
		if k := cur.trie.indexByte(cur.firstChild, cur.afterLastChild, key[i]); k != -1 {
			cur = cur.next(k)
			if cur.leaf {
				lastUnmatch = i + 1
			}
		} else {
			break
		}
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
	Nodes      string
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
	v.bitmap.sl = nil
	v.leaves.ranks = nil
	v.leaves.sl = nil
	v.nodes = w.Nodes
	v.size = w.Size

	v.bitmap.init()
	return nil
}

type bitset struct {
	bits  []uint64
	ranks []int32
	sl    []int32
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

func (b *bitset) getBit(pos int32) bool {
	if pos>>6 >= int32(len(b.bits)) {
		return false
	}

	return b.bits[pos>>6]&(uint64(1)<<(pos&63)) > 0
}

func (b *bitset) init() {
	for i := len(b.bits) - 1; i >= 0 && bits.OnesCount64(b.bits[i]) == 0; i-- {
		b.bits = b.bits[:i]
	}

	b.ranks = make([]int32, len(b.bits)+1)
	b.sl = make([]int32, len(b.bits)/2+1+(len(b.bits)&1))
	var t int32 = 1
	for i := 0; i < len(b.bits); i++ {
		n := bits.OnesCount64(b.bits[i])
		b.ranks[i+1] = b.ranks[i] + int32(n)
		if b.ranks[i+1]>>6 >= t {
			b.sl[t] = int32(i)
			t++
		}
	}
	b.sl[t] = int32(len(b.bits)) - 1
}

func (b *bitset) selects(nth int32) int32 {
	if b.ranks[len(b.ranks)-1] < nth {
		return -1
	}

	l, r := b.sl[nth>>6], b.sl[nth>>6+1]
	for ; l < r && b.ranks[l+1] < int32(nth); l++ {
	}

	return l<<6 + int32(nthSet(b.bits[l], uint8(nth-b.ranks[l]-1)))
}

const pop8tab = "" +
	"\x00\x01\x01\x02\x01\x02\x02\x03\x01\x02\x02\x03\x02\x03\x03\x04" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x01\x02\x02\x03\x02\x03\x03\x04\x02\x03\x03\x04\x03\x04\x04\x05" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x02\x03\x03\x04\x03\x04\x04\x05\x03\x04\x04\x05\x04\x05\x05\x06" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x03\x04\x04\x05\x04\x05\x05\x06\x04\x05\x05\x06\x05\x06\x06\x07" +
	"\x04\x05\x05\x06\x05\x06\x06\x07\x05\x06\x06\x07\x06\x07\x07\x08"

var precomp = [8]string{
	"\x00\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x06\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\a\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x06\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x05\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00\x04\x00\x01\x00\x02\x00\x01\x00\x03\x00\x01\x00\x02\x00\x01\x00",
	"\x00\x00\x00\x01\x00\x02\x02\x01\x00\x03\x03\x01\x03\x02\x02\x01\x00\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\x00\x05\x05\x01\x05\x02\x02\x01\x05\x03\x03\x01\x03\x02\x02\x01\x05\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\x00\x06\x06\x01\x06\x02\x02\x01\x06\x03\x03\x01\x03\x02\x02\x01\x06\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\x06\x05\x05\x01\x05\x02\x02\x01\x05\x03\x03\x01\x03\x02\x02\x01\x05\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\x00\a\a\x01\a\x02\x02\x01\a\x03\x03\x01\x03\x02\x02\x01\a\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\a\x05\x05\x01\x05\x02\x02\x01\x05\x03\x03\x01\x03\x02\x02\x01\x05\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\a\x06\x06\x01\x06\x02\x02\x01\x06\x03\x03\x01\x03\x02\x02\x01\x06\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01\x06\x05\x05\x01\x05\x02\x02\x01\x05\x03\x03\x01\x03\x02\x02\x01\x05\x04\x04\x01\x04\x02\x02\x01\x04\x03\x03\x01\x03\x02\x02\x01",
	"\x00\x00\x00\x00\x00\x00\x00\x02\x00\x00\x00\x03\x00\x03\x03\x02\x00\x00\x00\x04\x00\x04\x04\x02\x00\x04\x04\x03\x04\x03\x03\x02\x00\x00\x00\x05\x00\x05\x05\x02\x00\x05\x05\x03\x05\x03\x03\x02\x00\x05\x05\x04\x05\x04\x04\x02\x05\x04\x04\x03\x04\x03\x03\x02\x00\x00\x00\x06\x00\x06\x06\x02\x00\x06\x06\x03\x06\x03\x03\x02\x00\x06\x06\x04\x06\x04\x04\x02\x06\x04\x04\x03\x04\x03\x03\x02\x00\x06\x06\x05\x06\x05\x05\x02\x06\x05\x05\x03\x05\x03\x03\x02\x06\x05\x05\x04\x05\x04\x04\x02\x05\x04\x04\x03\x04\x03\x03\x02\x00\x00\x00\a\x00\a\a\x02\x00\a\a\x03\a\x03\x03\x02\x00\a\a\x04\a\x04\x04\x02\a\x04\x04\x03\x04\x03\x03\x02\x00\a\a\x05\a\x05\x05\x02\a\x05\x05\x03\x05\x03\x03\x02\a\x05\x05\x04\x05\x04\x04\x02\x05\x04\x04\x03\x04\x03\x03\x02\x00\a\a\x06\a\x06\x06\x02\a\x06\x06\x03\x06\x03\x03\x02\a\x06\x06\x04\x06\x04\x04\x02\x06\x04\x04\x03\x04\x03\x03\x02\a\x06\x06\x05\x06\x05\x05\x02\x06\x05\x05\x03\x05\x03\x03\x02\x06\x05\x05\x04\x05\x04\x04\x02\x05\x04\x04\x03\x04\x03\x03\x02",
	"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x03\x00\x00\x00\x00\x00\x00\x00\x04\x00\x00\x00\x04\x00\x04\x04\x03\x00\x00\x00\x00\x00\x00\x00\x05\x00\x00\x00\x05\x00\x05\x05\x03\x00\x00\x00\x05\x00\x05\x05\x04\x00\x05\x05\x04\x05\x04\x04\x03\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x06\x00\x06\x06\x03\x00\x00\x00\x06\x00\x06\x06\x04\x00\x06\x06\x04\x06\x04\x04\x03\x00\x00\x00\x06\x00\x06\x06\x05\x00\x06\x06\x05\x06\x05\x05\x03\x00\x06\x06\x05\x06\x05\x05\x04\x06\x05\x05\x04\x05\x04\x04\x03\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x03\x00\x00\x00\a\x00\a\a\x04\x00\a\a\x04\a\x04\x04\x03\x00\x00\x00\a\x00\a\a\x05\x00\a\a\x05\a\x05\x05\x03\x00\a\a\x05\a\x05\x05\x04\a\x05\x05\x04\x05\x04\x04\x03\x00\x00\x00\a\x00\a\a\x06\x00\a\a\x06\a\x06\x06\x03\x00\a\a\x06\a\x06\x06\x04\a\x06\x06\x04\x06\x04\x04\x03\x00\a\a\x06\a\x06\x06\x05\a\x06\x06\x05\x06\x05\x05\x03\a\x06\x06\x05\x06\x05\x05\x04\x06\x05\x05\x04\x05\x04\x04\x03",
	"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x04\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x05\x00\x00\x00\x00\x00\x00\x00\x05\x00\x00\x00\x05\x00\x05\x05\x04\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x06\x00\x06\x06\x04\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x06\x00\x06\x06\x05\x00\x00\x00\x06\x00\x06\x06\x05\x00\x06\x06\x05\x06\x05\x05\x04\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x04\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x05\x00\x00\x00\a\x00\a\a\x05\x00\a\a\x05\a\x05\x05\x04\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x06\x00\x00\x00\a\x00\a\a\x06\x00\a\a\x06\a\x06\x06\x04\x00\x00\x00\a\x00\a\a\x06\x00\a\a\x06\a\x06\x06\x05\x00\a\a\x06\a\x06\x06\x05\a\x06\x06\x05\x06\x05\x05\x04",
	"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x05\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x06\x00\x06\x06\x05\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x05\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x06\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x06\x00\x00\x00\a\x00\a\a\x06\x00\a\a\x06\a\x06\x06\x05",
	"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x06\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\x00\x00\x00\x00\a\x00\x00\x00\a\x00\a\a\x06",
	"\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\a",
}

func nthSet(v uint64, n uint8) uint8 {
	shift := uint8(0)
	p := pop8tab[v>>24&0xff] + pop8tab[v>>16&0xff] + pop8tab[v>>8&0xff] + pop8tab[v&0xff]
	if p <= n {
		v >>= 32
		shift += 32
		n -= p
	}
	p = pop8tab[(v>>8)&0xff] + pop8tab[v&0xff]
	if p <= n {
		v >>= 16
		shift += 16
		n -= p
	}
	p = pop8tab[v&0xff]
	if p <= n {
		shift += 8
		v >>= 8
		n -= p
	}
	return precomp[n][v&0xff] + shift
}

type queue[T any] struct {
	data  []T
	l, sz uint32
}

func newQueue[T any](size int) queue[T] {
	return queue[T]{
		data: make([]T, size),
	}
}

func (q *queue[T]) push(elm T) {
	q.data[int(q.l+q.sz)%len(q.data)] = elm
	q.sz++
}

func (q *queue[T]) size() int {
	return int(q.sz)
}

func (q *queue[T]) pop() T {
	if q.sz == 0 {
		panic("pop: no element")
	}

	ret := q.data[q.l]
	q.l = (q.l + 1) % uint32(len(q.data))
	q.sz--
	return ret
}
