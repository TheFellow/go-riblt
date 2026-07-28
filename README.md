# Generic RIBLT in Go

This repository is a small, generic implementation of a Rateless Invertible
Bloom Lookup Table. RIBLT reconciles two sets efficiently when their symmetric
difference is much smaller than the sets themselves. The sender does not need
to estimate that difference first: it streams coded cells until the receiver
can decode.

The implementation is generic over `T`. An application supplies a `Codec[T]`
that defines the XOR algebra and two independent hashes, so built-in and
third-party types need no RIBLT-specific methods.

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
    XOR(a, b T) T
    MappingHash(T) uint64
    Checksum(T) uint64
}
```

`IsZero` recognizes that identity, including for non-comparable types. `XOR`
must be pure and obey `x XOR zero = x` and `x XOR x = zero`. For slices,
maps, or pointers, it must return independent storage and must not mutate or
retain its arguments. Variable-length records normally need a fixed-width,
reversible encoding (or a fixed-width content ID) before they become symbols.

The two hashes have different jobs. The mapping hash determines placement; the
checksum rejects false singleton candidates caused by XOR collisions. They
must be domain-separated. The demo hashes are deterministic and educational,
not adversarially secure. A protocol accepting untrusted input should use a
keyed checksum, authenticate messages, and impose cell and memory limits.

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
`HashedSymbol` values when hashing large values is expensive.

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
protocol. A production system still needs serialization, version negotiation,
authentication, retransmission or ordering, resource limits, and application
semantics for applying additions and removals. Duplicate symbols are rejected:
if multiplicity matters, encode a unique occurrence ID or choose a multiset
protocol.
