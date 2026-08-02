// Package riblt implements generic Rateless Invertible Bloom Lookup Tables.
//
// A RIBLT reconciles two sets whose symmetric difference is much smaller than
// either set. An encoder exposes an unbounded, lazy iter.Seq2 of coded symbols;
// a decoder combines a prefix with its local set and peels it until the
// difference is known. Communication is proportional to the difference, not
// the set size.
//
// Values need not have methods. Applications can use the keyed Uint64Codec or
// fixed-width BytesCodec, or inject a Codec for another reversible fixed-width
// representation. A codec owns cloning, equality, validation, XOR, hashes, and
// a stable compatibility identity. RIBLT is a set protocol: adding the same
// value twice is an error.
//
// Encoder, Decoder, and Sketch are not safe for concurrent method calls.
// DecoderLimits bounds library allocations driven by coded cells, local
// symbols, and decoded symbols. The caller remains responsible for an
// authenticated, ordered transport, serialization limits, deadlines, protocol
// version and codec compatibility negotiation, and safe application of the
// decoded difference.
//
// Keyed hashes deter collision selection only by parties that do not know the
// key. A MappingHash collision gives different symbols the same placement
// sequence and can prevent convergence; Checksum cannot repair it. Protocols
// must therefore bound peer-controlled work even when using keyed codecs.
package riblt
