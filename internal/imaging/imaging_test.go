package imaging

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func makePNG(t *testing.T, w, h int, withAlpha bool) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			a := uint8(255)
			if withAlpha && (x+y)%2 == 0 {
				a = 128
			}
			img.Set(x, y, color.NRGBA{R: uint8(x * 30), G: uint8(y * 30), B: 100, A: a})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecodePNG(t *testing.T) {
	im, err := Decode(makePNG(t, 4, 3, false))
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 4 || im.Height != 3 {
		t.Errorf("dims = %dx%d, want 4x3", im.Width, im.Height)
	}
	if im.ColorSpace != "DeviceRGB" || im.BitsPerComponent != 8 || im.Filter != "FlateDecode" {
		t.Errorf("png metadata = %+v", im)
	}
	if im.SMask != nil {
		t.Errorf("opaque PNG should have no SMask")
	}
	if len(im.Data) == 0 {
		t.Errorf("no image data")
	}
}

func TestDecodePNGAlpha(t *testing.T) {
	im, err := Decode(makePNG(t, 4, 4, true))
	if err != nil {
		t.Fatal(err)
	}
	if im.SMask == nil {
		t.Errorf("PNG with alpha should produce an SMask")
	}
}

func TestDecodeJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 60, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	data := buf.Bytes()

	im, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 8 || im.Height != 6 {
		t.Errorf("dims = %dx%d, want 8x6", im.Width, im.Height)
	}
	if im.Filter != "DCTDecode" || im.ColorSpace != "DeviceRGB" || im.Format != "jpeg" {
		t.Errorf("jpeg metadata = %+v", im)
	}
	if !bytes.Equal(im.Data, data) {
		t.Errorf("JPEG should be embedded losslessly (data passthrough)")
	}
}

func TestDecodeDataURI(t *testing.T) {
	png := makePNG(t, 2, 2, false)
	uri := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	im, err := DecodeDataURI(uri)
	if err != nil {
		t.Fatal(err)
	}
	if im.Width != 2 || im.Height != 2 {
		t.Errorf("data URI dims = %dx%d, want 2x2", im.Width, im.Height)
	}
}

func TestDecodeUnknown(t *testing.T) {
	if _, err := Decode([]byte("not an image")); err == nil {
		t.Errorf("expected error for unknown format")
	}
}
