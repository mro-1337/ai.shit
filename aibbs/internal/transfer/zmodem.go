package transfer

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// ZMODEM wire constants.
const (
	zpad   = 0x2a // '*'
	zdle   = 0x18 // CAN, the link escape
	zbin   = 'A'
	zhex   = 'B'
	zbin32 = 'C'

	zcrce = 'h'
	zcrcg = 'i'
	zcrcq = 'j'
	zcrcw = 'k'
	zrub0 = 'l'
	zrub1 = 'm'

	zrqinit    = 0
	zrinit     = 1
	zsinit     = 2
	zack       = 3
	zfile      = 4
	zskip      = 5
	znak       = 6
	zabort     = 7
	zfin       = 8
	zrpos      = 9
	zdata      = 10
	zeof       = 11
	zferr      = 12
	zcrc       = 13
	zchallenge = 14
	zcompl     = 15
	zcan       = 16
	zfreecnt   = 17
	zcommand   = 18
	zstderr    = 19
)

const (
	// Receiver capabilities: full duplex, overlapped I/O, 32 bit FCS.
	zrinitFlags = 0x01 | 0x02 | 0x20
)

// ErrTransferTimeout means the sender stopped talking.
var ErrTransferTimeout = errors.New("transfer: timed out waiting for data")

// ErrTimeout is returned by a Stream when no byte arrived in time.
// The telnet adapter translates its own timeout into this sentinel.
var ErrTimeout = errors.New("transfer: timeout")

// ErrTimeoutRead is the internal "nothing yet" signal.
var ErrTimeoutRead = ErrTimeout

type zmReceiver struct {
	*zlink
	dir  string
	idle time.Duration
	logf Logger

	files []ReceivedFile

	cur     *os.File
	curName string
	curPath string
	curSize int64
	pos     int64
}

// ReceiveZModem downloads one or more files using ZMODEM.
func ReceiveZModem(in Stream, dir string, idle time.Duration, logf Logger) ([]ReceivedFile, error) {
	if idle <= 0 {
		idle = 60 * time.Second
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	r := &zmReceiver{zlink: &zlink{in: in}, dir: dir, idle: idle, logf: logf}
	defer r.closeCur()
	if err := r.sendHeader(zrinit, zrinitFlags, true); err != nil {
		return nil, err
	}
	logf("zmodem: sent ZRINIT")

	deadline := time.Now().Add(idle)
	for {
		if time.Now().After(deadline) {
			return r.files, ErrTransferTimeout
		}
		frame, data, err := r.readHeader(5 * time.Second)
		if err != nil {
			if errors.Is(err, ErrTimeoutRead) {
				if err := r.sendHeader(zrinit, zrinitFlags, true); err != nil {
					return r.files, err
				}
				continue
			}
			return r.files, err
		}
		deadline = time.Now().Add(idle)
		switch frame {
		case zrqinit:
			_ = r.sendHeader(zrinit, zrinitFlags, true)
		case zsinit:
			_, _, _ = r.readSubpacket(5 * time.Second)
			_ = r.sendHeader(zack, 0, false)
		case zfile:
			skip, err := r.beginFile()
			if err != nil {
				return r.files, err
			}
			if skip {
				_ = r.sendHeader(zskip, 0, true)
			} else if err := r.sendHeader(zrpos, 0, true); err != nil {
				return r.files, err
			}
		case zdata:
			if r.cur == nil {
				_ = r.sendHeader(zrpos, 0, true)
				continue
			}
			if r.pos > 0 && data != uint32(r.pos) {
				_ = r.sendHeader(zrpos, uint32(r.pos), true)
			}
			if err := r.receiveData(); err != nil {
				return r.files, err
			}
		case zeof:
			if r.cur != nil {
				if r.curSize > 0 && r.pos != r.curSize {
					r.logf("zmodem: short file %s (%d/%d), requesting resume", r.curName, r.pos, r.curSize)
					_ = r.sendHeader(zrpos, uint32(r.pos), true)
					continue
				}
				file := r.finishFile()
				r.logf("zmodem: received %s (%d bytes)", file.Name, file.Bytes)
			}
			_ = r.sendHeader(zrinit, zrinitFlags, true)
		case zfin:
			_ = r.sendHeader(zfin, 0, false)
			r.skipOverAndOut()
			return r.files, nil
		case zskip:
			r.closeCur()
		case znak:
			r.logf("zmodem: sender reported NAK")
		case zabort, zferr, zcan:
			return r.files, fmt.Errorf("zmodem: transfer aborted by sender (frame %d)", frame)
		case zack:
			// Acknowledgement; nothing to do.
		case zcrc:
			_ = r.sendHeader(zcrc, 0, true)
		case zfreecnt:
			_ = r.sendHeader(zack, 0, true)
		case zchallenge:
			_ = r.sendHeader(zack, data, true)
		case zcompl, zcommand, zstderr:
			// Nothing useful for a download.
		default:
			r.logf("zmodem: ignoring frame type %d", frame)
		}
	}
}

func (r *zmReceiver) beginFile() (bool, error) {
	payload, end, err := r.readSubpacket(10 * time.Second)
	if err != nil {
		return false, err
	}
	if end != zcrcw && end != zcrcq && end != zcrce {
		r.logf("zmodem: unexpected file info frame end %q", end)
	}
	// YMODEM block 0 style: "name\0size mtime mode serial\0"
	nul := strings.IndexByte(string(payload), 0)
	if nul < 0 {
		return false, fmt.Errorf("zmodem: malformed ZFILE info")
	}
	name := string(payload[:nul])
	rest := string(payload[nul+1:])
	if n := strings.IndexByte(rest, 0); n >= 0 {
		rest = rest[:n]
	}
	var size int64
	for _, field := range strings.Fields(rest) {
		if v, err := strconv.ParseInt(field, 10, 64); err == nil {
			size = v
			break
		}
	}
	safe, err := safeName(name)
	if err != nil {
		r.logf("zmodem: refusing unsafe filename %q", name)
		return true, nil
	}
	path, err := uniquePath(r.dir, safe)
	if err != nil {
		return false, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return false, err
	}
	r.closeCur()
	r.cur, r.curName, r.curPath = f, safe, path
	r.curSize, r.pos = size, 0
	r.logf("zmodem: incoming %s (%d bytes)", safe, size)
	return false, nil
}

func (r *zmReceiver) receiveData() error {
	for {
		data, end, err := r.readSubpacket(30 * time.Second)
		if err != nil {
			return err
		}
		if r.cur != nil && len(data) > 0 {
			if _, err := r.cur.Write(data); err != nil {
				return err
			}
			r.pos += int64(len(data))
		}
		switch end {
		case zcrcg:
			continue
		case zcrcq:
			if err := r.sendHeader(zack, uint32(r.pos), false); err != nil {
				return err
			}
			continue
		case zcrcw:
			if err := r.sendHeader(zack, uint32(r.pos), true); err != nil {
				return err
			}
			return nil
		case zcrce:
			return nil
		default:
			return fmt.Errorf("zmodem: bad frame end %q", end)
		}
	}
}

func (r *zmReceiver) finishFile() ReceivedFile {
	file := ReceivedFile{Name: r.curName, Path: r.curPath, Size: r.curSize, Bytes: r.pos}
	r.closeCur()
	r.files = append(r.files, file)
	return file
}

func (r *zmReceiver) closeCur() {
	if r.cur != nil {
		_ = r.cur.Sync()
		_ = r.cur.Close()
		r.cur = nil
	}
}

func (r *zmReceiver) skipOverAndOut() {
	// The sender follows ZFIN with "OO"; consume it if present, but do not
	// block the session if it never arrives.
	for i := 0; i < 2; i++ {
		if _, err := r.nextByte(300 * time.Millisecond); err != nil {
			return
		}
	}
}
