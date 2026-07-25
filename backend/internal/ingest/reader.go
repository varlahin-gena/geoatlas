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

// tryOctetCounting распознаёт RFC6587 через Peek — без потребления байт,
// пока формат не подтверждён. Логи с IP ("10.0.0.1 …") не платят MultiReader.
func (fr *frameReader) tryOctetCounting() (string, bool, error) {
	const maxLenDigits = 10 // "1048576" и запас
	peek, err := fr.r.Peek(maxLenDigits + 1)
	if len(peek) == 0 {
		return "", false, err
	}

	spaceIdx := -1
	for i, b := range peek {
		if b == ' ' {
			spaceIdx = i
			break
		}
		if b < '0' || b > '9' {
			// Не octet-counting (напр. IP) — байты остаются в буфере для LF-пути.
			return "", false, nil
		}
		if i+1 > maxLenDigits {
			return "", false, nil
		}
	}
	if spaceIdx < 0 {
		// Нет пробела в peek: при EOF/ошибке — как раньше (неполный кадр).
		if err != nil {
			return "", false, err
		}
		// Слишком много цифр без пробела — LF fallback.
		return "", false, nil
	}
	if spaceIdx == 0 {
		return "", false, nil
	}

	n, convErr := strconv.Atoi(string(peek[:spaceIdx]))
	if convErr != nil || n <= 0 || n > maxFrameBytes {
		// Невалидная длина — оставляем байты, читаем как LF-строку.
		return "", false, nil
	}

	// Подтвердили framing: снимаем "N " и читаем ровно n байт payload.
	if _, discErr := fr.r.Discard(spaceIdx + 1); discErr != nil {
		return "", true, discErr
	}

	buf := make([]byte, n)
	if _, readErr := io.ReadFull(fr.r, buf); readErr != nil {
		return "", true, readErr
	}

	// syslog-ng иногда добавляет LF после payload
	if b, _ := fr.r.Peek(1); len(b) > 0 && b[0] == '\n' {
		_, _ = fr.r.ReadByte()
	}

	return string(buf), true, nil
}
