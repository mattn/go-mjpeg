package mjpeg

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"sync"
	"time"
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

type Stream struct {
	m sync.Mutex
	s map[chan []byte]struct{}
}

func NewStream() *Stream {
	return &Stream{
		s: make(map[chan []byte]struct{}),
	}
}

func (s *Stream) Close() error {
	s.m.Lock()
	defer s.m.Unlock()
	for c := range s.s {
		close(c)
		delete(s.s, c)
	}
	return nil
}

func (s *Stream) Update(b []byte) {
	s.m.Lock()
	defer s.m.Unlock()
	for c := range s.s {
		select {
		case c <- b:
		default:
		}
	}
}

func (s *Stream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := make(chan []byte)
	s.m.Lock()
	s.s[c] = struct{}{}
	s.m.Unlock()

	defer func() {
		close(c)
		s.m.Lock()
		delete(s.s, c)
		s.m.Unlock()
	}()

	m := multipart.NewWriter(w)
	defer m.Close()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
	w.Header().Set("Connection", "close")
	h := textproto.MIMEHeader{}
	st := fmt.Sprint(time.Now().Unix())
	for {
		b, ok := <-c
		if !ok {
			break
		}
		h.Set("Content-Type", "image/jpeg")
		h.Set("Content-Length", fmt.Sprint(len(b)))
		h.Set("X-StartTime", st)
		h.Set("X-TimeStamp", fmt.Sprint(time.Now().Unix()))
		mw, err := m.CreatePart(h)
		if err != nil {
			break
		}
		_, err = mw.Write(b)
		if err != nil {
			break
		}
		if flusher, ok := mw.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
