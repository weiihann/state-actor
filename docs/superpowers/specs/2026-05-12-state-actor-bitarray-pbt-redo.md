# state-actor bitarray + PBT redo, with end-to-end integration verification

**Date:** 2026-05-12
**Status:** Spec, ready for implementation plan
**Owner:** weiihann

## Context

Two changes have landed in geth that the state-actor must adapt to:

1. **Bitarray path encoding** (geth commit `012bec0eb`, May 8): on-disk trie node paths use `BitArray.KeyBytes()` — packed active bytes followed by a single bit-length byte. The pre-bitarray format was 1 byte per bit.
2. **PBT zone-partitioned key derivation** (geth `binary/pbt` branch): the unified `bintrie.GetBinaryTreeKey(addr, treeIndex)` was replaced by zone-specific helpers (`GetBinaryTreeKeyBasicData`, `GetBinaryTreeKeyStorageSlot`, `GetBinaryTreeKeyCodeChunk`, …). The unified API is **absent** on `binary/pbt`.

`nerolation/main` of state-actor already uses the zoned helpers in most places (storage slots) but still calls the unified `GetBinaryTreeKey` for account/code stems. It still emits paths in the **old 1-byte-per-bit** format, so it cannot produce a DB that the current geth can read.

A prior attempt to land bitarray+PBT support (branches `bench/bitarray-base`, `bench/bitarray-pbt` against an older base) hit a deterministic "missing trie node" failure during the integration test's ERC20 deploy phase. Investigation traced it to multi-stem-slot hash divergence in the streaming builder. Since then, upstream `nerolation/main` has landed four targeted fixes:

- `cd5e42a fix: preserve all root-group slots in streaming builder grouped emission`
- `692a0a9 fix: dedup grouped slot recordings to prevent missing-trie-node`
- `4c383fc fix: cascade canonical recursive hashes + drop ghost re-flushes`
- `102267c generator: polish grouped-emission fix — comments, hot-path, broader tests`

Starting fresh from the current `nerolation/main` lets us pick up these fixes for free and focus the present work on bitarray + PBT adaptation only.

## Goals

- Two state-actor branches off latest `nerolation/main` that produce on-disk DBs readable by their corresponding geth branches.
- Both branches pass a unit-level state-root match against geth's `Commit` (groundtruth test) and the full `bintrie-benchmarks/integration-test/run.sh` at `TARGET_SIZE=100MB`.
- No performance comparison in this iteration — correctness only. A 1GB performance comparison is a follow-up.

## Non-goals

- CSV extraction or median-delta analysis.
- Merging the bench branches into anyone's main.
- Re-running with `TARGET_SIZE` larger than 100MB.

## Architecture

### Branch and pairing matrix

| Repo | Branch | Base | Pairs with |
|---|---|---|---|
| state-actor | `bench/bitarray-base` | `nerolation/main` | geth `feat/binary-trie/bitarray` |
| state-actor | `bench/bitarray-pbt` | `bench/bitarray-base` | geth `binary/pbt` |
| go-ethereum | `feat/binary-trie/bitarray` | (existing, unchanged) | bitarray-only run |
| go-ethereum | `binary/pbt` | (existing, unchanged) | bitarray + PBT run |
| bintrie-benchmarks | `pbt` (existing) | (existing) | runs both via env-vars |

`bench/bitarray-pbt` branches **from** `bench/bitarray-base` so that the PBT-specific diff is exactly one commit. If round-2 verification fails after round-1 passed, the regression must lie inside that single PBT-swap commit, which makes the search space small.

### Pre-work cleanup

Delete the existing `bench/bitarray-base` and `bench/bitarray-pbt` branches in state-actor before starting. They are built on a base that predates upstream's streaming-builder fixes and are throw-away.

### Binary stashing

Build per branch, stash to `/tmp/bench-bins/{geth,state-actor}-{base,pbt}`. The `integration-test/run.sh` script consumes binaries via `GETH_BIN` and `STATE_ACTOR_BIN` env vars, so swapping between scenarios is one env-var change per run.

## Code Changes

### state-actor `bench/bitarray-base` — one commit on top of `nerolation/main`

**Single conceptual change:** swap path encoding from 1-byte-per-bit to `BitArray.KeyBytes()`.

1. `generator/binary_stack_trie.go`: replace `makePath`:
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
   The `bintrie` import already exists in this file.

2. `generator/regroup.go`: any open-coded path constructions used as map keys or DB keys must produce `KeyBytes()` output too. Grep for `stemBitAt` and `make([]byte, depth)` patterns to find them. Replace with calls to `makePath` where possible, or with equivalent `BitArray.KeyBytes()` derivations.

3. Restore `generator/groundtruth_test.go` (currently uncommitted in working tree from prior session) onto this branch.

4. Generator unit-test golden state roots in `generator/generator_test.go` should **not** shift — the state root is computed over node bytes, not key bytes, so it is invariant to path-encoding format. If a golden root does shift, that signals a real bug to investigate, not a constant to update.

### state-actor `bench/bitarray-pbt` — one commit on top of `bench/bitarray-base`

**Single conceptual change:** swap `GetBinaryTreeKey(addr, X)` calls to the zoned equivalents that geth `binary/pbt` exposes.

1. `generator/binary_stack_trie.go`:
   - Around line 322: `bintrie.GetBinaryTreeKey(addr, zeroKey[:])` → likely `bintrie.GetBinaryTreeKeyBasicData(addr)`. Read context to confirm `zeroKey` is the basic-data tree-index.
   - Around line 370: `bintrie.GetBinaryTreeKey(addr, offset[:])` → likely `bintrie.GetBinaryTreeKeyCodeChunk(addr, chunknr)`. Read context to confirm `offset` is a code-chunk index.

2. `generator/genesis_integration_test.go`: same swap pattern wherever `bintrie.GetBinaryTreeKey(addr, …)` appears.

3. `GetBinaryTreeKeyStorageSlot` call sites (around lines 386, 426) already use the zoned API and need no changes.

### bintrie-benchmarks `pbt` (existing branch, carry-forward uncommitted work)

1. `integration-test/run.sh`: the `--override.verkle=0` → `--override.ubt=0` patch is already in the working tree. Commit it on the `pbt` branch since it applies to both scenarios.

## Verification Flow

Per-branch sequence. Each gate must pass before proceeding.

### Round 1 — `bench/bitarray-base` (geth `feat/binary-trie/bitarray`)

1. **Compile gate:** `go build ./...` in state-actor + `make geth` in go-ethereum.
2. **Unit gate:** `go test ./generator/...` in state-actor.
3. **Groundtruth gate:** `go test ./generator/ -run TestStreamingMatchesGethCommit -v`. Must pass at `n=2,4,8,32,128`. If this fails, stop and debug before touching the integration test.
4. **Integration gate:** `bintrie-benchmarks/integration-test/run.sh` with `TARGET_SIZE=100MB`, `GROUP_DEPTH=8`, and the round-1 binaries stashed at `/tmp/bench-bins/{geth,state-actor}-base`.

### Round 2 — `bench/bitarray-pbt` (geth `binary/pbt`)

Same four gates, with the round-2 binaries at `/tmp/bench-bins/{geth,state-actor}-pbt`. If any gate fails after Round 1 passed, the regression must lie in the single PBT-swap commit.

### Failure handling

- **Compile/unit failure:** fix on the current branch via amend or follow-up commit. No backward-compat shim.
- **Groundtruth failure:** this is the highest-signal gate. Capture the smallest failing `n`. Debug at the trie level using the existing reproducer's logging hooks before proceeding.
- **Integration failure with groundtruth passing:** novel failure mode. Capture the kept test directory (`--keep` flag), inspect `vA`-prefixed keys with `/tmp/bench-debug/inspect`, look for missing-trie-node errors in `geth-deploy.log`.

### Artifacts

- Binaries: `/tmp/bench-bins/{geth,state-actor}-{base,pbt}`.
- Run logs: `/tmp/bench-results/run-{base,pbt}-small.log`.
- Kept test directories: paths logged by `--keep` lines in the run logs.

### Done criterion

Both branches pass all four gates. CSV extraction, `analyze_data.py`, and `TARGET_SIZE=1GB` runs are deferred follow-ups.

## Open risks

- The two `GetBinaryTreeKey(addr, …)` call sites need their tree-index argument inspected before the zoned-helper swap. The mapping `zeroKey → BasicData` and `offset → CodeChunk` is plausible but not yet confirmed. First step of implementation is to read those call sites and confirm.
- The `regroup.go` path-encoding change requires grepping for all path-as-bytes construction sites. The count was 4 last session; it may have changed on `nerolation/main`.
- If groundtruth passes on Round 1 but fails on Round 2, the bug is in the PBT key swap, not in the path encoding — narrow scope to the single PBT-swap commit.

## Reused infrastructure

- `bintrie-benchmarks/integration-test/run.sh` — orchestrator.
- `generator/groundtruth_test.go` (restored from working tree) — unit-level state-root match.
- `/tmp/bench-debug/inspect` — Pebble DB inspector for `vA`-prefixed paths.
- `trie/bintrie/bitarray.go` (`KeyBytes`, `SetBytes`, `MSBs`) — encoding entry points.
