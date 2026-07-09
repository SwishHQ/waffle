// Package imaging decodes images for PDF embedding. JPEG is embedded losslessly
// via DCTDecode (original bytes passed through); PNG is decoded and re-encoded
// with FlateDecode, its alpha channel becoming a soft-mask (SMask). The result
// is ready to write as an image XObject.
package imaging

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"strings"
)

// Image is a decoded image ready to embed as a PDF XObject.
type Image struct {
	Width, Height    int
	ColorSpace       string // "DeviceRGB", "DeviceGray", or "DeviceCMYK"
	BitsPerComponent int
	Filter           string // "DCTDecode" or "FlateDecode"
	Data             []byte // XObject stream data (already filtered)
	SMask            []byte // optional 8-bit gray alpha, FlateDecode-compressed
	Format           string // "jpeg" or "png"
}

var pngMagic = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// Decode sniffs the image format and decodes it.
func Decode(data []byte) (*Image, error) {
	switch {
	case len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8:
		return decodeJPEG(data)
	case len(data) >= 8 && bytes.Equal(data[:8], pngMagic):
		return decodePNG(data)
	default:
		return nil, fmt.Errorf("imaging: unrecognized image format")
	}
}

// DecodeDataURI decodes a data: URI (base64 or raw payload).
func DecodeDataURI(s string) (*Image, error) {
	if !strings.HasPrefix(s, "data:") {
		return nil, fmt.Errorf("imaging: not a data URI")
	}
	comma := strings.IndexByte(s, ',')
	if comma < 0 {
		return nil, fmt.Errorf("imaging: malformed data URI")
	}
	meta, payload := s[5:comma], s[comma+1:]
	if strings.Contains(meta, "base64") {
		raw, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("imaging: bad base64: %w", err)
		}
		return Decode(raw)
	}
	return Decode([]byte(payload))
}

// decodeJPEG embeds the original JPEG bytes (DCTDecode) after reading the frame
// header for dimensions and component count.
func decodeJPEG(data []byte) (*Image, error) {
	i := 2 // past SOI
	for i+1 < len(data) {
		if data[i] != 0xFF {
			i++
			continue
		}
		marker := data[i+1]
		i += 2
		// Standalone markers (no length): RSTn, SOI, EOI, TEM.
		if marker == 0xD8 || marker == 0xD9 || marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7) {
			continue
		}
		if i+2 > len(data) {
			break
		}
		segLen := int(data[i])<<8 | int(data[i+1])
		if isSOF(marker) {
			if i+7 >= len(data) {
				break
			}
			precision := int(data[i+2])
			h := int(data[i+3])<<8 | int(data[i+4])
			w := int(data[i+5])<<8 | int(data[i+6])
			cs := "DeviceRGB"
			switch data[i+7] {
			case 1:
				cs = "DeviceGray"
			case 4:
				cs = "DeviceCMYK"
			}
			return &Image{
				Width: w, Height: h, ColorSpace: cs, BitsPerComponent: precision,
				Filter: "DCTDecode", Data: data, Format: "jpeg",
			}, nil
		}
		if marker == 0xDA { // start of scan; headers are done
			break
		}
		i += segLen
	}
	return nil, fmt.Errorf("imaging: no frame header (SOF) found in JPEG")
}

// isSOF reports whether a marker is a Start-Of-Frame (C0..CF except DHT/JPG/DAC).
func isSOF(m byte) bool {
	if m < 0xC0 || m > 0xCF {
		return false
	}
	return m != 0xC4 && m != 0xC8 && m != 0xCC
}

// decodePNG decodes any PNG (interlaced, 16-bit, palette, …) via the standard
// library, then re-encodes as 8-bit DeviceRGB with a separate 8-bit alpha SMask.
func decodePNG(data []byte) (*Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("imaging: png decode: %w", err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	nrgba := image.NewNRGBA(b)
	draw.Draw(nrgba, b, img, b.Min, draw.Src)

	rgb := make([]byte, 0, w*h*3)
	alpha := make([]byte, 0, w*h)
	hasAlpha := false
	for p := 0; p+3 < len(nrgba.Pix); p += 4 {
		rgb = append(rgb, nrgba.Pix[p], nrgba.Pix[p+1], nrgba.Pix[p+2])
		a := nrgba.Pix[p+3]
		alpha = append(alpha, a)
		if a < 255 {
			hasAlpha = true
		}
	}

	im := &Image{
		Width: w, Height: h, ColorSpace: "DeviceRGB", BitsPerComponent: 8,
		Filter: "FlateDecode", Data: flateCompress(rgb), Format: "png",
	}
	if hasAlpha {
		im.SMask = flateCompress(alpha)
	}
	return im, nil
}

func flateCompress(data []byte) []byte {
	var buf bytes.Buffer
	zw := zlib.NewWriter(&buf)
	_, _ = zw.Write(data)
	_ = zw.Close()
	return buf.Bytes()
}
