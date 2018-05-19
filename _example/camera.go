// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"log"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"os/signal"
	"sync"

	"gocv.io/x/gocv"
)

var (
	camera = flag.Int("camera", 0, "Camera ID")
	addr   = flag.String("addr", ":8080", "Server address")
)

func main() {
	flag.Parse()

	webcam, err := gocv.VideoCaptureDevice(camera)
	if err != nil {
		log.Println("unable to init web cam: %v", err)
		return
	}
	defer webcam.Close()

	var mutex sync.RWMutex
	var img image.Image

	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()
	if !classifier.Load("haarcascade_frontalface_default.xml") {
		log.Println("unable to load haarcascade_frontalface_default.xml")
		return
	}

	loop := true
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		loop = false
	}()

	log.Println("Start streaming")
	go func() {
		im := gocv.NewMat()
		for loop {
			if ok := webcam.Read(&im); !ok {
				continue
			}

			rects := classifier.DetectMultiScale(im)
			for _, r := range rects {
				face := im.Region(r)
				face.Close()
				gocv.Rectangle(&im, r, color.RGBA{0, 0, 255, 0}, 2)
			}
			buf, err := gocv.IMEncode(".jpg", im)
			if err != nil {
				continue
			}
			mutex.Lock()
			if tmp, err := jpeg.Decode(bytes.NewReader(buf)); err == nil {
				img = tmp
			}
			mutex.Unlock()
		}
	}()

	http.HandleFunc("/mjpeg", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Serve streaming")
		m := multipart.NewWriter(w)
		w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
		w.Header().Set("Connection", "close")
		header := textproto.MIMEHeader{}
		var buf bytes.Buffer
		for loop {
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
