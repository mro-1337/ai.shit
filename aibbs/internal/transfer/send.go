package transfer

import (
	"fmt"
	"time"
)

// OutFile is a file offered to a sender.
type OutFile struct {
	Name string
	Data []byte
}

// waitHeader waits for one of the accepted frame types.
func (z *zlink) waitHeader(wait time.Duration, accept ...uint32) (uint32, uint32, error) {
	deadline := time.Now().Add(wait)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return 0, 0, ErrTimeoutRead
		}
		frame, data, err := z.readHeader(remaining)
		if err != nil {
			return 0, 0, err
		}
		for _, a := range accept {
			if frame == a {
				return frame, data, nil
			}
		}
	}
}

// SendZModem uploads files using ZMODEM. It is used by the test BBS and is
// available for future upload support.
func SendZModem(in Stream, files []OutFile, idle time.Duration, logf Logger) error {
	if idle <= 0 {
		idle = 30 * time.Second
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	z := &zlink{in: in}
	// The receiver starts the session with ZRINIT.
	if _, _, err := z.waitHeader(idle, zrinit); err != nil {
		return fmt.Errorf("zmodem send: waiting for ZRINIT: %w", err)
	}

	for _, file := range files {
		if err := sendOneFile(z, file, idle, logf); err != nil {
			return err
		}
	}

	if err := z.sendHeader(zfin, 0, false); err != nil {
		return err
	}
	if _, _, err := z.waitHeader(5*time.Second, zfin); err == nil {
		_ = z.in.Write([]byte("OO"))
	}
	logf("zmodem: session complete")
	return nil
}

func sendOneFile(z *zlink, file OutFile, idle time.Duration, logf Logger) error {
	logf("zmodem: offering %s (%d bytes)", file.Name, len(file.Data))
	for attempt := 0; attempt < 3; attempt++ {
		if err := z.sendBinHeader(zfile, 1); err != nil { // ZCBIN
			return err
		}
		info := []byte(file.Name)
		info = append(info, 0)
		info = append(info, []byte(fmt.Sprintf("%d 0 0 0", len(file.Data)))...)
		info = append(info, 0)
		if err := z.sendSubpacket(info, zcrcw); err != nil {
			return err
		}
		frame, offset, err := z.waitHeader(idle, zrpos, zskip, znak)
		if err != nil {
			return err
		}
		if frame == zskip {
			logf("zmodem: receiver skipped %s", file.Name)
			return nil
		}
		if frame == znak {
			logf("zmodem: receiver NAKed the file header, retrying")
			continue
		}
		if offset != 0 {
			logf("zmodem: receiver asked for offset %d", offset)
		}
		if len(file.Data) == 0 {
			if err := z.sendHeader(zeof, 0, true); err != nil {
				return err
			}
		} else {
			if err := z.sendBinHeader(zdata, offset); err != nil {
				return err
			}
			const chunkSize = 1024
			pos := int(offset)
			for pos < len(file.Data) {
				end := pos + chunkSize
				if end > len(file.Data) {
					end = len(file.Data)
				}
				frameEnd := byte(zcrcg)
				if end == len(file.Data) {
					frameEnd = zcrcw
				}
				if err := z.sendSubpacket(file.Data[pos:end], frameEnd); err != nil {
					return err
				}
				pos = end
			}
			if _, _, err := z.waitHeader(idle, zack, zrpos); err != nil {
				return err
			}
			if err := z.sendHeader(zeof, uint32(len(file.Data)), true); err != nil {
				return err
			}
		}
		if _, _, err := z.waitHeader(idle, zrinit, zrpos); err != nil {
			return err
		}
		logf("zmodem: sent %s", file.Name)
		return nil
	}
	return fmt.Errorf("zmodem send: %s was repeatedly rejected", file.Name)
}

// SendXModem uploads files using XMODEM-CRC or YMODEM batch.
func SendXModem(in Stream, files []OutFile, ymodem bool, idle time.Duration, logf Logger) error {
	if idle <= 0 {
		idle = 30 * time.Second
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	// Wait for the receiver's start character ('C' for CRC, NAK for checksum).
	var start byte
	deadline := time.Now().Add(idle)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("xmodem send: receiver never started")
		}
		b, err := in.ReadByteTimeout(500 * time.Millisecond)
		if err != nil {
			if err == ErrTimeout {
				continue
			}
			return err
		}
		if b == xCRC || b == xNAK {
			start = b
			break
		}
	}
	crcMode := start == xCRC
	blockNo := byte(1)

	for _, f := range files {
		if ymodem {
			info := []byte(f.Name)
			info = append(info, 0)
			info = append(info, []byte(fmt.Sprintf("%d 0 0 0", len(f.Data)))...)
			info = append(info, 0)
			if err := sendXBlock(in, 0, info, 128, 0x00, crcMode, idle); err != nil {
				return err
			}
			blockNo = 1
		}
		for off := 0; off < len(f.Data); off += 128 {
			end := off + 128
			if end > len(f.Data) {
				end = len(f.Data)
			}
			if err := sendXBlock(in, blockNo, f.Data[off:end], 128, 0x1a, crcMode, idle); err != nil {
				return err
			}
			blockNo++
		}
		if err := sendXEOT(in, idle); err != nil {
			return err
		}
	}
	if ymodem {
		// Empty block 0 terminates a YMODEM batch.
		if err := sendXBlock(in, 0, nil, 128, 0x00, crcMode, idle); err != nil {
			return err
		}
	}
	logf("xmodem: sent %d file(s)", len(files))
	return nil
}

func sendXEOT(in Stream, idle time.Duration) error {
	for attempt := 0; attempt < 10; attempt++ {
		if err := in.Write([]byte{xEOT}); err != nil {
			return err
		}
		b, err := in.ReadByteTimeout(idle)
		if err != nil {
			if err == ErrTimeout {
				continue
			}
			return err
		}
		if b == xACK {
			return nil
		}
	}
	return fmt.Errorf("xmodem: EOT was never acknowledged")
}

func sendXBlock(in Stream, blockNo byte, data []byte, size int, pad byte, crcMode bool, idle time.Duration) error {
	payload := make([]byte, size)
	for i := range payload {
		payload[i] = pad
	}
	copy(payload, data)
	block := []byte{xSOH, blockNo, ^blockNo}
	block = append(block, payload...)
	if crcMode {
		crc := crc16CCITT(payload)
		block = append(block, byte(crc>>8), byte(crc))
	} else {
		var sum byte
		for _, b := range payload {
			sum += b
		}
		block = append(block, sum)
	}
	for attempt := 0; attempt < 10; attempt++ {
		if err := in.Write(block); err != nil {
			return err
		}
		b, err := in.ReadByteTimeout(idle)
		if err != nil {
			if err == ErrTimeout {
				continue
			}
			return err
		}
		switch b {
		case xACK:
			return nil
		case xCAN:
			return fmt.Errorf("xmodem: receiver cancelled")
		}
	}
	return fmt.Errorf("xmodem: block %d was never acknowledged", blockNo)
}
