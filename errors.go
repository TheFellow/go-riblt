package riblt

import "errors"

var (
	ErrNilCodec        = errors.New("riblt: nil codec")
	ErrDuplicate       = errors.New("riblt: duplicate symbol")
	ErrNotPresent      = errors.New("riblt: symbol is not present")
	ErrEncoderStarted  = errors.New("riblt: cannot add after encoding started")
	ErrDecoderStarted  = errors.New("riblt: cannot add local symbol after decoding started")
	ErrMappingOverflow = errors.New("riblt: mapping index overflow")
	ErrCountOverflow   = errors.New("riblt: coded-symbol count overflow")
	ErrDifferentLength = errors.New("riblt: sketches have different lengths")
	ErrAlreadyDecoded  = errors.New("riblt: decoder has already completed")
)
