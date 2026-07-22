package ingest

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
)

// maxFrameBytes — верхняя граница одного syslog-сообщения (LF и RFC6587).
const maxFrameBytes = 1024 * 1024

// errFrameTooLarge — кадр превысил maxFrameBytes; соединение следует закрыть.
var errFrameTooLarge = errors.New("ingest: frame exceeds maximum size")

// frameReader читает сообщения из TCP-потока syslog-ng.
// Поддерживает LF-delimited (legacy) и RFC6587 octet-counting framing.
type frameReader struct {
	r *bufio.Reader
}

func newFrameReader(r io.Reader) *frameReader {
	return &frameReader{r: bufio.NewReaderSize(r, 256*1024)}
}

func (fr *frameReader) ReadLine() (string, error) {
	peek, err := fr.r.Peek(1)
	if err != nil {
		return "", err
	}

	// RFC6587: "123 <payload>" или "123 <payload>\n"
	if peek[0] >= '0' && peek[0] <= '9' {
		if msg, ok, err := fr.tryOctetCounting(); ok || err != nil {
			return msg, err
		}
	}

	return fr.readLFLine()
}

func (fr *frameReader) readLFLine() (string, error) {
	var buf []byte
	for {
		chunk, err := fr.r.ReadSlice('\n')
		if len(chunk) > 0 {
			if len(buf)+len(chunk) > maxFrameBytes {
				if errors.Is(err, bufio.ErrBufferFull) {
					fr.discardThrough('\n')
				}
				return "", errFrameTooLarge
			}
			buf = append(buf, chunk...)
		}
		switch {
		case err == nil:
			return strings.TrimRight(string(buf), "\r\n"), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		default:
			if len(buf) == 0 {
				return "", err
			}
			if len(buf) > maxFrameBytes {
				return "", errFrameTooLarge
			}
			return strings.TrimRight(string(buf), "\r\n"), err
		}
	}
}

func (fr *frameReader) discardThrough(delim byte) {
	for {
		_, err := fr.r.ReadSlice(delim)
		if err == nil || !errors.Is(err, bufio.ErrBufferFull) {
			return
		}
	}
}

func (fr *frameReader) tryOctetCounting() (string, bool, error) {
	// Ограничиваем поле длины: без пробела ReadString(' ') рос бы безлимитно.
	const maxLenDigits = 10 // "1048576" и запас
	var lenField strings.Builder
	lenField.Grow(8)
	sawSpace := false
	for i := 0; i < maxLenDigits+1; i++ {
		b, err := fr.r.ReadByte()
		if err != nil {
			if lenField.Len() > 0 {
				fr.unreadPrefix(lenField.String())
			}
			return "", false, err
		}
		if b == ' ' {
			sawSpace = true
			break
		}
		if b < '0' || b > '9' {
			fr.unreadPrefix(lenField.String() + string(b))
			return "", false, nil
		}
		lenField.WriteByte(b)
		if lenField.Len() > maxLenDigits {
			fr.unreadPrefix(lenField.String())
			return "", false, nil
		}
	}
	if !sawSpace {
		fr.unreadPrefix(lenField.String())
		return "", false, nil
	}

	n, err := strconv.Atoi(lenField.String())
	if err != nil || n <= 0 || n > maxFrameBytes {
		// Не octet-counting (лог начинается с цифр, напр. IP). Вернём
		// потреблённые байты в поток и пусть ReadLine читает как LF-строку.
		fr.unreadPrefix(lenField.String() + " ")
		return "", false, nil
	}

	buf := make([]byte, n)
	if _, err := io.ReadFull(fr.r, buf); err != nil {
		return "", true, err
	}

	// syslog-ng иногда добавляет LF после payload
	if b, _ := fr.r.Peek(1); len(b) > 0 && b[0] == '\n' {
		_, _ = fr.r.ReadByte()
	}

	return string(buf), true, nil
}

func (fr *frameReader) unreadPrefix(prefix string) {
	fr.r = bufio.NewReader(io.MultiReader(strings.NewReader(prefix), fr.r))
}
