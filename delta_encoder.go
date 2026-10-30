package delta_encoder

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	// MaxVarintLen16 is the maximum number of bytes a varint-encoded uint16 can occupy.
	MaxVarintLen16 = 3
	// MaxVarintLen32 is the maximum number of bytes a varint-encoded uint32 can occupy.
	MaxVarintLen32 = 5
	// MaxVarintLen64 is the maximum number of bytes a varint-encoded uint64 can occupy.
	MaxVarintLen64 = 10
)

// ErrTruncated is returned by Decode when the byte slice ends before a complete
// varint or element can be read.
var ErrTruncated = fmt.Errorf("delta_encoder: truncated input")

// ErrCountNegative is returned by Encode when a negative count is supplied.
// Decode never produces a negative count.
var ErrCountNegative = fmt.Errorf("delta_encoder: count must be non-negative")

// Encode encodes a slice of int64 values using delta encoding followed by
// zigzag-varint. The output is self-delimiting: it begins with the count of
// elements as a Uvarint, followed by each delta as a Zigzag-varint. An empty
// input produces a single zero byte.
//
// The choice of zigzag-varint for deltas (rather than Uvarint on the raw
// difference) means that negative deltas — common when a series decreases —
// stay compact at two bytes instead of ballooning to ten. This matters for
// time-series data where readings often step down as well as up.
func Encode(values []int64) []byte {
	buf := make([]byte, 0, 1+len(values)*MaxVarintLen64)
	buf = binary.AppendUvarint(buf, uint64(len(values)))
	prev := int64(0)
	for _, v := range values {
		delta := v - prev
		buf = binary.AppendVarint(buf, delta)
		prev = v
	}
	return buf
}

// EncodeTo appends the delta-encoded form of values onto buf, returning the
// new slice. It is a convenience wrapper around Encode for callers that already
// own a buffer and want to avoid an extra copy.
func EncodeTo(buf []byte, values []int64) []byte {
	return append(buf, Encode(values)...)
}

// EncodeCount is like Encode but works from a pre-existing slice of deltas
// (already computed) rather than raw values. The count is emitted first so the
// output format is identical to Encode's; only the caller is spared the
// subtraction. This is useful when deltas are produced by an upstream stage.
func EncodeCount(deltas []int64) []byte {
	buf := make([]byte, 0, 1+len(deltas)*MaxVarintLen64)
	buf = binary.AppendUvarint(buf, uint64(len(deltas)))
	for _, d := range deltas {
		buf = binary.AppendVarint(buf, d)
	}
	return buf
}

// Decode reverses Encode. It reads a count from the head of data, then reads
// that many zigzag-varint deltas, accumulating them back into absolute values.
// On a malformed or truncated input it returns a non-nil error and a nil slice.
//
// Intentionally strict: any trailing bytes after the declared elements are
// reported via ErrTrailing. This catches misaligned buffers early rather than
// silently dropping data.
func Decode(data []byte) ([]int64, error) {
	count, n := binary.Uvarint(data)
	if n <= 0 {
		return nil, ErrTruncated
	}
	data = data[n:]

	values := make([]int64, 0, count)
	var prev int64
	for i := uint64(0); i < count; i++ {
		delta, m := binary.Varint(data)
		if m <= 0 {
			return nil, ErrTruncated
		}
		prev += delta
		values = append(values, prev)
		data = data[m:]
	}
	if len(data) > 0 {
		return nil, fmt.Errorf("delta_encoder: %d trailing byte(s)", len(data))
	}
	return values, nil
}

// EncodeValues encodes a slice of int64 values directly from a bytes.Buffer of
// raw little-endian int64s, returning the encoded bytes. This is useful when
// the input arrives as a packed buffer (e.g., from a binary file) and avoids an
// intermediate allocation of a []int64. The buffer must contain a whole number
// of int64s; otherwise an error is returned.
func EncodeValues(raw *bytes.Buffer) ([]byte, error) {
	if raw == nil {
		return nil, fmt.Errorf("delta_encoder: nil source buffer")
	}
	remaining := raw.Len()
	if remaining%8 != 0 {
		return nil, fmt.Errorf("delta_encoder: source has %d bytes, not a multiple of 8", remaining)
	}
	count := remaining / 8
	out := make([]byte, 0, 1+count*MaxVarintLen64)
	out = binary.AppendUvarint(out, uint64(count))
	prev := int64(0)
	for i := 0; i < count; i++ {
		var v int64
		if err := binary.Read(raw, binary.LittleEndian, &v); err != nil {
			return nil, fmt.Errorf("delta_encoder: read failed: %w", err)
		}
		out = binary.AppendVarint(out, v-prev)
		prev = v
	}
	return out, nil
}
