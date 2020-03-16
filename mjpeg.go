package mjpeg

import (
	"errors"
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
	m        sync.Mutex
	s        map[chan []byte]struct{}
	Interval time.Duration
}

func NewStream() *Stream {
	return &Stream{
		s:        make(map[chan []byte]struct{}),
		Interval: time.Millisecond,
	}
}

func NewStreamWithInterval(interval time.Duration) *Stream {
	return &Stream{
		s:        make(map[chan []byte]struct{}),
		Interval: interval,
	}
}

func (s *Stream) Close() error {
	s.m.Lock()
	defer s.m.Unlock()
	for c := range s.s {
		close(c)
		delete(s.s, c)
	}
	s.s = nil
	return nil
}

func (s *Stream) Update(b []byte) error {
	s.m.Lock()
	defer s.m.Unlock()
	if s.s == nil {
		return errors.New("stream was closed")
	}
	for c := range s.s {
		select {
		case c <- b:
		default:
		}
	}
	return nil
}

func (s *Stream) add(c chan []byte) {
	s.m.Lock()
	s.s[c] = struct{}{}
	s.m.Unlock()
}

func (s *Stream) destroy(c chan []byte) {
	s.m.Lock()
	if s.s != nil {
		close(c)
		delete(s.s, c)
	}
	s.m.Unlock()
}

func (s *Stream) NWatch() int {
	return len(s.s)
}

func (s *Stream) Current() []byte {
	c := make(chan []byte)
	s.add(c)
	defer s.destroy(c)

	return <-c
}

func (s *Stream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := make(chan []byte)
	s.add(c)
	defer s.destroy(c)

	m := multipart.NewWriter(w)
	defer m.Close()

	defer r.Body.Close()

	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
	w.Header().Set("Connection", "close")
	h := textproto.MIMEHeader{}
	st := fmt.Sprint(time.Now().Unix())
	timeout := time.Now()
	for {
		time.Sleep(s.Interval)

		select {
		case b, ok := <-c:
			if !ok {
				return
			}
			timeout = time.Now()
			h.Set("Content-Type", "image/jpeg")
			h.Set("Content-Length", fmt.Sprint(len(b)))
			h.Set("X-StartTime", st)
			h.Set("X-TimeStamp", fmt.Sprint(time.Now().Unix()))
			mw, err := m.CreatePart(h)
			if err != nil {
				return
			}
			_, err = mw.Write(b)
			if err != nil {
				return
			}
			if flusher, ok := mw.(http.Flusher); ok {
				flusher.Flush()
			}
		default:
			if time.Since(timeout) > time.Minute {
				return
			}
		}
	}
}
