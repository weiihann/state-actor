package generator

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb/memorydb"
	"github.com/ethereum/go-ethereum/trie/bintrie"
	"github.com/ethereum/go-ethereum/trie/trienode"
	"github.com/ethereum/go-ethereum/triedb"
)

// TestStreamingMatchesGethCommit feeds the same set of stems into state-actor's
// streamingBuilder and into geth's bintrie.BinaryTrie.Commit, then compares the
// resulting on-disk node sets. If the on-disk node sets diverge, we have a
// state-actor bug: same trie shape, same root hash, but missing or mismatched
// node writes.
func TestStreamingMatchesGethCommit(t *testing.T) {
	for _, n := range []int{2, 4, 8, 32, 128} {
		t.Run(fmt.Sprintf("n%d", n), func(t *testing.T) {
			entries := smallTestEntries(n)
			const groupDepth = 5

			// State-actor side: write via streamingBuilder.
			actorDB := memorydb.New()
			actorRoot, _ := computeBinaryRootStreamingFromSlice(entries, actorDB, groupDepth)

			// geth side: build a BinaryTrie, commit, and dump its NodeSet to memDB.
			gethDB := memorydb.New()
			gethRoot := buildGethTrieAndWrite(t, entries, gethDB, groupDepth)

			if actorRoot != gethRoot {
				t.Errorf("state-root mismatch:\n  state-actor: %s\n  geth:        %s",
					actorRoot.Hex(), gethRoot.Hex())
			}

			compareGroundTruth(t, actorDB, gethDB)
		})
	}
}

// buildGethTrieAndWrite uses geth's BinaryTrie to build the same trie from the
// same entries and writes its committed NodeSet to gethDB under the "vA"
// prefix that state-actor uses. Returns the root hash.
func buildGethTrieAndWrite(t *testing.T, entries []trieEntry, gethDB *memorydb.Database, groupDepth int) common.Hash {
	t.Helper()
	// Real triedb backed by an in-memory rawdb so NewBinaryTrie has a resolver.
	rawDB := rawdb.NewMemoryDatabase()
	tdb := triedb.NewDatabase(rawDB, triedb.UBTDefaults)
	tr, err := bintrie.NewBinaryTrie(types.EmptyBinaryHash, tdb, groupDepth)
	if err != nil {
		t.Fatalf("NewBinaryTrie: %v", err)
	}

	// Group entries by stem; build values arrays; call UpdateStem.
	var currentStem [stemSize]byte
	var group []trieEntry
	flush := func() {
		if len(group) == 0 {
			return
		}
		values := make([][]byte, bintrie.StemNodeWidth)
		for _, e := range group {
			suffix := e.Key[stemSize]
			v := append([]byte(nil), e.Value[:]...)
			values[suffix] = v
		}
		stemKey := append([]byte(nil), currentStem[:]...)
		stemKey = append(stemKey, 0) // UpdateStem expects 32-byte key (stem + suffix slot)
		if err := tr.UpdateStem(stemKey[:stemSize], values); err != nil {
			t.Fatalf("UpdateStem: %v", err)
		}
	}
	copy(currentStem[:], entries[0].Key[:stemSize])
	for _, e := range entries {
		if !bytes.Equal(e.Key[:stemSize], currentStem[:]) {
			flush()
			group = group[:0]
			copy(currentStem[:], e.Key[:stemSize])
		}
		group = append(group, e)
	}
	flush()

	root, nodeset := tr.Commit(false)
	if nodeset == nil {
		t.Fatal("Commit returned nil NodeSet")
	}
	nodeset.ForEachWithOrder(func(path string, n *trienode.Node) {
		key := append([]byte(verkleTrieNodeKeyPrefix), path...)
		if err := gethDB.Put(key, n.Blob); err != nil {
			t.Fatalf("write geth node: %v", err)
		}
	})
	return root
}

// compareGroundTruth surfaces differences between state-actor's and geth's
// on-disk node sets. Reports keys only in one side and any value mismatches.
func compareGroundTruth(t *testing.T, actorDB, gethDB *memorydb.Database) {
	t.Helper()
	prefix := verkleTrieNodeKeyPrefix
	actorKeys := keysWithPrefix(actorDB, prefix)
	gethKeys := keysWithPrefix(gethDB, prefix)

	actorSet := keySet(actorKeys)
	gethSet := keySet(gethKeys)

	var onlyActor, onlyGeth [][]byte
	for k := range actorSet {
		if _, ok := gethSet[k]; !ok {
			onlyActor = append(onlyActor, []byte(k))
		}
	}
	for k := range gethSet {
		if _, ok := actorSet[k]; !ok {
			onlyGeth = append(onlyGeth, []byte(k))
		}
	}
	sortBytes(onlyActor)
	sortBytes(onlyGeth)

	if len(onlyActor) > 0 {
		t.Errorf("nodes ONLY in state-actor (%d):", len(onlyActor))
		for _, k := range onlyActor {
			t.Errorf("  keyLen=%d %x", len(k)-len(prefix), k[len(prefix):])
		}
	}
	if len(onlyGeth) > 0 {
		t.Errorf("nodes ONLY in geth (%d) — STATE-ACTOR FORGOT TO WRITE THESE:", len(onlyGeth))
		for _, k := range onlyGeth {
			path := k[len(prefix):]
			geth, _ := gethDB.Get(k)
			t.Errorf("  keyLen=%d path=%x  expected_blob_first16=%x  blob_len=%d",
				len(path), path, geth[:min(16, len(geth))], len(geth))
		}
	}

	// For overlapping keys, compare blobs.
	for k := range actorSet {
		if _, ok := gethSet[k]; !ok {
			continue
		}
		key := []byte(k)
		a, _ := actorDB.Get(key)
		g, _ := gethDB.Get(key)
		if !bytes.Equal(a, g) {
			path := key[len(prefix):]
			t.Errorf("value mismatch at path=%x (keyLen=%d):\n  state-actor: %x\n  geth:        %x",
				path, len(path), a, g)
		}
	}
}

func smallTestEntries(n int) []trieEntry {
	var entries []trieEntry
	for i := 0; i < n; i++ {
		stemHash := sha256.Sum256([]byte{byte(i), byte(i >> 8)})
		var e trieEntry
		copy(e.Key[:stemSize], stemHash[:stemSize])
		e.Key[stemSize] = 0
		e.Value = sha256.Sum256([]byte{byte(i), 0xFF})
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool {
		return bytes.Compare(entries[i].Key[:], entries[j].Key[:]) < 0
	})
	return entries
}

func keysWithPrefix(db *memorydb.Database, prefix []byte) [][]byte {
	var keys [][]byte
	it := db.NewIterator(prefix, nil)
	defer it.Release()
	for it.Next() {
		if !bytes.HasPrefix(it.Key(), prefix) {
			break
		}
		k := make([]byte, len(it.Key()))
		copy(k, it.Key())
		keys = append(keys, k)
	}
	return keys
}

func keySet(keys [][]byte) map[string]struct{} {
	out := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		out[string(k)] = struct{}{}
	}
	return out
}

func sortBytes(s [][]byte) {
	sort.Slice(s, func(i, j int) bool { return bytes.Compare(s[i], s[j]) < 0 })
}
