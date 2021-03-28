package ws

import (
	"bytes"
	"compress/gzip"

	"github.com/Wenchy/bifrost/internal/atom"
)

func CompressByGzip(in []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)

	// Setting the Header fields is optional.
	// zw.Name = "a-new-hope.txt"
	// zw.Comment = "an epic space opera by George Lucas"
	// zw.ModTime = time.Date(1977, time.May, 25, 0, 0, 0, 0, time.UTC)

	_, err := zw.Write(in)
	if err != nil {
		atom.Log.Errorf("gzip write failed: %s", err)
		return nil, err
	}

	if err := zw.Close(); err != nil {
		atom.Log.Errorf("gzip close failed: %s", err)
		return nil, err
	}
	return buf.Bytes(), nil
}

func DecompressByGzip(in []byte) ([]byte, error) {
	inbuf := bytes.NewReader(in)
	zr, err := gzip.NewReader(inbuf)
	if err != nil {
		atom.Log.Errorf("NewReader failed: %s", err)
		return nil, err
	}

	// fmt.Printf("Name: %s\nComment: %s\nModTime: %s\n\n", zr.Name, zr.Comment, zr.ModTime.UTC())
	var outbuf bytes.Buffer
	if _, err := outbuf.ReadFrom(zr); err != nil {
		atom.Log.Errorf("ReadFrom failed: %s", err)
		return nil, err
	}

	if err := zr.Close(); err != nil {
		atom.Log.Errorf("Close failed: %s", err)
		return nil, err
	}

	return outbuf.Bytes(), nil
}
