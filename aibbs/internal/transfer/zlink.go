package transfer

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// zlink is the shared ZMODEM framing layer (headers + data subpackets).
// Both the receiver and the sender use it so escaping and CRCs cannot drift.
type zlink struct {
	in       Stream
	pushback []byte
	bin32    bool
}

func (z *zlink) nextByte(timeout time.Duration) (byte, error) {
	if n := len(z.pushback); n > 0 {
		b := z.pushback[n-1]
		z.pushback = z.pushback[:n-1]
		return b, nil
	}
	b, err := z.in.ReadByteTimeout(timeout)
	if err != nil {
		if errors.Is(err, ErrTimeout) {
			return 0, ErrTimeoutRead
		}
		return 0, err
	}
	return b, nil
}

func (z *zlink) unread(b byte) { z.pushback = append(z.pushback, b) }

func (z *zlink) readEscaped(timeout time.Duration) (byte, error) {
	b, err := z.nextByte(timeout)
	if err != nil {
		return 0, err
	}
	if b != zdle {
		return b, nil
	}
	c, err := z.nextByte(timeout)
	if err != nil {
		return 0, err
	}
	switch c {
	case zrub0:
		return 0x7f, nil
	case zrub1:
		return 0xff, nil
	default:
		return c ^ 0x40, nil
	}
}

// readHeader synchronises on a ZMODEM header and validates its CRC.
func (z *zlink) readHeader(wait time.Duration) (uint32, uint32, error) {
	deadline := time.Now().Add(wait)
	cans := 0
	for {
		if time.Now().After(deadline) {
			return 0, 0, ErrTimeoutRead
		}
		b, err := z.nextByte(200 * time.Millisecond)
		if err != nil {
			if errors.Is(err, ErrTimeoutRead) {
				continue
			}
			return 0, 0, err
		}
		if b == zdle {
			cans++
			if cans >= 5 {
				return 0, 0, errors.New("zmodem: cancel sequence received")
			}
			continue
		}
		cans = 0
		if b != zpad {
			continue
		}
		b2, err := z.nextByte(200 * time.Millisecond)
		if err != nil {
			if errors.Is(err, ErrTimeoutRead) {
				continue
			}
			return 0, 0, err
		}
		indicator := byte(0)
		switch b2 {
		case zpad:
			b3, err := z.nextByte(500 * time.Millisecond)
			if err != nil {
				continue
			}
			if b3 != zdle {
				continue
			}
			indicator, err = z.nextByte(500 * time.Millisecond)
			if err != nil {
				continue
			}
		case zdle:
			indicator, err = z.nextByte(500 * time.Millisecond)
			if err != nil {
				continue
			}
		default:
			continue
		}
		switch indicator {
		case zhex:
			return z.readHexHeader()
		case zbin:
			z.bin32 = false
			return z.readBinaryHeader(2)
		case zbin32:
			z.bin32 = true
			return z.readBinaryHeader(4)
		}
	}
}

func (z *zlink) readHexHeader() (uint32, uint32, error) {
	raw := make([]byte, 0, 14)
	for len(raw) < 14 {
		b, err := z.nextByte(2 * time.Second)
		if err != nil {
			return 0, 0, err
		}
		if b == '\r' || b == '\n' || b == ' ' || b == 0x11 {
			continue
		}
		raw = append(raw, b)
	}
	decoded := make([]byte, 7)
	for i := 0; i < 7; i++ {
		v, err := strconv.ParseUint(string(raw[i*2:i*2+2]), 16, 8)
		if err != nil {
			return 0, 0, fmt.Errorf("zmodem: bad hex header")
		}
		decoded[i] = byte(v)
	}
	want := uint16(decoded[5])<<8 | uint16(decoded[6])
	if got := crc16CCITT(decoded[:5]); got != want {
		return 0, 0, fmt.Errorf("zmodem: hex header CRC mismatch")
	}
	z.bin32 = false
	// HEX headers are normally followed by CR LF and XON.
	for i := 0; i < 3; i++ {
		b, err := z.nextByte(50 * time.Millisecond)
		if err != nil {
			break
		}
		if b != '\r' && b != '\n' && b != 0x11 {
			z.unread(b)
			break
		}
	}
	return bytesToUint32(decoded[0:1]), bytesToUint32(decoded[1:5]), nil
}

func (z *zlink) readBinaryHeader(crcLen int) (uint32, uint32, error) {
	hdr := make([]byte, 5)
	for i := range hdr {
		b, err := z.readEscaped(2 * time.Second)
		if err != nil {
			return 0, 0, err
		}
		hdr[i] = b
	}
	crc := make([]byte, crcLen)
	for i := range crc {
		b, err := z.readEscaped(2 * time.Second)
		if err != nil {
			return 0, 0, err
		}
		crc[i] = b
	}
	if crcLen == 2 {
		want := uint16(crc[0])<<8 | uint16(crc[1])
		if got := crc16CCITT(hdr); got != want {
			return 0, 0, fmt.Errorf("zmodem: header CRC16 mismatch")
		}
	} else if got, want := crc32ZModem(hdr), bytesToUint32(crc); got != want {
		return 0, 0, fmt.Errorf("zmodem: header CRC32 mismatch")
	}
	return bytesToUint32(hdr[0:1]), bytesToUint32(hdr[1:5]), nil
}

// readSubpacket reads one ZMODEM data subpacket and verifies its CRC.
func (z *zlink) readSubpacket(timeout time.Duration) ([]byte, byte, error) {
	data := make([]byte, 0, 1024)
	var crc uint32
	writeByte := func(b byte) {
		data = append(data, b)
		crc = crc32Update(crc, []byte{b})
	}
	for {
		b, err := z.nextByte(timeout)
		if err != nil {
			return nil, 0, err
		}
		if b != zdle {
			writeByte(b)
			continue
		}
		end, err := z.nextByte(timeout)
		if err != nil {
			return nil, 0, err
		}
		switch end {
		case zcrce, zcrcg, zcrcq, zcrcw:
			crc = crc32Update(crc, []byte{end})
			crcLen := 2
			if z.bin32 {
				crcLen = 4
			}
			got := make([]byte, crcLen)
			for i := range got {
				v, err := z.readEscaped(timeout)
				if err != nil {
					return nil, 0, err
				}
				got[i] = v
			}
			if crcLen == 2 {
				want := uint16(got[0])<<8 | uint16(got[1])
				if uint16(crc) != want {
					return nil, 0, fmt.Errorf("zmodem: data CRC16 mismatch")
				}
			} else if crc != bytesToUint32(got) {
				return nil, 0, fmt.Errorf("zmodem: data CRC32 mismatch")
			}
			return data, end, nil
		case zrub0:
			writeByte(0x7f)
		case zrub1:
			writeByte(0xff)
		default:
			writeByte(end ^ 0x40)
		}
	}
}

func (z *zlink) sendHeader(frameType, data uint32, xon bool) error {
	hdr := make([]byte, 5)
	hdr[0] = byte(frameType)
	putUint32(hdr[1:], data)
	crc := crc16CCITT(hdr)
	var b strings.Builder
	b.WriteString("**")
	b.WriteByte(zdle)
	b.WriteByte(zhex)
	b.WriteString(hex.EncodeToString(hdr))
	b.WriteString(fmt.Sprintf("%04x", crc))
	b.WriteByte('\r')
	b.WriteByte('\n')
	if xon {
		b.WriteByte(0x11)
	}
	return z.in.Write([]byte(b.String()))
}

// sendBinHeader writes a ZBIN32 header, which is required before binary data
// subpackets (HEX headers imply 16 bit CRCs).
func (z *zlink) sendBinHeader(frameType, data uint32) error {
	hdr := make([]byte, 5)
	hdr[0] = byte(frameType)
	putUint32(hdr[1:], data)
	crc := crc32ZModem(hdr)
	payload := []byte{zpad, zdle, zbin32}
	payload = append(payload, z.escapeData(hdr)...)
	payload = append(payload, z.escapeData([]byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)})...)
	z.bin32 = true
	return z.in.Write(payload)
}

// sendSubpacket writes a data subpacket using the CRC width implied by the
// preceding header.
func (z *zlink) sendSubpacket(data []byte, end byte) error {
	payload := z.escapeData(data)
	payload = append(payload, zdle, end)
	if z.bin32 {
		crc := crc32ZModem(data)
		crc = crc32Update(crc, []byte{end})
		payload = append(payload, z.escapeData([]byte{byte(crc >> 24), byte(crc >> 16), byte(crc >> 8), byte(crc)})...)
	} else {
		crc := crc16CCITT(data)
		crc = crc16Update(crc, end)
		payload = append(payload, z.escapeData([]byte{byte(crc >> 8), byte(crc)})...)
	}
	return z.in.Write(payload)
}

func (z *zlink) escapeData(data []byte) []byte {
	out := make([]byte, 0, len(data)+8)
	for _, b := range data {
		switch {
		case b == zdle:
			out = append(out, zdle, zdle^0x40)
		case b == 0x7f:
			out = append(out, zdle, zrub0)
		case b == 0xff:
			out = append(out, zdle, zrub1)
		case b < 0x20:
			out = append(out, zdle, b^0x40)
		default:
			out = append(out, b)
		}
	}
	return out
}

func bytesToUint32(b []byte) uint32 {
	var v uint32
	for _, c := range b {
		v = v<<8 | uint32(c)
	}
	return v
}
