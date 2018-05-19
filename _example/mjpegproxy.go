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

	dec, err := mjpeg.NewDecoderFromURL(*url)
	if err != nil {
		log.Fatal(err)
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
			stream.Update(buf.Bytes())
		}
	}()

	http.HandleFunc("/jpeg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(stream.Current())
	})

	http.HandleFunc("/mjpeg", stream.ServeHTTP)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<img src="/mjpeg" />`))
	})

	server.ListenAndServe()
}
