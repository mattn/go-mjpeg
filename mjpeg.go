package mjpeg

import (
	"image"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
)

// Decoder decode motion jpeg
type Decoder struct {
	r *multipart.Reader
	m sync.Mutex
}

// NewDecoder return new instance of Decoder
func NewDecoder(r io.Reader, b string) *Decoder {
	d := new(Decoder)
	d.r = multipart.NewReader(r, b)
	return d
}

// NewDecoderFromResponse return new instance of Decoder from http.Response
func NewDecoderFromResponse(res *http.Response) (*Decoder, error) {
	_, param, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil {
		return nil, err
	}
	return NewDecoder(res.Body, strings.Trim(param["boundary"], "-")), nil
}

// NewDecoderFromURL return new instance of Decoder from response which specified URL
func NewDecoderFromURL(u string) (*Decoder, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	return NewDecoderFromResponse(res)
}

// Decode do decoding
func (d *Decoder) Decode() (image.Image, error) {
	p, err := d.r.NextPart()
	if err != nil {
		return nil, err
	}
	return jpeg.Decode(p)
}
