package helpers

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"

	"errors"
)

var ErrInvalidImage = errors.New("invalid image or processing limits")

type ImageProcessor struct{ MaximumBytes, MaximumDimension, Size int }

func (processor ImageProcessor) Canonical(raw []byte) ([]byte, error) {
	if processor.MaximumBytes < 1 || processor.MaximumBytes > 16*1024*1024 || processor.MaximumDimension < 1 || processor.MaximumDimension > 4096 || processor.Size < 1 || processor.Size > 2048 {
		return nil, ErrInvalidImage
	}
	if len(raw) == 0 || len(raw) > processor.MaximumBytes {
		return nil, ErrInvalidImage
	}
	cfg, format, e := image.DecodeConfig(bytes.NewReader(raw))
	if e != nil || (format != "png" && format != "jpeg") || cfg.Width < 1 || cfg.Height < 1 || cfg.Width > processor.MaximumDimension || cfg.Height > processor.MaximumDimension {
		return nil, ErrInvalidImage
	}
	src, _, e := image.Decode(bytes.NewReader(raw))
	if e != nil {
		return nil, ErrInvalidImage
	}
	size := processor.Size
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	side := min(cfg.Width, cfg.Height)
	ox := (cfg.Width - side) / 2
	oy := (cfg.Height - side) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, a := src.At(src.Bounds().Min.X+ox+x*side/size, src.Bounds().Min.Y+oy+y*side/size).RGBA()
			dst.SetRGBA(x, y, color.RGBA{uint8((r + 65535 - a) >> 8), uint8((g + 65535 - a) >> 8), uint8((b + 65535 - a) >> 8), 255})
		}
	}
	var out bytes.Buffer
	if e = jpeg.Encode(&out, dst, &jpeg.Options{Quality: 85}); e != nil {
		return nil, e
	}
	return out.Bytes(), nil
}
