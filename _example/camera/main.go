package main

import (
	"context"
	"flag"
	"image/color"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"time"

	"github.com/mattn/go-mjpeg"

	"gocv.io/x/gocv"
)

var (
	camera   = flag.String("camera", "0", "Camera ID")
	addr     = flag.String("addr", ":8080", "Server address")
	xml      = flag.String("classifier", "haarcascade_frontalface_default.xml", "classifier XML file")
	interval = flag.Duration("interval", 200*time.Millisecond, "interval")
)

func capture(ctx context.Context, wg *sync.WaitGroup, stream *mjpeg.Stream) {
	defer wg.Done()

	var webcam *gocv.VideoCapture
	var err error
	if id, err := strconv.ParseInt(*camera, 10, 64); err == nil {
		webcam, err = gocv.VideoCaptureDevice(int(id))
	} else {
		webcam, err = gocv.VideoCaptureFile(*camera)
	}
	if err != nil {
		log.Println("unable to init web cam: %v", err)
		return
	}
	defer webcam.Close()

	classifier := gocv.NewCascadeClassifier()
	defer classifier.Close()
	if !classifier.Load(*xml) {
		log.Println("unable to load:", *xml)
		return
	}

	im := gocv.NewMat()

	for len(ctx.Done()) == 0 {
		var buf []byte
		if stream.NWatch() > 0 {
			if ok := webcam.Read(&im); !ok {
				continue
			}

			rects := classifier.DetectMultiScale(im)
			for _, r := range rects {
				face := im.Region(r)
				face.Close()
				gocv.Rectangle(&im, r, color.RGBA{0, 0, 255, 0}, 2)
			}
			nbuf, err := gocv.IMEncode(".jpg", im)
			if err != nil {
				continue
			}
			buf = nbuf.GetBytes()
		}
		err = stream.Update(buf)
		if err != nil {
			break
		}
	}
}

func main() {
	flag.Parse()

	stream := mjpeg.NewStreamWithInterval(*interval)

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go capture(ctx, &wg, stream)

	http.HandleFunc("/mjpeg", stream.ServeHTTP)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src="/mjpeg" />`))
	})

	server := &http.Server{Addr: *addr}
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		server.Shutdown(ctx)
	}()
	server.ListenAndServe()
	stream.Close()
	cancel()

	wg.Wait()
}
