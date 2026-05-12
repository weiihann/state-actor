package generator

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/ethdb/memorydb"
)

func TestPrintSingleStemRoot(t *testing.T) {
	var entries []trieEntry
	var e trieEntry
	for i := 0; i < stemSize; i++ {
		e.Key[i] = byte(i * 7)
	}
	e.Key[stemSize] = 0
	e.Value = sha256.Sum256([]byte("test-value"))
	entries = append(entries, e)

	for _, gd := range []int{0, 1, 4, 5, 8} {
		t.Run(fmt.Sprintf("gd%d", gd), func(t *testing.T) {
			db := memorydb.New()
			root, _ := computeBinaryRootStreamingFromSlice(entries, db, gd)
			t.Logf("gd=%d root=%x", gd, root)
		})
	}
}
