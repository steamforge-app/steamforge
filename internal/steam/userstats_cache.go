package steam

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

// LocalAchievementCount holds counts parsed from a local UserGameStats binary file.
type LocalAchievementCount struct {
	Achieved int
	Total    int // from schema; -1 if schema unavailable
}

// userStatsFileRe matches UserGameStats_{accountID}_{appID}.bin filenames.
var userStatsFileRe = regexp.MustCompile(`^UserGameStats_(\d+)_(\d+)\.bin$`)

// ScanLocalAchievementCounts reads all UserGameStats_{accountID}_{appID}.bin files
// from the appcache/stats directory and returns the achieved count for each app.
// Only returns entries where the file was parsed successfully and had achievement data.
func ScanLocalAchievementCounts(steamID uint64) map[uint32]int {
	installPath, err := GetInstallPath()
	if err != nil {
		slog.Debug("cannot scan local stats: install path", "error", err)
		return nil
	}

	accountID := uint32(steamID & 0xFFFFFFFF)
	statsDir := filepath.Join(installPath, "appcache", "stats")

	entries, err := os.ReadDir(statsDir)
	if err != nil {
		slog.Debug("cannot read stats dir", "path", statsDir, "error", err)
		return nil
	}

	prefix := fmt.Sprintf("UserGameStats_%d_", accountID)
	result := make(map[uint32]int)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || len(name) < len(prefix) {
			continue
		}

		m := userStatsFileRe.FindStringSubmatch(name)
		if m == nil {
			continue
		}

		fileAccountID, _ := strconv.ParseUint(m[1], 10, 32)
		if uint32(fileAccountID) != accountID {
			continue
		}

		appID, _ := strconv.ParseUint(m[2], 10, 32)
		if appID == 0 {
			continue
		}

		filePath := filepath.Join(statsDir, name)
		achieved, err := parseUserStatsAchievedCount(filePath)
		if err != nil {
			slog.Debug("skip user stats file", "file", name, "error", err)
			continue
		}

		result[uint32(appID)] = achieved
	}

	slog.Info("local stats scan complete", "files_parsed", len(result))
	return result
}

// parseUserStatsAchievedCount opens a UserGameStats binary file and counts the
// number of entries in the AchievementTimes section.
func parseUserStatsAchievedCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	// Empty cache files are ~38 bytes with no achievement data.
	if info.Size() < 40 {
		return 0, nil
	}

	br := &statsReader{r: f}
	return countAchievementTimes(br)
}

// countAchievementTimes walks the binary VDF tree looking for the
// "AchievementTimes" object and counts its child entries.
// Uses standard binary VDF type codes (0x02 = Int32).
func countAchievementTimes(br *statsReader) (int, error) {
	return walkVDFForAchievements(br, 0, false)
}

func walkVDFForAchievements(br *statsReader, depth int, inAchTimes bool) (int, error) {
	if depth > 32 {
		return 0, fmt.Errorf("VDF nesting too deep")
	}

	count := 0
	for {
		typeByte, err := br.readByte()
		if err != nil {
			if err == io.EOF {
				return count, nil
			}
			return count, err
		}
		if typeByte == 0x08 || typeByte == 0x0B { // End / AlternateEnd
			return count, nil
		}

		key, err := br.readCString()
		if err != nil {
			return count, fmt.Errorf("read key at depth %d: %w", depth, err)
		}

		switch typeByte {
		case 0x00: // Nested object
			isAchTimes := key == "AchievementTimes"
			childCount, err := walkVDFForAchievements(br, depth+1, inAchTimes || isAchTimes)
			if err != nil {
				return count, err
			}
			count += childCount

		case 0x02: // Int32
			_, err := br.readU32()
			if err != nil {
				return count, fmt.Errorf("read int32 value: %w", err)
			}
			if inAchTimes {
				count++
			}

		case 0x01: // String
			if _, err := br.readCString(); err != nil {
				return count, fmt.Errorf("skip string value: %w", err)
			}

		default:
			n, err := fixedValueSize(typeByte)
			if err != nil {
				return count, fmt.Errorf("unknown VDF type 0x%02X for key %q: %w", typeByte, key, err)
			}
			if err := br.skip(n); err != nil {
				return count, fmt.Errorf("skip value for key %q: %w", key, err)
			}
		}
	}
}

func fixedValueSize(t byte) (int, error) {
	switch t {
	case 0x03, 0x04, 0x06: // float32, pointer, color
		return 4, nil
	case 0x07, 0x0A: // uint64, int64
		return 8, nil
	default:
		return 0, fmt.Errorf("unrecognized type 0x%02X", t)
	}
}

// statsReader is a minimal binary reader for user stats files.
type statsReader struct {
	r   io.Reader
	buf [8]byte
}

func (r *statsReader) readByte() (byte, error) {
	_, err := io.ReadFull(r.r, r.buf[:1])
	return r.buf[0], err
}

func (r *statsReader) readU32() (uint32, error) {
	_, err := io.ReadFull(r.r, r.buf[:4])
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(r.buf[:4]), nil
}

func (r *statsReader) readCString() (string, error) {
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

func (r *statsReader) skip(n int) error {
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
