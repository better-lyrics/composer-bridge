package icons

import "testing"

func TestEmbeddedIconsArePopulated(t *testing.T) {
	if len(Mac) == 0 {
		t.Error("Mac icon bytes are empty; //go:embed icon-mac.png failed")
	}
	if len(Default) == 0 {
		t.Error("Default icon bytes are empty; //go:embed icon.png failed")
	}
	// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
	if !(Mac[0] == 0x89 && Mac[1] == 'P' && Mac[2] == 'N' && Mac[3] == 'G') {
		t.Error("Mac icon does not start with PNG magic bytes")
	}
	if !(Default[0] == 0x89 && Default[1] == 'P' && Default[2] == 'N' && Default[3] == 'G') {
		t.Error("Default icon does not start with PNG magic bytes")
	}
}
