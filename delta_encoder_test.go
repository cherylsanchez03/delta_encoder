package delta_encoder

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestRoundTripEmpty(t *testing.T) {
	out := Encode(nil)
	if len(out) != 1 || out[0] != 0 {
		t.Fatalf("expected single zero byte for empty input, got %v", out)
	}
	got, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestRoundTripSingle(t *testing.T) {
	in := []int64{42}
	out := Encode(in)
	got, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("expected [42], got %v", got)
	}
}

func TestRoundTripIncreasing(t *testing.T) {
	in := []int64{10, 20, 30, 40, 50}
	got, err := Decode(Encode(in))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("length mismatch: %d vs %d", len(got), len(in))
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestRoundTripDecreasing(t *testing.T) {
	// Decreasing series produces negative deltas; zigzag-varint keeps them compact.
	in := []int64{100, 90, 80, 70, 60}
	got, err := Decode(Encode(in))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestRoundTripMixedSigns(t *testing.T) {
	in := []int64{-5, 3, -2, 0, 7, -100}
	got, err := Decode(Encode(in))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestRoundTripExtremes(t *testing.T) {
	in := []int64{1<<63 - 1, -(1 << 62), 0, -1, 1}
	got, err := Decode(Encode(in))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestRoundTripConstant(t *testing.T) {
	in := []int64{7, 7, 7, 7, 7, 7, 7}
	out := Encode(in)
	got, err := Decode(out)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestRoundTripLarge(t *testing.T) {
	in := make([]int64, 1000)
	for i := range in {
		in[i] = int64(i) * 3
	}
	got, err := Decode(Encode(in))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestEncodeFormat(t *testing.T) {
	// Verify the on-the-wire format explicitly so a change in encoding is caught.
	out := Encode([]int64{10, 20, 30})
	want := []byte{3, 20, 20, 20}
	if len(out) != len(want) {
		t.Fatalf("length %d, want %d (got %v)", len(out), len(want), out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("byte %d: got %d want %d (full %v)", i, out[i], want[i], out)
		}
	}
}

func TestDecodeTruncatedCount(t *testing.T) {
	// A varint whose continuation bit is set but no continuation byte follows.
	_, err := Decode([]byte{0x80})
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("expected ErrTruncated, got %v", err)
	}
}

func TestDecodeTruncatedDelta(t *testing.T) {
	// Count=2, first delta ok, second missing.
	buf := []byte{2, 20}
	_, err := Decode(buf)
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("expected ErrTruncated, got %v", err)
	}
}

func TestDecodeTrailingBytes(t *testing.T) {
	// Count=1, one delta, then one stray byte.
	buf := []byte{1, 20, 0xFF}
	_, err := Decode(buf)
	if err == nil {
		t.Fatalf("expected error for trailing bytes, got nil")
	}
}

func TestEncodeTo(t *testing.T) {
	prefix := []byte{0xAA, 0xBB}
	out := EncodeTo(prefix, []int64{10, 20})
	if len(out) < 2 || out[0] != 0xAA || out[1] != 0xBB {
		t.Fatalf("prefix not preserved: %v", out)
	}
	rest := out[2:]
	got, err := Decode(rest)
	if err != nil {
		t.Fatalf("Decode failed on tail: %v", err)
	}
	if len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("expected [10 20], got %v", got)
	}
}

func TestEncodeCount(t *testing.T) {
	// Pre-computed deltas; output must be identical to Encode.
	a := EncodeCount([]int64{10, 10, 10})
	b := Encode([]int64{10, 20, 30})
	if !bytes.Equal(a, b) {
		t.Fatalf("EncodeCount and Encode disagree: %v vs %v", a, b)
	}
}

func TestEncodeValuesRoundTrip(t *testing.T) {
	in := []int64{5, 15, 12, -3}
	var raw bytes.Buffer
	for _, v := range in {
		if err := binary.Write(&raw, binary.LittleEndian, v); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	encoded, err := EncodeValues(&raw)
	if err != nil {
		t.Fatalf("EncodeValues failed: %v", err)
	}
	got, err := Decode(encoded)
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	for i := range in {
		if got[i] != in[i] {
			t.Fatalf("index %d: got %d want %d", i, got[i], in[i])
		}
	}
}

func TestEncodeValuesEmpty(t *testing.T) {
	var raw bytes.Buffer
	encoded, err := EncodeValues(&raw)
	if err != nil {
		t.Fatalf("EncodeValues failed on empty: %v", err)
	}
	if len(encoded) != 1 || encoded[0] != 0 {
		t.Fatalf("expected single zero byte, got %v", encoded)
	}
}

func TestEncodeValuesPartialWord(t *testing.T) {
	var raw bytes.Buffer
	raw.Write([]byte{1, 2, 3}) // 3 bytes, not a multiple of 8
	_, err := EncodeValues(&raw)
	if err == nil {
		t.Fatalf("expected error for partial word, got nil")
	}
}

func TestEncodeValuesNilBuffer(t *testing.T) {
	_, err := EncodeValues(nil)
	if err == nil {
		t.Fatalf("expected error for nil buffer, got nil")
	}
}
