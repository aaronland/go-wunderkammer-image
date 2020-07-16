package decode

import (
	"context"
	"github.com/aaronland/go-roster"
	"image"
	"io"
	"net/url"
)

type InitializeDecoderFunc func(context.Context, string) (Decoder, error)

type Decoder interface {
	Decode(context.Context, io.ReadSeeker) (image.Image, string, error)
}

var decoders roster.Roster

func ensureRoster() error {

	if decoders == nil {

		r, err := roster.NewDefaultRoster()

		if err != nil {
			return err
		}

		decoders = r
	}

	return nil
}

func RegisterDecoder(ctx context.Context, f InitializeDecoderFunc, schemes ...string) error {

	err := ensureRoster()

	if err != nil {
		return err
	}

	for _, s := range schemes {

		err := decoders.Register(ctx, s, f)

		if err != nil {
			return err
		}
	}

	return nil
}

func NewDecoder(ctx context.Context, uri string) (Decoder, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, err
	}

	scheme := u.Scheme

	i, err := decoders.Driver(ctx, scheme)

	if err != nil {
		return nil, err
	}

	f := i.(InitializeDecoderFunc)
	return f(ctx, uri)
}
