package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"runtime"
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

	dec, err := mjpeg.NewDecoderFromURL(*url)
	if err != nil {
		log.Fatal(err)
	}

	var mutex sync.RWMutex
	var img image.Image

	log.Println("Start streaming")
	go func() {
		for {
			decodedImage, err := dec.Decode()
			if err != nil {
				break
			}
			mutex.Lock()
			img = decodedImage
			mutex.Unlock()
		}
	}()

	http.HandleFunc("/jpeg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		mutex.RLock()
		err = jpeg.Encode(w, img, nil)
		mutex.RUnlock()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	http.HandleFunc("/mjpeg", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serve streaming")
		m := multipart.NewWriter(w)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
		w.Header().Set("Connection", "close")
		header := textproto.MIMEHeader{}
		var buf bytes.Buffer
		for {
			mutex.RLock()
			if img == nil {
				http.Error(w, "Not Found", 404)
				return
			}
			buf.Reset()
			err = jpeg.Encode(&buf, img, nil)
			mutex.RUnlock()
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
