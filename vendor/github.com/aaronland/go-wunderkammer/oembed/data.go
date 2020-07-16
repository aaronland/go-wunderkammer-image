package oembed

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/aaronland/go-image-decode"
	"github.com/aaronland/go-image-encode"
	"github.com/aaronland/go-image-halftone"
	"github.com/aaronland/go-image-resize"
	"github.com/aaronland/go-image-rotate"
	"github.com/esimov/caire"
	"image"
	"image/draw"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strings"
)

type DataURLOptions struct {
	ContentAwareResize bool
	ContentAwareHeight int
	ContentAwareWidth  int
	Dither             bool
	Resize             bool
	ResizeMaxDimension int
	Format             string
	AutoRotate         bool
}

func DataURL(ctx context.Context, url string, opts *DataURLOptions) (string, error) {

	select {
	case <-ctx.Done():
		return "", nil
	default:
		// pass
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)

	if err != nil {
		return "", err
	}

	// make this a singleton?
	cl := &http.Client{}

	rsp, err := cl.Do(req)

	if err != nil {
		return "", err
	}

	defer rsp.Body.Close()

	content_type := rsp.Header.Get("Content-type")

	body, err := ioutil.ReadAll(rsp.Body)

	if err != nil {
		return "", err
	}

	im_type := strings.Split(content_type, "/")

	if len(im_type) != 2 {
		return "", errors.New("Unrecognized content type")
	}

	if im_type[0] == "image" {

		im_format := opts.Format

		if im_format == "" {
			im_format = im_type[1]
		}

		dec, err := decode.NewDecoder(ctx, "image://")

		if err != nil {
			return "", err
		}

		enc_uri := fmt.Sprintf("%s://", im_format)

		enc, err := encode.NewEncoder(ctx, enc_uri)

		if err != nil {
			return "", err
		}

		br := bytes.NewReader(body)

		new_im, format, err := dec.Decode(ctx, br)

		if err != nil {
			return "", err
		}

		if opts.AutoRotate {

			orientation := "0"

			if format == "jpeg" {

				_, err := br.Seek(0, 0)

				if err != nil {
					return "", err
				}

				o, err := rotate.GetImageOrientation(ctx, br)

				if err != nil {
					log.Println(err)
				} else {
					orientation = o
				}
			}

			new_im, err = rotate.RotateImageWithOrientation(ctx, new_im, orientation)

			if err != nil {
				return "", err
			}
		}

		if opts.ContentAwareResize {

			caire_w := opts.ContentAwareWidth
			caire_h := opts.ContentAwareHeight

			// failing here because... ?
			// https://github.com/esimov/caire/blob/eb499d00d8b9e45511b0a5fc3418b26b24123081/process.go#L165

			pr := &caire.Processor{
				NewWidth:  caire_w,
				NewHeight: caire_h,
				Scale:     true,
			}

			b := new_im.Bounds()
			caire_im := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
			draw.Draw(caire_im, caire_im.Bounds(), new_im, b.Min, draw.Src)

			resized_im, err := pr.Resize(caire_im)

			if err != nil {

				log.Printf("Failed to resize %s, %v\n", url, err)

				max_fl := math.Max(float64(caire_w), float64(caire_h))
				max := int(max_fl)

				new_im, err = resize.ResizeImageMax(ctx, new_im, max)

				if err != nil {
					return "", err
				}

			} else {
				new_im = resized_im
			}

		}

		if !opts.ContentAwareResize && opts.Resize {

			new_im, err = resize.ResizeImageMax(ctx, new_im, opts.ResizeMaxDimension)

			if err != nil {
				return "", err
			}
		}

		// end of caire stuff

		if opts.Dither {

			opts := halftone.NewDefaultHalftoneOptions()
			new_im, err = halftone.HalftoneImage(ctx, new_im, opts)

			if err != nil {
				return "", err
			}
		}

		var buf bytes.Buffer
		wr := bufio.NewWriter(&buf)

		err = enc.Encode(ctx, new_im, wr)

		if err != nil {
			return "", err
		}

		wr.Flush()

		body = buf.Bytes()
	}

	b64_data := base64.StdEncoding.EncodeToString(body)
	data_url := fmt.Sprintf("data:%s;base64,%s", content_type, b64_data)

	return data_url, nil
}
