package icons

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"

	"golang.org/x/image/draw"
)

// icoSizes are the square sizes Windows asks for across tray and menu icons at
// 100% to 200% display scaling. Sizes above the source are skipped.
var icoSizes = []int{16, 20, 24, 32, 40, 48, 64, 256}

// ICO packs a PNG into a multi-size .ico whose entries are PNG-compressed.
// Windows' LoadImage (used by systray on Windows) only accepts .ico data, and
// picks the entry closest to the size it needs.
func ICO(pngData []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, fmt.Errorf("decode png: %w", err)
	}
	srcSize := src.Bounds().Dx()
	if srcSize != src.Bounds().Dy() {
		return nil, fmt.Errorf("icon must be square, got %dx%d", srcSize, src.Bounds().Dy())
	}

	var entries [][]byte
	var sizes []int
	for _, size := range icoSizes {
		if size > srcSize {
			break
		}
		dst := image.NewNRGBA(image.Rect(0, 0, size, size))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
		var buf bytes.Buffer
		if err := png.Encode(&buf, dst); err != nil {
			return nil, fmt.Errorf("encode %dpx: %w", size, err)
		}
		entries = append(entries, buf.Bytes())
		sizes = append(sizes, size)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("icon is smaller than %dpx", icoSizes[0])
	}

	const headerLen, entryLen = 6, 16
	var out bytes.Buffer
	le := binary.LittleEndian
	out.Write(le.AppendUint16(nil, 0))
	out.Write(le.AppendUint16(nil, 1))
	out.Write(le.AppendUint16(nil, uint16(len(entries))))
	offset := headerLen + entryLen*len(entries)
	for i, data := range entries {
		dim := byte(sizes[i])
		if sizes[i] >= 256 {
			dim = 0
		}
		out.Write([]byte{dim, dim, 0, 0})
		out.Write(le.AppendUint16(nil, 1))
		out.Write(le.AppendUint16(nil, 32))
		out.Write(le.AppendUint32(nil, uint32(len(data))))
		out.Write(le.AppendUint32(nil, uint32(offset)))
		offset += len(data)
	}
	for _, data := range entries {
		out.Write(data)
	}
	return out.Bytes(), nil
}
