package decode

import (
	"context"
	"image"
	"io"
)

type ImageDecoder struct {
	Decoder
}

func init() {

	ctx := context.Background()
	err := RegisterDecoder(ctx, NewImageDecoder, "image")

	if err != nil {
		panic(err)
	}
}

func NewImageDecoder(ctx context.Context, uri string) (Decoder, error) {

	e := &ImageDecoder{}
	return e, nil
}

func (e *ImageDecoder) Decode(ctx context.Context, r io.ReadSeeker) (image.Image, string, error) {
	return image.Decode(r)
}
