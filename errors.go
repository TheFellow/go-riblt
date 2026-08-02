package riblt

import "errors"

var (
	ErrNilCodec         = errors.New("riblt: nil codec")
	ErrDuplicate        = errors.New("riblt: duplicate symbol")
	ErrNotPresent       = errors.New("riblt: symbol is not present")
	ErrEncoderStarted   = errors.New("riblt: cannot add after encoding started")
	ErrDecoderStarted   = errors.New("riblt: cannot add local symbol after decoding started")
	ErrMappingOverflow  = errors.New("riblt: mapping index overflow")
	ErrCountOverflow    = errors.New("riblt: coded-symbol count overflow")
	ErrDifferentLength  = errors.New("riblt: sketches have different lengths")
	ErrAlreadyDecoded   = errors.New("riblt: decoder has already completed")
	ErrInvalidSymbol    = errors.New("riblt: invalid symbol")
	ErrInvalidWidth     = errors.New("riblt: byte codec width must be positive")
	ErrWeakKey          = errors.New("riblt: hash key must contain at least 16 bytes")
	ErrIncompatible     = errors.New("riblt: incompatible codec configuration")
	ErrResourceLimit    = errors.New("riblt: decoder resource limit exceeded")
	ErrInvalidCodec     = errors.New("riblt: invalid codec configuration")
	ErrSketchSubtracted = errors.New("riblt: sketch has already been subtracted")
)
