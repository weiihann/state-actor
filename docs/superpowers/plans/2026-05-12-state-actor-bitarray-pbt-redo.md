# state-actor bitarray + PBT redo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce two state-actor branches off latest `nerolation/main` that pair correctly with geth `feat/binary-trie/bitarray` (bitarray-only) and geth `binary/pbt` (bitarray + PBT), each verified by `groundtruth_test.go` and the `integration-test/run.sh` at `TARGET_SIZE=100MB`.

**Architecture:** Linear branching — `bench/bitarray-pbt` is one commit on top of `bench/bitarray-base`, which is itself one bitarray-adaptation commit on top of `nerolation/main`. Each branch is verified independently before moving on; the linear structure isolates the PBT-specific diff to a single commit. The local `replace github.com/ethereum/go-ethereum => ../go-ethereum` in state-actor's `go.mod` means we switch geth branches in-place between verification rounds and rebuild.

**Tech Stack:** Go 1.24, `pebble` KV store, `bintrie` package in geth (`trie/bintrie`), `bintrie-benchmarks/integration-test/run.sh` (bash + spamoor for ERC20 deploys).

**File inventory:**
- `state-actor/docs/superpowers/specs/2026-05-12-state-actor-bitarray-pbt-redo.md` — spec, committed in Task 1.
- `state-actor/docs/superpowers/plans/2026-05-12-state-actor-bitarray-pbt-redo.md` — this file, committed in Task 1.
- `state-actor/generator/binary_stack_trie.go` — `makePath` rewrite in Task 3; PBT key-helper swap in Task 11.
- `state-actor/generator/regroup.go` — additional path-encoding sites in Task 4.
- `state-actor/generator/groundtruth_test.go` — restored from working tree in Task 5.
- `state-actor/generator/genesis_integration_test.go` — PBT key-helper swap in Task 12.
- `bintrie-benchmarks/integration-test/run.sh` — `--override.ubt=0` patch committed in Task 7.

**Glossary** (used throughout):
- "bitarray format" = `BitArray.KeyBytes()` output: packed active bytes (`ceil(bits/8)`) followed by a single bit-length byte. Empty path = empty bytes.
- "1-byte-per-bit format" = pre-bitarray on-disk format: one byte per bit, byte value `0x00` or `0x01`.
- "Round 1" = bench/bitarray-base + geth feat/binary-trie/bitarray.
- "Round 2" = bench/bitarray-pbt + geth binary/pbt.

---

### Task 1: Clean working tree, delete stale branches, create fresh `bench/bitarray-base`

**Files:**
- Discard: `state-actor/go.mod`, `state-actor/go.sum` (revert working-tree modifications from prior session).
- Create branches: `state-actor:bench/bitarray-base` off `nerolation/main`.
- Delete: `state-actor:bench/bitarray-base` (old), `state-actor:bench/bitarray-pbt` (current HEAD).
- Commit: `state-actor/docs/superpowers/specs/2026-05-12-state-actor-bitarray-pbt-redo.md` and `state-actor/docs/superpowers/plans/2026-05-12-state-actor-bitarray-pbt-redo.md`.

- [ ] **Step 1: Snapshot uncommitted test files outside the repo so they survive branch deletion**

```bash
mkdir -p /tmp/state-actor-snapshot
cp /Users/han/Documents/Codes/state-actor/generator/groundtruth_test.go /tmp/state-actor-snapshot/
cp /Users/han/Documents/Codes/state-actor/generator/single_stem_test.go /tmp/state-actor-snapshot/
ls -la /tmp/state-actor-snapshot/
```

Expected: both files listed.

- [ ] **Step 2: Discard go.mod/go.sum modifications**

```bash
cd /Users/han/Documents/Codes/state-actor
git checkout -- go.mod go.sum
git status --short
```

Expected: no `M go.mod` or `M go.sum` lines. Still shows untracked `docs/superpowers/`, `generator/groundtruth_test.go`, `generator/single_stem_test.go`.

- [ ] **Step 3: Move untracked test files out of the working tree (we'll re-add later)**

```bash
mv /Users/han/Documents/Codes/state-actor/generator/groundtruth_test.go /tmp/state-actor-snapshot/groundtruth_test.go.staged
mv /Users/han/Documents/Codes/state-actor/generator/single_stem_test.go /tmp/state-actor-snapshot/single_stem_test.go.staged
mv /Users/han/Documents/Codes/state-actor/docs/superpowers /tmp/state-actor-snapshot/docs-superpowers
cd /Users/han/Documents/Codes/state-actor
git status --short
```

Expected: clean working tree (nothing reported).

- [ ] **Step 4: Fetch upstream and check current branch isn't bench/***

```bash
cd /Users/han/Documents/Codes/state-actor
git fetch nerolation
git branch --show-current
```

Expected: current branch is `bench/bitarray-pbt` (or whichever). We'll need to switch away before deleting.

- [ ] **Step 5: Create the new bench/bitarray-base branch off nerolation/main**

```bash
cd /Users/han/Documents/Codes/state-actor
git checkout -B bench/bitarray-base nerolation/main
git log --oneline -3
```

Expected: HEAD is at `4c383fc` (`fix: cascade canonical recursive hashes + drop ghost re-flushes`) or later.

- [ ] **Step 6: Delete the old bench branches**

```bash
cd /Users/han/Documents/Codes/state-actor
git branch -D bench/bitarray-pbt
# bench/bitarray-base was overwritten by -B in step 5, so no separate delete needed.
git branch | grep bench
```

Expected: only `bench/bitarray-base` (current) and `bench/mpt-compact` (unrelated, leave alone).

- [ ] **Step 7: Restore docs and snapshot test files into the new branch's working tree**

```bash
mkdir -p /Users/han/Documents/Codes/state-actor/docs/superpowers
mv /tmp/state-actor-snapshot/docs-superpowers/* /Users/han/Documents/Codes/state-actor/docs/superpowers/
mv /tmp/state-actor-snapshot/groundtruth_test.go.staged /Users/han/Documents/Codes/state-actor/generator/groundtruth_test.go
mv /tmp/state-actor-snapshot/single_stem_test.go.staged /Users/han/Documents/Codes/state-actor/generator/single_stem_test.go
cd /Users/han/Documents/Codes/state-actor
git status --short
```

Expected: untracked entries for `docs/superpowers/`, `generator/groundtruth_test.go`, `generator/single_stem_test.go`.

- [ ] **Step 8: Commit the spec and plan (docs only, not the test files yet)**

```bash
cd /Users/han/Documents/Codes/state-actor
git add docs/superpowers/specs/2026-05-12-state-actor-bitarray-pbt-redo.md \
        docs/superpowers/plans/2026-05-12-state-actor-bitarray-pbt-redo.md
git commit -m "docs: spec + plan for state-actor bitarray+PBT redo

Adds the design spec and bite-sized implementation plan for the
bitarray + PBT state-actor adaptation, paired with the corresponding
geth branches and verified by groundtruth_test.go and the
integration-test harness at 100MB scale."
git log --oneline -2
```

Expected: new commit at HEAD, parent is the `nerolation/main` tip.

---

### Task 2: Inspect path-encoding call sites before changing anything

**Files:**
- Read: `state-actor/generator/binary_stack_trie.go`, `state-actor/generator/regroup.go`.

- [ ] **Step 1: Locate every path-construction site in the generator package**

```bash
cd /Users/han/Documents/Codes/state-actor
rg -n 'stemBitAt|makePath\(|make\(\[\]byte,\s*(depth|boundary)' generator/ | head -40
```

Expected: at least these hits — `makePath` definition in `binary_stack_trie.go`, plus call sites in the same file and likely in `regroup.go`.

- [ ] **Step 2: Read makePath and its first caller (writeGroupedNode) for context**

Read `state-actor/generator/binary_stack_trie.go` around `makePath` and around `writeGroupedNode`. Note the exact line numbers and which functions call `makePath`. Record this list in a short scratchpad so Task 3 and Task 4 know what to update.

Specifically write down:
- Line number of `makePath` definition.
- Line numbers of every `makePath(stem, X)` call.
- Line numbers of any *inline* path-encoding (not using `makePath`) — these are the highest-risk sites because they may have been added later and forgotten.

No commit yet — this task produces information, not changes.

---

### Task 3: Rewrite `makePath` to emit bitarray format

**Files:**
- Modify: `state-actor/generator/binary_stack_trie.go` (`makePath` function).
- Test: `state-actor/generator/groundtruth_test.go` (currently in working tree, will be added in Task 5).

- [ ] **Step 1: Read the current `makePath` body and the `bintrie` import**

```bash
cd /Users/han/Documents/Codes/state-actor
rg -n -A 10 '^func makePath' generator/binary_stack_trie.go
rg -n 'trie/bintrie' generator/binary_stack_trie.go
```

Expected: current `makePath` returns 1-byte-per-bit. The `bintrie` import is already present.

- [ ] **Step 2: Replace `makePath` with the bitarray implementation**

In `state-actor/generator/binary_stack_trie.go`, replace the body of `makePath` so the function becomes:

```go
func makePath(stem []byte, depth int) []byte {
    if depth <= 0 {
        return nil
    }
    var ba bintrie.BitArray
    ba.SetBytes(uint8(len(stem)*8), stem)
    var path bintrie.BitArray
    path.MSBs(&ba, uint8(depth))
    return path.KeyBytes()
}
```

Leave the surrounding `stemBitAt` helper alone — it's still used for slot computation in other call sites (not for on-disk paths).

- [ ] **Step 3: Verify the file compiles**

```bash
cd /Users/han/Documents/Codes/state-actor
go build ./generator/...
```

Expected: clean build. If a `BitArray.KeyBytes` or `BitArray.SetBytes` signature has shifted on the linked geth, fix the call by reading the current signature in `../go-ethereum/trie/bintrie/bitarray.go`.

- [ ] **Step 4: Confirm `../go-ethereum` is on `feat/binary-trie/bitarray` for this build**

```bash
cd /Users/han/Documents/Codes/go-ethereum
git branch --show-current
```

Expected: `feat/binary-trie/bitarray`. If not, `git checkout feat/binary-trie/bitarray` then re-run Step 3.

- [ ] **Step 5: Commit the makePath change**

```bash
cd /Users/han/Documents/Codes/state-actor
git add generator/binary_stack_trie.go
git commit -m "generator: emit bitarray-format paths in makePath

Switches the on-disk path encoding from 1-byte-per-bit to
BitArray.KeyBytes() (packed bytes + postpend bit-length byte) so
that paths written by state-actor match what geth's bintrie reads
since the postpend-bit-length disambiguation landed."
```

---

### Task 4: Update any other path-encoding sites in regroup.go

**Files:**
- Modify: `state-actor/generator/regroup.go` (if it has inline path-as-bytes constructions).

- [ ] **Step 1: Grep for inline path constructions in regroup.go**

```bash
cd /Users/han/Documents/Codes/state-actor
rg -n 'stemBitAt|make\(\[\]byte,\s*(depth|boundary|bits)' generator/regroup.go
rg -n 'ActiveBytes|KeyBytes' generator/regroup.go
```

Expected: zero or more hits. Record what you find.

- [ ] **Step 2: For each hit, decide if it constructs a path-as-bytes used for on-disk lookup**

Two categories of hit:
- **Path-as-bytes used to look up trie nodes on disk (`vA<path>` keys or map keys mirroring those):** must produce bitarray-format. Replace with `makePath(stem, depth)` if the inputs match, or with an equivalent `BitArray.MSBs(...).KeyBytes()` call.
- **Slot computation (`stemBitAt` used to compute a slot index, not a path):** leave alone. These compute integer slot indices, not on-disk keys.

For each path-as-bytes hit, edit `regroup.go` to use `makePath`. If there's no `stem` variable in scope, you'll need a different `BitArray.MSBs`-based form — read the surrounding context to choose.

If there are **zero** path-as-bytes hits, skip to Step 4.

- [ ] **Step 3: Verify the file compiles**

```bash
cd /Users/han/Documents/Codes/state-actor
go build ./generator/...
```

Expected: clean build.

- [ ] **Step 4: Commit if any changes were made**

```bash
cd /Users/han/Documents/Codes/state-actor
if [ -n "$(git status --short generator/regroup.go)" ]; then
  git add generator/regroup.go
  git commit -m "generator: bitarray-format paths in regroup.go too

The regroup helper constructs path-as-bytes used for on-disk key
matching; same encoding rule as makePath applies."
else
  echo "No regroup.go changes — skipping commit."
fi
```

---

### Task 5: Add the groundtruth reproducer test

**Files:**
- Create: `state-actor/generator/groundtruth_test.go` (restore from working tree; file already in place after Task 1 Step 7).

- [ ] **Step 1: Confirm the file is present and compiles**

```bash
cd /Users/han/Documents/Codes/state-actor
ls -la generator/groundtruth_test.go
go vet ./generator/...
```

Expected: file present (5994 bytes). `go vet` clean.

- [ ] **Step 2: Try compiling the test file specifically**

```bash
cd /Users/han/Documents/Codes/state-actor
go test -c -o /tmp/generator-tests ./generator/
```

Expected: a test binary at `/tmp/generator-tests`. If it fails, fix any API drift in `groundtruth_test.go` (the test may reference geth APIs that shifted; reads `bintrie.NewBinaryTrie`, `UpdateStem`, `Commit`, `trienode.Node` — check against the current `feat/binary-trie/bitarray` geth).

- [ ] **Step 3: Commit the test file**

```bash
cd /Users/han/Documents/Codes/state-actor
git add generator/groundtruth_test.go
git commit -m "generator: add groundtruth test comparing streamingBuilder to geth Commit

Feeds the same stem set into state-actor's streamingBuilder and into
geth's BinaryTrie.UpdateStem + Commit, then diffs both DBs. Runs at
n=2,4,8,32,128. High-signal correctness gate before integration test."
```

- [ ] **Step 4: Decide what to do with single_stem_test.go**

```bash
cd /Users/han/Documents/Codes/state-actor
cat generator/single_stem_test.go
```

Read the contents. If it's just a small focused reproducer that complements `groundtruth_test.go`, commit it too:

```bash
git add generator/single_stem_test.go
git commit -m "generator: add single-stem reproducer for narrowing groundtruth failures"
```

If it's redundant with `groundtruth_test.go`, delete it:

```bash
rm generator/single_stem_test.go
```

---

### Task 6: Run the groundtruth test on `bench/bitarray-base`

**Files:**
- Run: `state-actor/generator/groundtruth_test.go::TestStreamingMatchesGethCommit`.

- [ ] **Step 1: Confirm geth is on `feat/binary-trie/bitarray`**

```bash
cd /Users/han/Documents/Codes/go-ethereum
git branch --show-current
```

Expected: `feat/binary-trie/bitarray`.

- [ ] **Step 2: Run the groundtruth test**

```bash
cd /Users/han/Documents/Codes/state-actor
go test ./generator/ -run TestStreamingMatchesGethCommit -v
```

Expected: PASS for `n=2`, `n=4`, `n=8`, `n=32`, `n=128`.

If FAIL: this means the streaming builder bug we documented last session is **not** fully fixed by upstream's recent commits. Stop and debug — capture the smallest failing `n`, dump the diff at the trie-node level, and route into systematic debugging. Do **not** proceed to integration test.

- [ ] **Step 3: Run the rest of the generator unit tests**

```bash
cd /Users/han/Documents/Codes/state-actor
go test ./generator/...
```

Expected: all PASS. If any golden state-root constants in `generator_test.go` fail, investigate — the path encoding change should not affect state-root values, so a shift indicates a real bug.

---

### Task 7: Commit bintrie-benchmarks `--override.ubt` fix

**Files:**
- Modify: `bintrie-benchmarks/integration-test/run.sh` (already has working-tree change).

- [ ] **Step 1: Check the existing working-tree diff**

```bash
cd /Users/han/Documents/Codes/bintrie-benchmarks
git diff integration-test/run.sh
```

Expected: shows `--override.verkle=0` replaced by `--override.ubt=0` on one line.

- [ ] **Step 2: Confirm we're on the `pbt` branch**

```bash
cd /Users/han/Documents/Codes/bintrie-benchmarks
git branch --show-current
```

Expected: `pbt`.

- [ ] **Step 3: Commit**

```bash
cd /Users/han/Documents/Codes/bintrie-benchmarks
git add integration-test/run.sh
git commit -m "integration-test: rename --override.verkle to --override.ubt

The geth flag was renamed when binary-trie work consolidated under
the 'ubt' (Unified Binary Trie) name. Old --override.verkle no longer
parses, causing geth to refuse to boot. Same fix applies to both
the bitarray-only and bitarray+PBT scenarios."
```

---

### Task 8: Build round-1 binaries (geth + state-actor) and stash

**Files:**
- Build: `go-ethereum/build/bin/geth` (on `feat/binary-trie/bitarray`).
- Build: `state-actor/state-actor` (on `bench/bitarray-base`).
- Stash: `/tmp/bench-bins/{geth,state-actor}-base`.

- [ ] **Step 1: Ensure both repos are on the right branches**

```bash
cd /Users/han/Documents/Codes/go-ethereum && git branch --show-current
cd /Users/han/Documents/Codes/state-actor && git branch --show-current
```

Expected: `feat/binary-trie/bitarray` and `bench/bitarray-base` respectively.

- [ ] **Step 2: Build geth**

```bash
cd /Users/han/Documents/Codes/go-ethereum
make geth
ls -la build/bin/geth
```

Expected: built binary, size around 49 MB.

- [ ] **Step 3: Build state-actor**

```bash
cd /Users/han/Documents/Codes/state-actor
go build -o state-actor .
ls -la state-actor
```

Expected: built binary, size around 29 MB.

- [ ] **Step 4: Stash binaries**

```bash
mkdir -p /tmp/bench-bins
cp /Users/han/Documents/Codes/go-ethereum/build/bin/geth /tmp/bench-bins/geth-base
cp /Users/han/Documents/Codes/state-actor/state-actor /tmp/bench-bins/state-actor-base
ls -la /tmp/bench-bins/
```

Expected: both stashed binaries present.

---

### Task 9: Run integration test for round 1

**Files:**
- Run: `bintrie-benchmarks/integration-test/run.sh` at `TARGET_SIZE=100MB`.
- Log: `/tmp/bench-results/run-base-small.log`.

- [ ] **Step 1: Prepare results directory**

```bash
mkdir -p /tmp/bench-results
```

- [ ] **Step 2: Run the integration test**

```bash
cd /Users/han/Documents/Codes/bintrie-benchmarks/integration-test
GETH_BIN=/tmp/bench-bins/geth-base \
STATE_ACTOR_BIN=/tmp/bench-bins/state-actor-base \
TARGET_SIZE=100MB \
GROUP_DEPTH=8 \
bash run.sh --keep 2>&1 | tee /tmp/bench-results/run-base-small.log
```

Expected: all three stages pass (state-gen → ERC20 deploy → restart-validate). The last line should be `Test directory preserved (--keep): /tmp/bintrie-integration-XXXXXX` (or equivalent).

If FAIL with "missing trie node": last-session's bug is still present. Capture the test directory path, inspect with `/tmp/bench-debug/inspect` if it still exists, otherwise rebuild it from the source under `/tmp/bench-debug/`. Route into systematic debugging — do not proceed.

- [ ] **Step 3: Record the kept test directory path for the comparison phase**

```bash
TEST_DIR=$(grep -o '/tmp/bintrie-integration-[A-Za-z0-9]*' /tmp/bench-results/run-base-small.log | tail -1)
echo "Round 1 test directory: $TEST_DIR"
ls "$TEST_DIR" | head -10
```

Expected: directory contains `state-actor.log`, `geth-deploy.log`, `geth-restart.log`, `spamoor.log`, plus the generated DB.

- [ ] **Step 4: Sanity-check geth-deploy.log has clean block insertions**

```bash
grep -E 'inserting block failed|missing trie node' "$TEST_DIR/geth-deploy.log" | head -5 || echo "no failures"
```

Expected: `no failures`.

---

### Task 10: Create `bench/bitarray-pbt` branch off `bench/bitarray-base`

**Files:**
- Create branch: `state-actor:bench/bitarray-pbt`.

- [ ] **Step 1: Branch off the current state of bench/bitarray-base**

```bash
cd /Users/han/Documents/Codes/state-actor
git checkout -b bench/bitarray-pbt bench/bitarray-base
git log --oneline -3
```

Expected: HEAD is the same as `bench/bitarray-base`. New branch created.

- [ ] **Step 2: Switch geth to `binary/pbt`**

```bash
cd /Users/han/Documents/Codes/go-ethereum
git checkout binary/pbt
git branch --show-current
```

Expected: `binary/pbt`.

- [ ] **Step 3: Confirm state-actor no longer compiles against PBT geth (this validates the key-API gap)**

```bash
cd /Users/han/Documents/Codes/state-actor
go build ./generator/... 2>&1 | head -20
```

Expected: compile errors like `undefined: bintrie.GetBinaryTreeKey`. This proves we need to swap to zoned helpers in the next task.

If there are **no** errors (state-actor compiles cleanly): that means either (a) the upstream PBT geth still exposes `GetBinaryTreeKey` (didn't remove it), or (b) state-actor's calls happen to be in code paths that aren't compiled. Read the next task's pre-swap inspection step carefully before editing.

---

### Task 11: Swap `GetBinaryTreeKey` calls in binary_stack_trie.go to zoned helpers

**Files:**
- Modify: `state-actor/generator/binary_stack_trie.go` (around lines 322 and 370).

- [ ] **Step 1: Read both call sites for context**

```bash
cd /Users/han/Documents/Codes/state-actor
rg -n -B 5 -A 5 'bintrie\.GetBinaryTreeKey\(addr,' generator/binary_stack_trie.go
```

Expected: two hits. For each, note the second argument and the surrounding intent (is it building a stem for account basic data, code chunks, etc.).

- [ ] **Step 2: Read the available zoned helpers on the current geth**

```bash
cd /Users/han/Documents/Codes/go-ethereum
rg -n '^func GetBinaryTreeKey' trie/bintrie/*.go
```

Expected: helpers like `GetBinaryTreeKeyBasicData(addr)`, `GetBinaryTreeKeyCodeHash(addr)`, `GetBinaryTreeKeyCodeChunk(address, chunknr)`, `GetBinaryTreeKeyStorageSlot(address, slotnum)`.

- [ ] **Step 3: Map each call site to the correct zoned helper**

For each `bintrie.GetBinaryTreeKey(addr, X)` site, the second argument `X` is a treeIndex/key. Map it:
- If `X == zeroKey[:]` (32 zero bytes): the site is building the account's basic-data stem. Replace with `bintrie.GetBinaryTreeKeyBasicData(addr)`.
- If `X == offset[:]` where `offset` is a code-chunk index (often `uint256.Int.Bytes32()` form): replace with `bintrie.GetBinaryTreeKeyCodeChunk(addr, &offset)` — read the precise zoned helper signature on the current geth before swapping; `chunknr` may need a `*uint256.Int` or a raw `[]byte`.
- Any other `X`: read the call site's intent and pick the matching zoned helper. If unclear, dump the call site to the log and ask the user before swapping.

- [ ] **Step 4: Apply the swap**

Edit `state-actor/generator/binary_stack_trie.go` to replace each call. Example for the basic-data site:

```go
// Before:
stem := bintrie.GetBinaryTreeKey(addr, zeroKey[:])
// After:
stem := bintrie.GetBinaryTreeKeyBasicData(addr)
```

Example for the code-chunk site (signature depends on what current geth exposes; verify):

```go
// Before:
stem = bintrie.GetBinaryTreeKey(addr, offset[:])
// After:
stem = bintrie.GetBinaryTreeKeyCodeChunk(addr, &chunknr)
```

- [ ] **Step 5: Verify the file compiles**

```bash
cd /Users/han/Documents/Codes/state-actor
go build ./generator/...
```

Expected: clean build.

If build fails on signature mismatch (e.g., `CodeChunk` wants `*uint256.Int` but we passed `[]byte`), re-read the signature on geth's `binary/pbt` branch and adjust the call accordingly.

- [ ] **Step 6: Commit the swap**

```bash
cd /Users/han/Documents/Codes/state-actor
git add generator/binary_stack_trie.go
git commit -m "generator: swap GetBinaryTreeKey calls to PBT zoned helpers

The unified bintrie.GetBinaryTreeKey API is absent on the binary/pbt
geth branch — PBT zone partitioning splits stem derivation into
type-specific helpers. Swap account-basic-data and code-chunk sites
to GetBinaryTreeKeyBasicData and GetBinaryTreeKeyCodeChunk
respectively."
```

---

### Task 12: Same swap in genesis_integration_test.go

**Files:**
- Modify: `state-actor/generator/genesis_integration_test.go`.

- [ ] **Step 1: Grep for the call**

```bash
cd /Users/han/Documents/Codes/state-actor
rg -n 'bintrie\.GetBinaryTreeKey\(' generator/genesis_integration_test.go
```

Expected: one or more hits.

- [ ] **Step 2: Apply the same mapping as Task 11 Step 3**

Edit `genesis_integration_test.go` to swap each `bintrie.GetBinaryTreeKey(addr, X)` to the appropriate zoned helper based on the second argument.

- [ ] **Step 3: Verify compile**

```bash
cd /Users/han/Documents/Codes/state-actor
go build ./generator/...
go vet ./generator/...
```

Expected: clean.

- [ ] **Step 4: Commit**

```bash
cd /Users/han/Documents/Codes/state-actor
git add generator/genesis_integration_test.go
git commit -m "generator: PBT zoned helpers in genesis_integration_test.go too

Mirrors the swap in binary_stack_trie.go for the test-side call site."
```

---

### Task 13: Run groundtruth + unit tests on bench/bitarray-pbt

**Files:**
- Run: `state-actor/generator/groundtruth_test.go::TestStreamingMatchesGethCommit`.

- [ ] **Step 1: Confirm geth is on `binary/pbt`**

```bash
cd /Users/han/Documents/Codes/go-ethereum
git branch --show-current
```

Expected: `binary/pbt`.

- [ ] **Step 2: Run the groundtruth test**

```bash
cd /Users/han/Documents/Codes/state-actor
go test ./generator/ -run TestStreamingMatchesGethCommit -v
```

Expected: PASS at all sizes.

If FAIL: the regression is in the single PBT-swap commit (since Round 1 passed). Diff the test outputs against Round 1's pass to narrow the bug. Likely root causes: a wrong zoned-helper mapping in Task 11 Step 3, or a signature mismatch that compiled but produces wrong bytes (e.g., passing the wrong indexable form to `CodeChunk`).

- [ ] **Step 3: Run remaining unit tests**

```bash
cd /Users/han/Documents/Codes/state-actor
go test ./generator/...
```

Expected: all PASS. The golden state-root constants in `generator_test.go` **will likely differ** under PBT because zone partitioning changes the stem-derivation hash. If a golden root fails, this is expected for the PBT case — confirm by inspecting which test failed and whether it computes a deterministic root from a known input. If yes, update the golden constant **on this branch only** (not on `bench/bitarray-base`) and document in the commit message.

---

### Task 14: Build round-2 binaries and stash

**Files:**
- Build: `go-ethereum/build/bin/geth` (on `binary/pbt`).
- Build: `state-actor/state-actor` (on `bench/bitarray-pbt`).
- Stash: `/tmp/bench-bins/{geth,state-actor}-pbt`.

- [ ] **Step 1: Confirm branches**

```bash
cd /Users/han/Documents/Codes/go-ethereum && git branch --show-current
cd /Users/han/Documents/Codes/state-actor && git branch --show-current
```

Expected: `binary/pbt` and `bench/bitarray-pbt`.

- [ ] **Step 2: Build geth and state-actor**

```bash
cd /Users/han/Documents/Codes/go-ethereum && make geth
cd /Users/han/Documents/Codes/state-actor && go build -o state-actor .
```

Expected: both clean.

- [ ] **Step 3: Stash binaries**

```bash
cp /Users/han/Documents/Codes/go-ethereum/build/bin/geth /tmp/bench-bins/geth-pbt
cp /Users/han/Documents/Codes/state-actor/state-actor /tmp/bench-bins/state-actor-pbt
ls -la /tmp/bench-bins/
```

Expected: all four stashed binaries present (`geth-base`, `geth-pbt`, `state-actor-base`, `state-actor-pbt`).

---

### Task 15: Run integration test for round 2

**Files:**
- Run: `bintrie-benchmarks/integration-test/run.sh` at `TARGET_SIZE=100MB`.
- Log: `/tmp/bench-results/run-pbt-small.log`.

- [ ] **Step 1: Run the integration test**

```bash
cd /Users/han/Documents/Codes/bintrie-benchmarks/integration-test
GETH_BIN=/tmp/bench-bins/geth-pbt \
STATE_ACTOR_BIN=/tmp/bench-bins/state-actor-pbt \
TARGET_SIZE=100MB \
GROUP_DEPTH=8 \
bash run.sh --keep 2>&1 | tee /tmp/bench-results/run-pbt-small.log
```

Expected: all three stages pass.

If FAIL with "missing trie node": same diagnostic path as Round 1 — capture the test directory, inspect, and route into systematic debugging. The regression must lie in the single PBT-swap commit.

- [ ] **Step 2: Record the kept test directory**

```bash
TEST_DIR=$(grep -o '/tmp/bintrie-integration-[A-Za-z0-9]*' /tmp/bench-results/run-pbt-small.log | tail -1)
echo "Round 2 test directory: $TEST_DIR"
grep -E 'inserting block failed|missing trie node' "$TEST_DIR/geth-deploy.log" | head -5 || echo "no failures"
```

Expected: `no failures`.

---

### Task 16: Final verification summary

**Files:**
- Read-only: confirm both run logs.

- [ ] **Step 1: Verify both run logs reported success**

```bash
tail -20 /tmp/bench-results/run-base-small.log
tail -20 /tmp/bench-results/run-pbt-small.log
```

Expected: both end with success messages from `run.sh` (typically a "Tests completed" or kept-dir line).

- [ ] **Step 2: Verify the four-binary stash is intact**

```bash
ls -la /tmp/bench-bins/
```

Expected: `geth-base`, `geth-pbt`, `state-actor-base`, `state-actor-pbt`, all roughly 29-49 MB.

- [ ] **Step 3: Report**

Print a short summary:
- Round 1 (bitarray-only): groundtruth PASS, integration PASS at 100MB.
- Round 2 (bitarray + PBT): groundtruth PASS, integration PASS at 100MB.
- Both bench branches committed in `state-actor`, both kept test directories on disk for inspection.
- Follow-up (out of scope this iteration): `TARGET_SIZE=1GB` runs + CSV/median analysis.
