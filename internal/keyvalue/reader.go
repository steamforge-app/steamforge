package keyvalue

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"unicode/utf16"
)

const (
	maxCStringLen = 64 * 1024 // 64KB limit for C-string reads
	maxDepth      = 256       // maximum nesting depth for binary KV trees
)

// LoadBinary reads a binary KeyValue file from disk.
func LoadBinary(path string) (*Node, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	return ReadBinary(f)
}

// ReadBinary reads a binary KeyValue tree from a reader.
func ReadBinary(r io.Reader) (*Node, error) {
	br := &binaryReader{r: r}
	root := &Node{
		Type: TypeNone,
		Name: "<root>",
	}
	if err := readChildren(br, root, 0); err != nil {
		return nil, err
	}
	if len(root.Children) == 1 {
		return root.Children[0], nil
	}
	return root, nil
}

func readChildren(r *binaryReader, parent *Node, depth int) error {
	if depth > maxDepth {
		return fmt.Errorf("maximum nesting depth %d exceeded", maxDepth)
	}

	for {
		typeByte, err := r.ReadByte()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read type: %w", err)
		}

		t := Type(typeByte)
		if t == TypeEnd {
			return nil
		}

		name, err := r.ReadCString()
		if err != nil {
			return fmt.Errorf("read name: %w", err)
		}

		node := &Node{
			Type: t,
			Name: name,
		}

		switch t {
		case TypeNone:
			if err := readChildren(r, node, depth+1); err != nil {
				return fmt.Errorf("read children of %s: %w", name, err)
			}
		case TypeString:
			s, err := r.ReadCString()
			if err != nil {
				return fmt.Errorf("read string value: %w", err)
			}
			node.Value = s
			node.Valid = true
		case TypeWideString:
			s, err := r.ReadWideString()
			if err != nil {
				return fmt.Errorf("read wide string value: %w", err)
			}
			node.Value = s
			node.Valid = true
		case TypeInt32:
			v, err := r.ReadInt32()
			if err != nil {
				return fmt.Errorf("read int32: %w", err)
			}
			node.Value = v
			node.Valid = true
		case TypeUInt64:
			v, err := r.ReadUInt64()
			if err != nil {
				return fmt.Errorf("read uint64: %w", err)
			}
			node.Value = v
			node.Valid = true
		case TypeFloat32:
			v, err := r.ReadFloat32()
			if err != nil {
				return fmt.Errorf("read float32: %w", err)
			}
			node.Value = v
			node.Valid = true
		case TypeColor:
			v, err := r.ReadUInt32()
			if err != nil {
				return fmt.Errorf("read color: %w", err)
			}
			node.Value = v
			node.Valid = true
		case TypePointer:
			v, err := r.ReadUInt32()
			if err != nil {
				return fmt.Errorf("read pointer: %w", err)
			}
			node.Value = v
			node.Valid = true
		default:
			return fmt.Errorf("unknown type %d for key %s", t, name)
		}

		parent.Children = append(parent.Children, node)
	}
}

// binaryReader wraps an io.Reader for reading binary data.
type binaryReader struct {
	r   io.Reader
	buf [8]byte
}

func (r *binaryReader) ReadByte() (byte, error) {
	_, err := io.ReadFull(r.r, r.buf[:1])
	return r.buf[0], err
}

func (r *binaryReader) ReadCString() (string, error) {
	var result []byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		if b == 0 {
			return string(result), nil
		}
		result = append(result, b)
		if len(result) > maxCStringLen {
			return "", fmt.Errorf("string exceeds %d byte limit", maxCStringLen)
		}
	}
}

func (r *binaryReader) ReadInt32() (int32, error) {
	_, err := io.ReadFull(r.r, r.buf[:4])
	if err != nil {
		return 0, err
	}
	return int32(binary.LittleEndian.Uint32(r.buf[:4])), nil
}

func (r *binaryReader) ReadUInt32() (uint32, error) {
	_, err := io.ReadFull(r.r, r.buf[:4])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(r.buf[:4]), nil
}

func (r *binaryReader) ReadUInt64() (uint64, error) {
	_, err := io.ReadFull(r.r, r.buf[:8])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(r.buf[:8]), nil
}

func (r *binaryReader) ReadWideString() (string, error) {
	var codepoints []uint16
	for {
		_, err := io.ReadFull(r.r, r.buf[:2])
		if err != nil {
			return "", err
		}
		ch := binary.LittleEndian.Uint16(r.buf[:2])
		if ch == 0 {
			return string(utf16.Decode(codepoints)), nil
		}
		codepoints = append(codepoints, ch)
		if len(codepoints) > maxCStringLen {
			return "", fmt.Errorf("wide string exceeds %d char limit", maxCStringLen)
		}
	}
}

func (r *binaryReader) ReadFloat32() (float32, error) {
	v, err := r.ReadUInt32()
	if err != nil {
		return 0, err
	}
	return math.Float32frombits(v), nil
}
