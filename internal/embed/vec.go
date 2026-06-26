package embed

import (
	"encoding/binary"
	"math"
)

// EncodeVector serializes an embedding as little-endian float32 bytes, the form
// stored in the note_vectors BLOB column.
func EncodeVector(v []float32) []byte {
	b := make([]byte, len(v)*4)
	for i, x := range v {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(x))
	}
	return b
}

// DecodeVector is the inverse of EncodeVector.
func DecodeVector(b []byte) []float32 {
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}
