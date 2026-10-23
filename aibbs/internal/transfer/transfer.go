// Package transfer implements the file transfer protocols BBSes use:
// ZMODEM (the common case), XMODEM-CRC/1K-XMODEM and YMODEM batch.
//
// The receivers are driven synchronously by the session: after the BBS says
// "start your transfer", the session hands the raw telnet stream to one of
// these receivers.
package transfer

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Stream is the byte pipe a receiver talks over. telnet.Client satisfies it.
type Stream interface {
	ReadByteTimeout(timeout time.Duration) (byte, error)
	Write([]byte) error
}

// ReceivedFile describes a file that was successfully received.
type ReceivedFile struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Bytes int64  `json:"bytes"`
}

// Logger receives human readable progress messages.
type Logger func(format string, args ...any)

var (
	crc32Table = crc32.MakeTable(crc32.IEEE)
)

func crc32Update(crc uint32, data []byte) uint32 {
	return crc32.Update(crc, crc32Table, data)
}

// crc16Update continues a CRC-16-CCITT over one byte.
func crc16Update(crc uint16, b byte) uint16 {
	crc ^= uint16(b) << 8
	for i := 0; i < 8; i++ {
		if crc&0x8000 != 0 {
			crc = (crc << 1) ^ 0x1021
		} else {
			crc <<= 1
		}
	}
	return crc
}

// crc16CCITT implements the CRC-16 used by XMODEM/YMODEM and ZMODEM headers.
func crc16CCITT(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// zmodem CRC-32 starts at zero (not 0xFFFFFFFF) and has no final xor.
func crc32ZModem(data []byte) uint32 {
	return crc32.Checksum(data, crc32Table)
}

func putUint32(b []byte, v uint32) {
	binary.BigEndian.PutUint32(b, v)
}

func safeName(name string) (string, error) {
	// Strip any path components: a BBS must never choose where we write.
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", fmt.Errorf("unsafe remote filename %q", name)
	}
	clean := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r < 32, r == '/', r == '\\', r == ':':
			clean = append(clean, '_')
		default:
			clean = append(clean, r)
		}
	}
	out := string(clean)
	if len(out) > 120 {
		out = out[:120]
	}
	return out, nil
}

// uniquePath returns a path inside dir that does not clobber an existing file.
func uniquePath(dir, name string) (string, error) {
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); err != nil {
		return p, nil
	}
	ext := filepath.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	for i := 2; i < 10000; i++ {
		cand := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(cand); err != nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("no free filename for %q", name)
}
