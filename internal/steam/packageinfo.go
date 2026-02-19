package steam

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// packageinfo.vdf binary format constants.
const (
	pkgMagicMask    = 0xFFFFFF00
	pkgMagicExpect  = 0x06565500 // upper 3 bytes: \x06UV
	pkgVersionV39   = 0x27
	pkgVersionV40   = 0x28
	pkgTerminator   = 0xFFFFFFFF
	pkgSHA1Size     = 20
)

// Standard binary VDF type codes (different from the keyvalue package's
// schema-specific codes — packageinfo.vdf uses the standard mapping).
const (
	bvdfNone   byte = 0x00
	bvdfString byte = 0x01
	bvdfInt32  byte = 0x02
	bvdfEnd    byte = 0x08
	bvdfEndAlt byte = 0x0B
)

// ScanPackageAppIDs reads packageinfo.vdf and returns a deduplicated list of
// all app IDs found across every package entry.
func ScanPackageAppIDs() ([]uint32, error) {
	installPath, err := GetInstallPath()
	if err != nil {
		return nil, fmt.Errorf("get Steam install path: %w", err)
	}

	path := filepath.Join(installPath, "appcache", "packageinfo.vdf")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open packageinfo.vdf: %w", err)
	}
	defer f.Close()

	return parsePackageInfo(f)
}

func parsePackageInfo(r io.Reader) ([]uint32, error) {
	br := &pkgReader{r: r}

	// Header: 4-byte magic, 4-byte universe.
	magic, err := br.readU32()
	if err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	version := magic & 0xFF
	if magic&pkgMagicMask != pkgMagicExpect {
		return nil, fmt.Errorf("bad magic 0x%08X (expected upper bytes 0x065655xx)", magic)
	}
	if version != pkgVersionV39 && version != pkgVersionV40 {
		return nil, fmt.Errorf("unsupported packageinfo version %d", version)
	}

	universe, err := br.readU32()
	if err != nil {
		return nil, fmt.Errorf("read universe: %w", err)
	}
	slog.Debug("packageinfo header", "version", version, "universe", universe)

	hasPICSToken := version >= pkgVersionV40

	seen := make(map[uint32]bool)
	var allAppIDs []uint32

	for {
		pkgID, err := br.readU32()
		if err != nil {
			return nil, fmt.Errorf("read package id: %w", err)
		}
		if pkgID == pkgTerminator {
			break
		}

		// SHA-1 hash (20 bytes)
		if err := br.skip(pkgSHA1Size); err != nil {
			return nil, fmt.Errorf("skip sha1 for pkg %d: %w", pkgID, err)
		}

		// Change number (4 bytes)
		if _, err := br.readU32(); err != nil {
			return nil, fmt.Errorf("read change number for pkg %d: %w", pkgID, err)
		}

		// PICS token (8 bytes, v40+ only)
		if hasPICSToken {
			if err := br.skip(8); err != nil {
				return nil, fmt.Errorf("skip PICS token for pkg %d: %w", pkgID, err)
			}
		}

		// Binary VDF blob — extract app IDs.
		appIDs, err := readPackageVDFAppIDs(br)
		if err != nil {
			return nil, fmt.Errorf("parse VDF for pkg %d: %w", pkgID, err)
		}
		for _, id := range appIDs {
			if id != 0 && !seen[id] {
				seen[id] = true
				allAppIDs = append(allAppIDs, id)
			}
		}
	}

	slog.Info("packageinfo scan complete", "unique_appids", len(allAppIDs))
	return allAppIDs, nil
}

// readPackageVDFAppIDs reads one binary VDF blob from the stream, returning
// only the app IDs found under the "appids" key.
func readPackageVDFAppIDs(br *pkgReader) ([]uint32, error) {
	return vdfCollectAppIDs(br, 0, false)
}

// vdfCollectAppIDs recursively walks a binary VDF object. When inside an
// "appids" container, int32 values are collected as app IDs.
func vdfCollectAppIDs(br *pkgReader, depth int, collectInts bool) ([]uint32, error) {
	if depth > 32 {
		return nil, fmt.Errorf("VDF nesting too deep")
	}

	var appIDs []uint32

	for {
		typeByte, err := br.readByte()
		if err != nil {
			return appIDs, err
		}
		if typeByte == bvdfEnd || typeByte == bvdfEndAlt {
			return appIDs, nil
		}

		key, err := br.readCString()
		if err != nil {
			return nil, fmt.Errorf("read key at depth %d: %w", depth, err)
		}

		switch typeByte {
		case bvdfNone:
			// Nested object. Collect ints if this is the "appids" node.
			isAppIDs := key == "appids"
			childIDs, err := vdfCollectAppIDs(br, depth+1, collectInts || isAppIDs)
			if err != nil {
				return nil, err
			}
			appIDs = append(appIDs, childIDs...)

		case bvdfInt32:
			val, err := br.readU32()
			if err != nil {
				return nil, fmt.Errorf("read int32 value: %w", err)
			}
			if collectInts {
				appIDs = append(appIDs, val)
			}

		case bvdfString:
			if _, err := br.readCString(); err != nil {
				return nil, fmt.Errorf("skip string value: %w", err)
			}

		default:
			// Types 0x03-0x07 (float32, pointer, widestring, color, uint64)
			// are rare in packageinfo but handle them by size.
			n, err := bvdfValueSize(typeByte)
			if err != nil {
				return nil, fmt.Errorf("unknown VDF type 0x%02X for key %q: %w", typeByte, key, err)
			}
			if err := br.skip(n); err != nil {
				return nil, fmt.Errorf("skip value for key %q: %w", key, err)
			}
		}
	}
}

// bvdfValueSize returns the byte size of a fixed-width binary VDF value type.
func bvdfValueSize(t byte) (int, error) {
	switch t {
	case 0x03, 0x04, 0x06: // float32, pointer, color
		return 4, nil
	case 0x07, 0x0A: // uint64, int64
		return 8, nil
	case 0x05: // widestring — variable length, not handled here
		return 0, fmt.Errorf("widestring not supported in packageinfo context")
	default:
		return 0, fmt.Errorf("unrecognized type 0x%02X", t)
	}
}

// pkgReader is a minimal binary reader for packageinfo.vdf.
type pkgReader struct {
	r   io.Reader
	buf [8]byte
}

func (r *pkgReader) readByte() (byte, error) {
	_, err := io.ReadFull(r.r, r.buf[:1])
	return r.buf[0], err
}

func (r *pkgReader) readU32() (uint32, error) {
	_, err := io.ReadFull(r.r, r.buf[:4])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(r.buf[:4]), nil
}

func (r *pkgReader) readCString() (string, error) {
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
			return "", fmt.Errorf("string exceeds 64KB limit")
		}
	}
}

func (r *pkgReader) skip(n int) error {
	// Use buf for small skips, discard for larger.
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
