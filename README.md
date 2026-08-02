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
if err != nil {
    log.Fatal(err)
}
encoder, err := riblt.NewEncoder[uint64](codec)
if err != nil {
    log.Fatal(err)
}
decoder, err := riblt.NewDecoderWithLimits[uint64](codec, riblt.DecoderLimits{
    MaxCells: 10_000, MaxLocalSymbols: 1_000_000, MaxDecodedSymbols: 10_000,
})
if err != nil {
    log.Fatal(err)
}
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
4. `Encoder.Cells` makes the sketch rateless: the sender can keep streaming.
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
domains, truncated to the algorithm's 64-bit fields. Always check their
constructor errors: a weak-key or otherwise uninitialized built-in codec is
rejected by `NewEncoder`, `NewDecoder`, `NewSketch`, and `Hash`.

Keying prevents a party that does not know the key from cheaply choosing hash
collisions. It does not make a peer that shares the key non-adversarial; the
surrounding protocol must decide which peers are trusted and enforce finite
work and transport limits. A `MappingHash` collision gives two distinct
symbols the same placement sequence. If they occur on opposite sides of the
difference, decoding may never converge, and an independent `Checksum` cannot
repair that placement collision. The 64-bit hashes make accidental collisions
unlikely, not impossible. Custom codecs exposed to untrusted input should
provide equivalent keyed domain separation. Public prehashed insertion is
intentionally absent: insertion validates, clones, and recomputes both hashes
at the library boundary. `Hash` remains diagnostic only.

## Follow one reconciliation in code

The sender registers its set before streaming:

```go
encoder, err := riblt.NewEncoder[Symbol](codec)
if err != nil {
    return err
}
for _, value := range upstream {
    _ = encoder.Add(value)
}
```

The receiver likewise registers local state before the first cell:

```go
decoder, err := riblt.NewDecoder[Symbol](codec)
if err != nil {
    return err
}
for _, value := range downstream {
    _ = decoder.AddLocal(value)
}
```

`Cells` exposes the rateless stream as a lazy `iter.Seq2`. The protocol loop
can range until the receiver has enough information, then break without
producing another cell:

```go
for cell, err := range encoder.Cells() {
    if err != nil {
        return err
    }
    // Send cell over an ordered transport, then process it downstream.
    if err := decoder.AddCoded(cell); err != nil {
        return err
    }
    if err := decoder.TryDecode(); err != nil {
        return err
    }
    if decoder.Complete() {
        break
    }
}

add := decoder.Remote()   // upstream-only
remove := decoder.Local() // downstream-only
```

The sequence is tied to the encoder's current position rather than replayable.
It produces one cell only when the range requests a value, and a later range
resumes at the following cell. A transport or event loop that needs explicit
one-cell-at-a-time control can adapt the sequence with `iter.Pull2`, calling
the returned stop function when it finishes. Cells must arrive in order. The
receiver needs a way to signal completion, or the sender can transmit bounded
batches and await an acknowledgement. Cache hashes inside a custom immutable
codec only if profiling justifies it; callers cannot supply cached hashes as
trusted state.

If a prefix length is known in advance, `Sketch[T]` offers a fixed-length form
with `Add`, `Remove`, `Subtract`, and `Decode`. Build each input sketch first;
successful `Subtract` seals its receiver, after which only `Cells` and `Decode`
may be called. A sealed sketch represents a signed difference and cannot be
used as either operand of another subtraction. Streaming is the central RIBLT
advantage because it avoids guessing the difference size.

## Choose what a symbol means in your project

The generic type is not limited to integer IDs. The useful question is which
fixed-width, reversibly XOR-able representation best matches the data flow in
your project:

| Project data | RIBLT symbol | Codec approach |
| --- | --- | --- |
| Numeric IDs or versions | `uint64` | Use `NewUint64Codec`. |
| UUIDs, hashes, or fixed binary keys | fixed-width `[]byte` | Use `NewBytesCodec`. |
| Fixed-layout domain records | the whole struct | Implement `Codec[Record]`; XOR every field and hash a canonical encoding. |
| Small, bounded text or blobs | length plus padded payload | Encode one fixed-width slot and use `BytesCodec`. |
| Large documents, files, or variable graphs | content digest or stable ID | Reconcile IDs, then fetch missing values outside RIBLT. |

Every valid symbol representation must be closed under XOR because coded cells
contain combinations of symbols, not just original values. That rules out
direct XOR of Go strings, maps, pointers, variable-length slices, and structs
whose invariants reject intermediate bit patterns. Canonicalize those values
into fixed-width bytes or reconcile fixed-width identities instead.

Run the complete-record example:

```sh
go run ./cmd/structs
```

[`cmd/structs`](cmd/structs/main.go) carries an entire `Record` through
`Encoder[Record]` and `Decoder[Record]`. Its custom codec XORs the fixed-width
fields directly and delegates keyed mapping and checksum hashes to
`BytesCodec`. An updated record is naturally reported as the old value to
remove and the new value to add.

Run the bounded-document example:

```sh
go run ./cmd/documents
```

[`cmd/documents`](cmd/documents/main.go) carries actual UTF-8 document bodies.
It encodes each one as a 256-byte slot with an explicit byte length and zero
padding. This is useful when the project has a real maximum payload size. It
is intentionally not a general document transport: for large or unbounded
documents, reconcile content digests and retrieve the missing bodies through
the surrounding synchronization protocol.

Run the content-addressed synchronization example:

```sh
go run ./cmd/chunks
```

[`cmd/chunks`](cmd/chunks/main.go) scales that pattern to larger values. It
splits a document into chunks, reconciles the set of SHA-256 chunk IDs, copies
only missing chunk bodies, and rebuilds the document from an ordered manifest.
The manifest preserves order and repeated chunks while RIBLT efficiently
discovers the set difference. Each transferred chunk and the rebuilt document
are verified against their digest. The example uses fixed-size chunks for
clarity; content-defined chunking can retain more sharing after insertions.

## Observe communication scaling

```sh
go run ./cmd/experiment
go test ./...
go test -fuzz=FuzzReconcile -fuzztime=30s
go test -fuzz=FuzzMalformedCodedBytes -fuzztime=30s
go test -bench=. -benchmem ./...
```

`go test ./...` runs only the seed corpus for fuzz tests. Use the explicit
`-fuzz` commands above for sustained fuzzing; Go runs one fuzz target per
invocation.

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
