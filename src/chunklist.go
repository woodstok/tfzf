package fzf

import (
	"sync"

	quetty "github.com/woodstok/quetty/src"
)

// Chunk is a list of Items whose size has the upper limit of chunkSize
type Chunk struct {
	items [chunkSize]Item
	count int
}

// ItemBuilder is a closure type that builds Item object from byte array
type ItemBuilder func(*Item, []byte) bool

// ChunkList is a list of Chunks
type ChunkList struct {
	chunks    []*Chunk
	mutex     sync.Mutex
	trans     ItemBuilder
	tokenize  bool
	tokenType string
}

// NewChunkList returns a new ChunkList
func NewChunkList(trans ItemBuilder) *ChunkList {
	return &ChunkList{
		chunks: []*Chunk{},
		mutex:  sync.Mutex{},
		trans:  trans}
}

func (c *Chunk) push(trans ItemBuilder, data []byte) bool {
	if trans(&c.items[c.count], data) {
		c.count++
		return true
	}
	return false
}

// IsFull returns true if the Chunk is full
func (c *Chunk) IsFull() bool {
	return c.count == chunkSize
}

func (cl *ChunkList) lastChunk() *Chunk {
	return cl.chunks[len(cl.chunks)-1]
}

// CountItems returns the total number of Items
func CountItems(cs []*Chunk) int {
	if len(cs) == 0 {
		return 0
	}
	return chunkSize*(len(cs)-1) + cs[len(cs)-1].count
}

// Push adds the item to the list
func (cl *ChunkList) Push(data []byte) bool {
	cl.mutex.Lock()

	if len(cl.chunks) == 0 || cl.lastChunk().IsFull() {
		cl.chunks = append(cl.chunks, &Chunk{})
	}

	ret := cl.lastChunk().push(cl.trans, data)
	cl.mutex.Unlock()
	return ret
}

// Clear clears the data
func (cl *ChunkList) Clear() {
	cl.mutex.Lock()
	cl.chunks = nil
	cl.mutex.Unlock()
}

func (cl *ChunkList) Snapshot() ([]*Chunk, int) {
	if cl.tokenize {
		return cl.tokenizedSnapshot()
	} else {
		return cl.normalSnapshot()
	}
}

// Snapshot returns immutable snapshot of the ChunkList
func (cl *ChunkList) normalSnapshot() ([]*Chunk, int) {
	cl.mutex.Lock()

	ret := make([]*Chunk, len(cl.chunks))
	copy(ret, cl.chunks)

	// Duplicate the last chunk
	if cnt := len(ret); cnt > 0 {
		newChunk := *ret[cnt-1]
		ret[cnt-1] = &newChunk
	}

	cl.mutex.Unlock()
	return ret, CountItems(ret)
}

func (cl *ChunkList) getTokenizer() quetty.Tokenizer {
	switch cl.tokenType {
	case "ip":
		return &quetty.IpTokenizer{}
	case "path":
		return &quetty.PathTokenizer{}
	case "num":
		return quetty.NewRegexTokenizer(`\d{5,}`)
	case "hash":
		return quetty.NewRegexTokenizer(quetty.HASHREGEX)
	case "word":
		return quetty.NewRegexTokenizer(`\w{5,}`)
	default:
		panic("unknown token type")
	}
}

// Tokenized Snapshot returns immutable tokenized snapshot of the ChunkList
func (cl *ChunkList) tokenizedSnapshot() ([]*Chunk, int) {
	cl.mutex.Lock()

	tokenizedChunkList := NewChunkList(cl.trans)
	uniqTokens := quetty.NewTokens(nil)
	for _, chunk := range cl.chunks {
		for _, item := range chunk.items {
			tokens, err := quetty.Tokenize(item.AsString(true), cl.getTokenizer())
			if err != nil {
				continue
			}
			uniqTokens.Extend(tokens)
		}
	}
	for token, _ := range uniqTokens {
		tokenizedChunkList.Push([]byte(token))
	}

	cl.mutex.Unlock()
	return tokenizedChunkList.normalSnapshot()
}

func (cl *ChunkList) SetTokenize(val bool) {
	cl.mutex.Lock()
	cl.tokenize = val
	cl.mutex.Unlock()
}

// if same tokentype, toggle, otherwise, set to true
func (cl *ChunkList) ToggleTokenize(tokenType string) {
	cl.mutex.Lock()
	if cl.tokenType == tokenType {
		cl.tokenize = !cl.tokenize
	} else {
		cl.tokenize = true
	}
	cl.tokenType = tokenType
	cl.mutex.Unlock()
}
