# Succinct Trie

A memory-efficient trie for testing the existence/prefixes of string only.



## Install

```bash
go get -u github.com/nobekanai/sutrie
```



## Documentation

A simple and common use case: querying whether the key appears in the dictionary or whether the prefix of the key is in the dictionary

```go
keys := []string{"hat", "is", "it", "a"}
trie := sutrie.BuildSuccinctTrie(keys)

lastUnmatch := trie.SearchPrefix("hatt")
println(lastUnmatch) // will print 3, same if you search "hat"

lastUnmatch = trie.SearchPrefix("ha")
println(lastUnmatch) // will print 0, because neither "ha" nor its prefix (that is "h") is in trie 
```



### Advanced Usage

You can customize trie's traversal process. For example, define a domain name lookup rule: `*.example.com` matches `www.example.com` and `xxx.example.com` but not `xxx.www.example.com` and `example.com`

```go
func reverse(s string) string {
        bytes := []byte(s)
        for i, j := 0, len(bytes)-1; i < j; i, j = i+1, j-1 {
                bytes[i], bytes[j] = bytes[j], bytes[i]
        }
        return string(bytes)
}

func main() {
        // First build a trie with reversed domains because we usually match domains backwards
        domains := []string{reverse("*.example.com"), reverse("google.*")}
        trie := sutrie.BuildSuccinctTrie(domains)

        // Then let's define the matching rule
        var search func(node sutrie.Node, domain string, idx int) bool
        search = func(node sutrie.Node, domain string, idx int) (matched bool) {
                if idx < 0 {
                        return node.Leaf
                }

                for k, c := range node.Children {
                        if c == domain[idx] {
                                matched = search(trie.Next(node, k), domain, idx-1)
                        } else if c == '*' {
                                tmp := idx
                                for tmp >= 0 && domain[tmp] != '.' {
                                        tmp--
                                }
                                matched = search(trie.Next(node, k), domain, tmp)
                        }

                        if matched {
                                return true
                        }
                }
                return
        }

        Search := func(domain string) bool {
                return search(trie.Root(), domain, len(domain)-1)
        }

        println(Search("www.example.com"))     // true
        println(Search("xxx.example.com"))     // true
        println(Search("xxx.www.example.com")) // false
        println(Search("example.com"))         // false
        println(Search("google.io"))           // true
}
```

