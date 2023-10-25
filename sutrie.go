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
	trie           *SuccinctTrie
	firstChild     int
	afterLastChild int
	leaf           bool
}

// BuildSuccinctTrie constructs an immutable, succinct prefix tree/trie data structure.
// You can traverse the tree from root node, but you cannot modify it.
func BuildSuccinctTrie(dict []string) *SuccinctTrie {
	sort.Strings(dict)

	ret := &SuccinctTrie{
		nodes: make([]byte, 1),
	}

	type bfsNode struct {
		l, r  int32
		depth int32
	}

	zeroIdx := 1 // well this is actually one index cause that's easier
	queue := queue[bfsNode]{}
	queue.push(bfsNode{0, int32(len(dict)), 0})

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

			ret.nodes = append(ret.nodes, dict[i][cur.depth])

			// touch bottom, this is a leaf
			if len(dict[i]) == int(cur.depth+1) {
				ret.leaves.setBit(len(ret.nodes)-1, true)
				ret.size++
			}

			queue.push(bfsNode{i, r, cur.depth + 1})
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
	return n.afterLastChild - n.firstChild
}

// Leaf returns whether the current node is a leaf node, that is, whether the current node corresponds to a complete entry.
func (n Node) Leaf() bool {
	return n.leaf
}

// ChildBytes function returns a copy of the bytes corresponding to the edges of the current node’s child nodes in the trie.
func (n Node) ChildBytes() []byte {
	if !n.Exists() {
		return []byte{}
	}

	dst := make([]byte, n.Size())
	copy(dst, n.trie.nodes[n.firstChild:n.afterLastChild])
	return dst
}

func (n Node) next(childIdx int) Node {
	if childIdx >= n.afterLastChild-n.firstChild || childIdx < 0 {
		return Node{}
	}

	node := n.firstChild + childIdx

	firstChild := n.trie.bitmap.selects(node+1) - node
	if firstChild >= len(n.trie.nodes) {
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

func (t *SuccinctTrie) indexByte(l, r int, b byte) int {
	len := r - l
	if len < 13 {
		for i := 0; i < len; i++ {
			if t.nodes[l+i] == b {
				return i
			}
		}
	} else {
		x, y := 0, len-1
		for x <= y {
			k := (x + y) >> 1
			if t.nodes[l+k] == b {
				return k
			}
			if t.nodes[l+k] > b {
				y = k - 1
			} else {
				x = k + 1
			}
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

func (b bitset) getBit(pos int) bool {
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

func (b bitset) rank(pos int) int {
	if pos>>6 >= len(b.bits) {
		return int(b.ranks[len(b.ranks)-1])
	}

	return int(b.ranks[pos>>6]) + bits.OnesCount64(b.bits[pos>>6]&(uint64(1)<<(pos&63+1)-1))
}

func (b bitset) selects(pos int) int {
	l, r := 0, len(b.bits)<<6-1
	for l < r {
		mid := (l + r) >> 1
		rank := int(b.ranks[mid>>6]) + bits.OnesCount64(b.bits[mid>>6]&(uint64(1)<<(mid&63+1)-1))
		if rank < pos {
			l = mid + 1
		} else {
			r = mid
		}
	}

	return l
}

type queue[T any] struct {
	data  []T
	l, sz uint32
}

func (q *queue[T]) push(elm T) {
	if int(q.sz) >= len(q.data) {
		var newSize uint32
		if q.sz > 1024 {
			newSize = q.sz + q.sz/4
		} else {
			newSize = (q.sz + 1) * 2
		}

		newData := make([]T, newSize)
		copy(newData[copy(newData, q.data[q.l:]):], q.data[:q.l])
		q.data = newData
		q.l = 0
	}

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
