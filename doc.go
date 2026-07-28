// Package riblt implements generic Rateless Invertible Bloom Lookup Tables.
//
// A RIBLT reconciles two sets whose symmetric difference is much smaller than
// either set. An encoder emits an unbounded stream of coded symbols; a decoder
// combines a prefix with its local set and peels it until the difference is
// known. Communication is proportional to the difference, not the set size.
//
// Values need not have methods. Instead, applications inject a Codec. This is
// useful for built-in and third-party types and makes the distinct placement
// and validation hashes visible. RIBLT is a set protocol: adding the same value
// twice is an error.
//
// This implementation assumes trusted inputs. An unkeyed checksum permits
// adversarially constructed XOR collisions; production protocols exposed to
// attackers should use a keyed, domain-separated checksum, authenticate the
// transport, and apply resource limits.
package riblt
