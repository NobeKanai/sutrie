# Succinct Trie

A memory-efficient trie for testing the existence/prefixes of string only(for now).



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

You can customize trie's traversal process in a depth-first search manner. For example, define a domain name lookup rule: `*.example.com` matches `www.example.com` and `xxx.example.com` but not `xxx.www.example.com` and `example.com`

```go
// First build a trie with reversed domains because we usually match domains backwards
domains := []string{"*.example.com", "google.*"}

reversed := make([]string, len(domains))
for i, p := range domains {
    bytes := []byte(p)
    for i2, j := 0, len(bytes)-1; i2 < j; i2, j = i2+1, j-1 {
        bytes[i2], bytes[j] = bytes[j], bytes[i2]
    }
    reversed[i] = string(bytes)
}

trie := sutrie.BuildSuccinctTrie(reversed)

// Then let's define the matching method
search := func(domain string) bool {
    i := len(domain) - 1
    matched := false
    trie.Search(func(children []byte, isLeaf bool, next func(int)) {
        // Define ending condition
        if i < 0 {
            matched = isLeaf
            return
        }

        for k, c := range children {
            if c == '*' {
                tmp := i
                for i >= 0 && domain[i] != '.' {
                    i--
                }

                next(k)
                if matched {
                    return
                }

                i = tmp
            } else if c == domain[i] {
                i--

                next(k)
                if matched {
                    return
                }

                i++
            }
        }
    })

    return matched
}

println(search("www.example.com")) // true
println(search("xxx.example.com")) // true
println(search("xxx.www.example.com")) // false
println(search("example.com")) // false
println(search("google.io")) // true
```






