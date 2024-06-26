package base

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io/ioutil"
)

// Compress gzips a string and base64 encodes it
func Compress(s string) (string, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)

	_, err := gz.Write([]byte(s))
	if err != nil {
		return "", err
	}

	err = gz.Flush()
	if err != nil {
		return "", err
	}

	err = gz.Close()
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

// Uncompress uncompresses a string
func Uncompress(s string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}

	rdata := bytes.NewReader(decoded)
	r, err := gzip.NewReader(rdata)
	if err != nil {
		return "", err
	}

	decompressed, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}

	return string(decompressed), nil
}
