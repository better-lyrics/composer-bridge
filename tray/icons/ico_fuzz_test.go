package icons

import "testing"

func FuzzICO(f *testing.F) {
	f.Add(MenuDot)
	f.Add([]byte("\x89PNG\r\n\x1a\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = ICO(data)
	})
}
