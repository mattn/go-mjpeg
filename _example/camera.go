// +build ignore

package main

import (
	"context"
	"flag"
	"image/color"
	"log"
	"net/http"
	"os"
	"os/signal"

	"github.com/mattn/go-mjpeg"

	"gocv.io/x/gocv"
)

var (
	camera = flag.Int("camera", 0, "Camera ID")
	addr   = flag.String("addr", ":8080", "Server address")
)

func main() {
	flag.Parse()

	webcam, err := gocv.VideoCaptureDevice(*camera)
	if err != nil {
		log.Println("unable to init web cam: %v", err)
		return
	}
	defer webcam.Close()

	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()
	if !classifier.Load("haarcascade_frontalface_default.xml") {
		log.Println("unable to load haarcascade_frontalface_default.xml")
		return
	}

	server := &http.Server{Addr: *addr}
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		server.Shutdown(context.Background())
	}()

	stream := mjpeg.NewStream()

	go func() {
		im := gocv.NewMat()
		for {
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
			stream.Update(buf)
		}
	}()

	http.HandleFunc("/mjpeg", stream.ServeHTTP)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src="/mjpeg" />`))
	})

	server.ListenAndServe()
}
