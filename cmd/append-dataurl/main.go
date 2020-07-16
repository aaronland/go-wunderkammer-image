package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"github.com/aaronland/go-wunderkammer/oembed"
	"github.com/aaronland/go-wunderkammer-image"	
	"github.com/tidwall/pretty"
	"io"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func main() {

	content_aware_resize := flag.Bool("content-aware-resize", false, "Enable content aware (seam carving) resizing.")
	content_aware_height := flag.Int("content-aware-height", 0, "Content aware resizing to this height.")
	content_aware_width := flag.Int("content-aware-width", 0, "Content aware resizing to this width.")

	dither := flag.Bool("dither", false, "Dither (halftone) the final image.")
	resize := flag.Bool("resize", false, "Resize images to a maximum dimension (preserving aspect ratio).")
	resize_max_dimension := flag.Int("resize-max-dimension", 0, "Resize images to this maximum height or width (preserving aspect ratio).")

	auto_rotate := flag.Bool("auto-rotate", false, "Auto-rotate image based on EXIF data")

	overwrite := flag.Bool("overwrite", false, "Overwrite exisiting data_url properties")

	format_json := flag.Bool("format", false, "Emit results as formatted JSON.")
	as_json := flag.Bool("json", false, "Emit results as a JSON array.")

	to_stdout := flag.Bool("stdout", true, "Emit to STDOUT")
	to_devnull := flag.Bool("null", false, "Emit to /dev/null")

	workers := flag.Int("workers", runtime.NumCPU(), "The number of concurrent workers to append data URLs with")
	timings := flag.Bool("timings", false, "Log timings (time to wait to process, time to complete processing")

	strict := flag.Bool("strict", false, "If true any error appending a data URL will stop execution.")

	image_format := flag.String("image-format", "jpeg", "Output format for encoded 'data_url' images. If empty then the content-type of the source image (defined in the 'url' property) will be used.")

	flag.Parse()

	if *content_aware_resize {

		if *content_aware_height == 0 {
			log.Fatalf("Missing -content-aware-height value")
		}

		if *content_aware_width == 0 {
			log.Fatalf("Missing -content-aware-width value")
		}
	}

	if *resize {

		if *resize_max_dimension == 0 {
			log.Fatalf("Missing -resize-max-dimension value")
		}
	}

	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	writers := make([]io.Writer, 0)

	if *to_stdout {
		writers = append(writers, os.Stdout)
	}

	if *to_devnull {
		writers = append(writers, ioutil.Discard)
	}

	if len(writers) == 0 {
		log.Fatal("Nothing to write to.")
	}

	wr := io.MultiWriter(writers...)

	reader := bufio.NewReader(os.Stdin)

	count := int32(0)

	throttle := make(chan bool, *workers)

	for i := 0; i < *workers; i++ {
		throttle <- true
	}

	mu := new(sync.RWMutex)
	wg := new(sync.WaitGroup)

	t0 := time.Now()

	for {

		select {
		case <-ctx.Done():
			break
		default:
			// pass
		}

		body, err := reader.ReadBytes('\n')

		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatalf("Failed to read bytes, %v", err)
		}

		body = bytes.TrimSpace(body)

		var rec *oembed.Photo

		err = json.Unmarshal(body, &rec)

		if err != nil {
			log.Fatalf("Failed to unmarshal OEmbed record, %v", err)
		}

		t1 := time.Now()

		<-throttle

		if *timings {
			log.Printf("Time to wait to process %s, %v\n", rec.URL, time.Since(t1))
		}

		wg.Add(1)

		go func(rec *oembed.Photo) {

			t2 := time.Now()

			defer func() {

				throttle <- true
				wg.Done()

				if *timings {
					log.Printf("Time to complete processing for %s, %v\n", rec.URL, time.Since(t2))
				}
			}()

			select {
			case <-ctx.Done():
				return
			default:
				// pass
			}

			if rec.DataURL == "" || *overwrite {

				opts := &image.DataURLOptions{
					ContentAwareResize: *content_aware_resize,
					ContentAwareWidth:  *content_aware_width,
					ContentAwareHeight: *content_aware_height,
					Resize:             *resize,
					ResizeMaxDimension: *resize_max_dimension,
					Dither:             *dither,
					Format:             *image_format,
					AutoRotate:         *auto_rotate,
				}

				data_url, err := image.DataURL(ctx, rec.URL, opts)

				if err != nil {

					log.Printf("Failed to populate data URL for '%s', %v\n", rec.URL, err)

					if *strict {
						cancel()
					}

					return
				}

				rec.DataURL = data_url
			}

			body, err := json.Marshal(rec)

			if err != nil {
				log.Fatalf("Failed to marshal record, %v", err)
			}

			if *format_json {
				body = pretty.Pretty(body)
			}

			new_count := atomic.AddInt32(&count, 1)

			mu.Lock()
			defer mu.Unlock()

			if *as_json && new_count > 1 {
				wr.Write([]byte(","))
			}

			wr.Write(body)
			wr.Write([]byte("\n"))

		}(rec)

	}

	if *as_json {
		wr.Write([]byte("]"))
	}

	wg.Wait()

	if *timings {
		log.Printf("Time to process %d records, %v\n", count, time.Since(t0))
	}
}
