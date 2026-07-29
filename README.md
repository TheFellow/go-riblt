# Generic RIBLT in Go

This repository is a small, generic implementation of a Rateless Invertible
Bloom Lookup Table. RIBLT reconciles two sets efficiently when their symmetric
difference is much smaller than the sets themselves. The sender does not need
to estimate that difference first: it streams coded cells until the receiver
can decode.

The implementation is generic over `T`. Ready-to-use keyed codecs cover
`uint64` and fixed-width `[]byte` values. Applications can supply a `Codec[T]`
for another reversible fixed-width representation.

```go
key := []byte("0123456789abcdef0123456789abcdef")
codec, err := riblt.NewUint64Codec(key)
encoder, err := riblt.NewEncoder[uint64](codec)
decoder, err := riblt.NewDecoderWithLimits[uint64](codec, riblt.DecoderLimits{
    MaxCells: 10_000, MaxLocalSymbols: 1_000_000, MaxDecodedSymbols: 10_000,
})
```

Both peers must use the same key and codec parameters. Provision keys through
the surrounding authenticated protocol; do not hard-code the illustrative
key above.

## Run the guided example

```sh
go run ./cmd/walkthrough
```

The walkthrough uses these sets:

```text
upstream:   [100 101 102 103]
downstream: [100 102 104]
```

It prints every transmitted cell and the decoder state after each peel. Read
the output alongside [`cmd/walkthrough/main.go`](cmd/walkthrough/main.go). The
sections demonstrate:

1. `Zero` and `XOR` form the Boolean group used to combine and recover values.
2. `MappingHash` deterministically chooses a symbol's sparse sequence of cells.
3. `Checksum` independently validates a candidate singleton.
4. `Encoder.Next` makes the sketch rateless: the sender can keep streaming.
5. `Decoder.AddCoded` subtracts the receiver's local set from each incoming
   cell.
6. `Decoder.TryDecode` peels `+1` (upstream-only), `-1`
   (downstream-only), and resolved zero cells, propagating each discovery.
7. `Decoder.Complete` tells the receiver when it can stop the sender.

The final result tells the downstream to add `101` and `103`, and remove `104`.
Order is an implementation detail; treat the results as sets.

## The generic contract

The demo uses `Symbol`, a struct containing a `uint64` ID:

```go
type Codec[T any] interface {
    Zero() T
    IsZero(T) bool
    Clone(T) T
    Equal(a, b T) bool
    Validate(T) error
    XOR(a, b T) T
    MappingHash(T) uint64
    Checksum(T) uint64
    CompatibilityID() [32]byte
}
```

`IsZero` recognizes the identity. `XOR` must be pure and obey
`x XOR zero = x` and `x XOR x = zero`. `Clone` gives the library ownership of
mutable values and `Equal` distinguishes genuine duplicates even when hashes
collide. `Validate` defines the accepted algebra. `CompatibilityID` binds all
of these semantics, the hash key, and encoding parameters so fixed sketches
cannot be subtracted under mismatched configurations.

There is no universal XOR default for `T any`: Go cannot reversibly XOR an
arbitrary struct, string, map, or variable-length slice. Convert records to a
fixed-width content ID, use `BytesCodec`, or implement the full contract.

The built-in codecs use HMAC-SHA-256 with independent mapping and checksum
domains, truncated to the algorithm's 64-bit fields. Custom codecs exposed to
untrusted input should provide equivalent keyed domain separation. Public
prehashed insertion is intentionally absent: insertion validates, clones, and
recomputes both hashes at the library boundary. `Hash` remains diagnostic only.

## Follow one reconciliation in code

The sender registers its set before streaming:

```go
encoder, _ := riblt.NewEncoder[Symbol](codec)
for _, value := range upstream {
    _ = encoder.Add(value)
}
```

The receiver likewise registers local state before the first cell:

```go
decoder, _ := riblt.NewDecoder[Symbol](codec)
for _, value := range downstream {
    _ = decoder.AddLocal(value)
}
```

The protocol loop can run over any ordered transport:

```go
for !decoder.Complete() {
    cell, _ := encoder.Next() // send this cell
    _ = decoder.AddCoded(cell)
    _ = decoder.TryDecode()
}

add := decoder.Remote()   // upstream-only
remove := decoder.Local() // downstream-only
```

Cells must arrive in order. The receiver needs a way to signal completion, or
the sender can transmit bounded batches and await an acknowledgement. Cache
hashes inside a custom immutable codec only if profiling justifies it; callers
cannot supply cached hashes as trusted state.

If a prefix length is known in advance, `Sketch[T]` offers a fixed-length form
with `Add`, `Remove`, `Subtract`, and `Decode`. Streaming is the central RIBLT
advantage because it avoids guessing the difference size.

## Observe communication scaling

```sh
go run ./cmd/experiment
go test ./...
go test -bench=. -benchmem ./...
```

The experiment holds symmetric difference constant while changing total set
size. Its `24 payload bytes` per cell is an accounting convention for the
demo's three fixed-width fields (`Symbol`, `Checksum`, and `Count`); transport
framing and serialization are additional. Exact cell counts vary with symbol
hashes, but the useful comparison is cells versus symmetric difference rather
than cells versus total set size.

## Scope

RIBLT is a set reconciliation primitive, not a complete synchronization
protocol. `ProtocolVersion` is `go-riblt/v1`; it fixes the mapping constants,
floating-point gap calculation, cell semantics, and hash domains. Put that
version and `CompatibilityID` in the authenticated handshake and reject a
mismatch. The package intentionally does not prescribe serialization.

Production callers still own framing and byte limits, authentication and key
agreement, ordered delivery/retransmission, deadlines/cancellation, and atomic
application of additions and removals. Configure finite `DecoderLimits` for
untrusted peers; they complement rather than replace transport limits.
Encoder, Decoder, and Sketch are not safe for concurrent method calls.
Duplicate symbols are rejected; encode a unique occurrence ID if multiplicity
matters.
