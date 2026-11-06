package icons

import (
	"bytes"
	"encoding/binary"
	"image/png"
	"testing"
)

// The Windows tray loads icons with LoadImage(IMAGE_ICON), which rejects PNG
// bytes. Every icon handed to systray on Windows must be a valid .ico.
func TestICO_EveryWindowsIconIsAValidICO(t *testing.T) {
	for name, p := range windowsIcons() {
		ico, err := ICO(*p)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		le := binary.LittleEndian
		if le.Uint16(ico[0:]) != 0 || le.Uint16(ico[2:]) != 1 {
			t.Fatalf("%s: missing ICO header", name)
		}
		count := int(le.Uint16(ico[4:]))
		if count == 0 {
			t.Fatalf("%s: no entries", name)
		}
		for i := range count {
			e := ico[6+16*i:]
			want := int(e[0])
			if want == 0 {
				want = 256
			}
			size, offset := le.Uint32(e[8:]), le.Uint32(e[12:])
			img, err := png.Decode(bytes.NewReader(ico[offset : offset+size]))
			if err != nil {
				t.Fatalf("%s entry %d: %v", name, i, err)
			}
			if got := img.Bounds().Dx(); got != want {
				t.Errorf("%s entry %d: directory says %dpx, image is %dpx", name, i, want, got)
			}
		}
	}
}
