package steam

import (
	"encoding/binary"
	"errors"
	"io"
)

// binaryReader is a minimal little-endian binary reader shared by the
// VDF-derived binary formats this package parses (userstats cache, package
// info cache). Both formats use the same primitive read operations even
// though their higher-level structure differs.
type binaryReader struct {
	r   io.Reader
	buf [8]byte
}

func newBinaryReader(r io.Reader) *binaryReader {
	return &binaryReader{r: r}
}

func (r *binaryReader) readByte() (byte, error) {
	_, err := io.ReadFull(r.r, r.buf[:1])
	return r.buf[0], err
}

func (r *binaryReader) readU32() (uint32, error) {
	_, err := io.ReadFull(r.r, r.buf[:4])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(r.buf[:4]), nil
}

func (r *binaryReader) readCString() (string, error) {
	var result []byte
	for {
		b, err := r.readByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			return string(result), nil
		}
		result = append(result, b)
		if len(result) > 64*1024 {
			return "", errors.New("string exceeds 64KB limit")
		}
	}
}

func (r *binaryReader) skip(n int) error {
	for n > 0 {
		chunk := n
		if chunk > len(r.buf) {
			chunk = len(r.buf)
		}
		if _, err := io.ReadFull(r.r, r.buf[:chunk]); err != nil {
			return err
		}
		n -= chunk
	}
	return nil
}
