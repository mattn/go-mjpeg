package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/mattn/go-mjpeg"
)

var url = flag.String("url", "", "Camera host")
var addr = flag.String("addr", ":8080", "Server address")

func main() {
	flag.Parse()
	if *url == "" {
		flag.Usage()
		os.Exit(1)
	}

	runtime.GOMAXPROCS(runtime.NumCPU())

	req, err := http.NewRequest("GET", *url, nil)
	if err != nil {
		log.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	_, param, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
	if err != nil {
		log.Fatal(err)
	}
	boundary := param["boundary"]
	if !strings.HasPrefix(boundary, "--") {
		log.Fatal("boundary should be started with `--`")
	}
	dec := mjpeg.NewDecoder(res.Body, boundary[2:])

	var mutex sync.Mutex
	var img image.Image

	log.Println("Start streaming")
	go func() {
		for {
			var tmp image.Image
			err = dec.Decode(&tmp)
			if err != nil {
				break
			}
			mutex.Lock()
			img = tmp
			mutex.Unlock()
		}
	}()

	http.HandleFunc("/jpeg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		mutex.Lock()
		err = jpeg.Encode(w, img, nil)
		mutex.Unlock()
	})

	http.HandleFunc("/mjpeg", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serve streaming")
		m := multipart.NewWriter(w)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
		w.Header().Set("Connection", "close")
		header := textproto.MIMEHeader{}
		var buf bytes.Buffer
		for {
			mutex.Lock()
			buf.Reset()
			err = jpeg.Encode(&buf, img, nil)
			mutex.Unlock()
			if err != nil {
				break
			}
			header.Set("Content-Type", "image/jpeg")
			header.Set("Content-Length", fmt.Sprint(buf.Len()))
			mw, err := m.CreatePart(header)
			if err != nil {
				break
			}
			mw.Write(buf.Bytes())
			if flusher, ok := mw.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		log.Println("Stop streaming")
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src="/mjpeg" />`))
	})

	http.ListenAndServe(*addr, nil)
}
