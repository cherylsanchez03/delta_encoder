# delta_encoder

Encodes and decodes sequences of int64 using delta encoding with varint output. Deltas are zigzag-varint so that negative steps stay compact; the output is length-prefixed with a Uvarint count and is self-delimiting.

```go
package main

import (
	"fmt"

	"example.com/delta_encoder"
)

func main() {
	values := []int64{10, 20, 30, 25, 5}
	encoded := delta_encoder.Encode(values)
	decoded, err := delta_encoder.Decode(encoded)
	if err != nil {
		panic(err)
	}
	fmt.Println(decoded) // [10 20 30 25 5]
}
```

## Why

Time-series and sensor data often arrive as a stream of integers where consecutive values differ by a small amount. Storing the absolute values wastes the redundancy; storing only the difference between each value and the previous one, then varint-encoding that difference, yields a byte stream that is both compact and self-delimiting. The trade-off is that random access into the middle of a stream requires a full rescan from the start — this library optimises for append-and-read-all, not index access.

Zigzag-varint was chosen over raw Uvarint for the deltas because deltas are frequently negative (any decreasing run), and a raw Uvarint on a negative int64 costs ten bytes, whereas zigzag maps small negatives to small positive codes. For a series that decreases as often as it increases this roughly halves the output size compared to the naive approach.

## Edge cases

The decoder is strict about trailing bytes: if `Decode` is given a buffer that contains a complete stream followed by extra data, it returns an error rather than silently dropping the tail. This catches misaligned buffers early. Truncated inputs (a varint whose continuation bit promises a byte that never arrives) are reported via `ErrTruncated`. The first value in a stream is encoded as a delta from zero, so a leading negative value is valid and handled.

## API

- `func Encode(values []int64) []byte` — encode absolute values to a self-delimiting byte slice.
- `func EncodeTo(buf []byte, values []int64) []byte` — like `Encode` but appends onto `buf`.
- `func EncodeCount(deltas []int64) []byte` — encode pre-computed deltas (same output format as `Encode`).
- `func Decode(data []byte) ([]int64, error)` — reverse of `Encode`; returns an error on truncation or trailing bytes.
- `func EncodeValues(raw *bytes.Buffer) ([]byte, error)` — encode from a buffer of little-endian int64s, skipping an intermediate `[]int64` allocation. Requires a whole number of int64s.
- `var ErrTruncated` — sentinel returned when input ends mid-varint.
- `const MaxVarintLen16`, `const MaxVarintLen32`, `const MaxVarintLen64` — maximum byte lengths for pre-sizing buffers.
