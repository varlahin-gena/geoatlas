package ingestnet

import (
	"bytes"
	"io"
	"testing"
)

func FuzzFrameReader(f *testing.F) {
	f.Add([]byte("hello\n"))
	f.Add([]byte("5 hello"))
	f.Add([]byte("5 hello\n"))
	f.Add([]byte("0 \n"))
	f.Add([]byte("999999999 payload that claims huge length"))
	f.Add([]byte("10.0.0.1 message without framing\n"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		fr := newFrameReader(bytes.NewReader(data))
		for i := 0; i < 64; i++ {
			_, err := fr.ReadLine()
			if err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return
				}
				return
			}
		}
	})
}
