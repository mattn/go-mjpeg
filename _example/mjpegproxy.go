// +build ignore

package main

import (
	"bytes"
	"context"
	"flag"
	"image/jpeg"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/mattn/go-mjpeg"
)

var (
	url      = flag.String("url", "", "Camera host")
	addr     = flag.String("addr", ":8080", "Server address")
	interval = flag.Duration("interval", 200*time.Millisecond, "interval")
)

func proxy(wg *sync.WaitGroup, stream *mjpeg.Stream) {
	defer wg.Done()

	dec, err := mjpeg.NewDecoderFromURL(*url)
	if err != nil {
		log.Fatal(err)
	}

	var buf bytes.Buffer
	for {
		img, err := dec.Decode()
		if err != nil {
			break
		}
		buf.Reset()
		err = jpeg.Encode(&buf, img, nil)
		if err != nil {
			break
		}
		err = stream.Update(buf.Bytes())
		if err != nil {
			break
		}
	}
}

func main() {
	flag.Parse()
	if *url == "" {
		flag.Usage()
		os.Exit(1)
	}

	stream := mjpeg.NewStreamWithInterval(*interval)

	var wg sync.WaitGroup
	wg.Add(1)
	go proxy(&wg, stream)

	http.HandleFunc("/jpeg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(stream.Current())
	})

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
		server.Shutdown(context.Background())
	}()
	server.ListenAndServe()
	stream.Close()

	wg.Wait()
}
