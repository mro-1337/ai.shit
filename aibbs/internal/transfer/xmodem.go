package transfer

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// XMODEM control bytes.
const (
	xSOH = 0x01
	xSTX = 0x02
	xEOT = 0x04
	xACK = 0x06
	xNAK = 0x15
	xCAN = 0x18
	xCRC = 'C' // request CRC mode
)

// XModemOptions configures an XMODEM/YMODEM receive.
type XModemOptions struct {
	YModem   bool
	Timeout  time.Duration
	MaxBytes int64
}

// ReceiveXModem downloads files using XMODEM-CRC, 1K-XMODEM or YMODEM batch.
func ReceiveXModem(in Stream, dir string, opts XModemOptions, logf Logger) ([]ReceivedFile, error) {
	if opts.Timeout <= 0 {
		opts.Timeout = 10 * time.Second
	}
	if opts.MaxBytes <= 0 {
		opts.MaxBytes = 64 << 20
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	r := &xReceiver{in: in, dir: dir, opts: opts, logf: logf}
	if err := r.run(); err != nil {
		r.closeCur()
		return r.files, err
	}
	return r.files, nil
}

type xReceiver struct {
	in   Stream
	dir  string
	opts XModemOptions
	logf Logger

	files []ReceivedFile
	cur   *os.File
	name  string
	path  string
	size  int64
	pos   int64
}

func (r *xReceiver) run() error {
	// Ask for CRC mode; some older BBSes only speak checksum mode, so fall
	// back after repeated silence rather than failing outright.
	retries := 0
	useCRC := true
	wantBlock := byte(1)
	if r.opts.YModem {
		wantBlock = 0
	}
	start := byte(xCRC)
	if err := r.in.Write([]byte{start}); err != nil {
		return err
	}
	for {
		b, err := r.in.ReadByteTimeout(r.opts.Timeout)
		if err != nil {
			if errors.Is(err, ErrTimeout) {
				retries++
				if retries >= 10 {
					return fmt.Errorf("xmodem: no response from sender")
				}
				if useCRC {
					start = xCRC
				} else {
					start = xNAK
				}
				if err := r.in.Write([]byte{start}); err != nil {
					return err
				}
				continue
			}
			return err
		}
		switch b {
		case xCAN:
			// Two CANs abort; a single CAN is treated as noise.
			if b2, err := r.in.ReadByteTimeout(200 * time.Millisecond); err == nil && b2 == xCAN {
				return fmt.Errorf("xmodem: sender cancelled")
			}
			continue
		case xEOT:
			if r.cur != nil {
				r.finishFile()
			}
			if err := r.in.Write([]byte{xACK}); err != nil {
				return err
			}
			if !r.opts.YModem {
				return nil
			}
			// YMODEM: the sender now emits an empty block 0, then we are done.
			done, err := r.readYModemPrologue(nil)
			if err != nil {
				return err
			}
			if done {
				return nil
			}
			continue
		case xSOH, xSTX:
			blockSize := 128
			if b == xSTX {
				blockSize = 1024
			}
			payload, blockNo, err := r.readBlock(blockSize, useCRC)
			if err != nil {
				if errors.Is(err, errCRC) {
					useCRC = false
					_ = r.in.Write([]byte{xNAK})
					retries++
					if retries > 10 {
						return fmt.Errorf("xmodem: too many CRC errors")
					}
					continue
				}
				return err
			}
			if r.opts.YModem && wantBlock == 0 {
				if blockNo != 0 {
					_ = r.in.Write([]byte{xNAK})
					continue
				}
				done, err := r.readYModemPrologue(payload)
				if err != nil {
					return err
				}
				if done {
					return nil
				}
				wantBlock = 1
				_ = r.in.Write([]byte{xACK})
				retries = 0
				continue
			}
			if blockNo != wantBlock {
				// Duplicate block (our ACK was lost): acknowledge again.
				_ = r.in.Write([]byte{xACK})
				continue
			}
			if err := r.writeData(payload); err != nil {
				return err
			}
			if err := r.in.Write([]byte{xACK}); err != nil {
				return err
			}
			wantBlock++
			retries = 0
		default:
			// Line noise / leftover menu text: ignore.
		}
	}
}

func (r *xReceiver) readBlock(size int, useCRC bool) ([]byte, byte, error) {
	blockNo, err := r.readByteTimeout()
	if err != nil {
		return nil, 0, err
	}
	inv, err := r.readByteTimeout()
	if err != nil {
		return nil, 0, err
	}
	if blockNo^0xff != inv {
		return nil, 0, errCRC
	}
	data := make([]byte, size)
	for i := range data {
		b, err := r.readByteTimeout()
		if err != nil {
			return nil, 0, err
		}
		data[i] = b
	}
	if useCRC {
		hi, err := r.readByteTimeout()
		if err != nil {
			return nil, 0, err
		}
		lo, err := r.readByteTimeout()
		if err != nil {
			return nil, 0, err
		}
		want := uint16(hi)<<8 | uint16(lo)
		if got := crc16CCITT(data); got != want {
			return nil, 0, errCRC
		}
	} else {
		sum, err := r.readByteTimeout()
		if err != nil {
			return nil, 0, err
		}
		var calc byte
		for _, b := range data {
			calc += b
		}
		if calc != sum {
			return nil, 0, errCRC
		}
	}
	return data, blockNo, nil
}

func (r *xReceiver) readByteTimeout() (byte, error) {
	b, err := r.in.ReadByteTimeout(r.opts.Timeout)
	if err != nil {
		if errors.Is(err, ErrTimeout) {
			return 0, errCRC
		}
		return 0, err
	}
	return b, nil
}

func (r *xReceiver) readYModemPrologue(payload []byte) (bool, error) {
	if payload == nil {
		// Expect one more block header (empty block 0).
		head, err := r.readByteTimeout()
		if err != nil {
			return false, err
		}
		size := 128
		if head == xSTX {
			size = 1024
		} else if head != xSOH {
			return false, fmt.Errorf("xmodem: expected empty YMODEM header")
		}
		p, _, err := r.readBlock(size, true)
		if err != nil {
			return false, err
		}
		payload = p
	}
	nul := strings.IndexByte(string(payload), 0)
	if nul <= 0 {
		_ = r.in.Write([]byte{xACK})
		return true, nil
	}
	name := string(payload[:nul])
	rest := string(payload[nul+1:])
	if n := strings.IndexByte(rest, 0); n >= 0 {
		rest = rest[:n]
	}
	var size int64
	if fields := strings.Fields(rest); len(fields) > 0 {
		size, _ = strconv.ParseInt(fields[0], 10, 64)
	}
	if size > r.opts.MaxBytes {
		return false, fmt.Errorf("xmodem: %s is too large (%d bytes)", name, size)
	}
	safe, err := safeName(name)
	if err != nil {
		return false, err
	}
	path, err := uniquePath(r.dir, safe)
	if err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return false, err
	}
	r.closeCur()
	r.cur, r.name, r.path, r.size, r.pos = f, safe, path, size, 0
	r.logf("xmodem: incoming %s (%d bytes)", safe, size)
	return false, nil
}

func (r *xReceiver) writeData(data []byte) error {
	if r.cur == nil {
		// Plain XMODEM has no filename header; invent one.
		path, err := uniquePath(r.dir, "xmodem-download.bin")
		if err != nil {
			return err
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err != nil {
			return err
		}
		r.cur, r.name, r.path = f, "xmodem-download.bin", path
	}
	length := len(data)
	if r.size > 0 && r.pos+int64(length) > r.size {
		length = int(r.size - r.pos)
	}
	if length <= 0 {
		return nil
	}
	// XMODEM pads the final block with 0x1A (CP/M EOF); trim by size if known.
	if _, err := r.cur.Write(data[:length]); err != nil {
		return err
	}
	r.pos += int64(length)
	return nil
}

func (r *xReceiver) finishFile() {
	if r.size == 0 && r.cur != nil {
		// No length header: strip XMODEM's CP/M EOF padding.
		trimmed := r.pos
		for trimmed > 0 {
			b, err := readAt(r.cur, trimmed-1)
			if err != nil || b != 0x1a {
				break
			}
			trimmed--
		}
		if trimmed != r.pos {
			_ = r.cur.Truncate(trimmed)
			r.pos = trimmed
		}
	}
	file := ReceivedFile{Name: r.name, Path: r.path, Size: r.size, Bytes: r.pos}
	r.closeCur()
	r.files = append(r.files, file)
	r.logf("xmodem: received %s (%d bytes)", file.Name, file.Bytes)
}

func (r *xReceiver) closeCur() {
	if r.cur != nil {
		_ = r.cur.Sync()
		_ = r.cur.Close()
		r.cur = nil
	}
}

var errCRC = errors.New("xmodem: crc mismatch")

func readAt(f *os.File, off int64) (byte, error) {
	var b [1]byte
	if _, err := f.ReadAt(b[:], off); err != nil {
		return 0, err
	}
	return b[0], nil
}
