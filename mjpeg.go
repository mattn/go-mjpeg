package mjpeg

import (
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"sync"
)

type Decoder struct {
	r *multipart.Reader
	m sync.Mutex
}

func NewDecoder(r io.Reader, b string) *Decoder {
	d := new(Decoder)
	d.r = multipart.NewReader(r, b)
	return d
}

func (d *Decoder) Decode(img *image.Image) error {
	p, err := d.r.NextPart()
	if err != nil {
		return err
	}
	tmp, err := jpeg.Decode(p)
	if err != nil {
		return err
	}
	d.m.Lock()
	*img = tmp
	d.m.Unlock()
	return nil
}
