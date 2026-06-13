package ytdlp

import (
	"io"
	"os"
)

// IsMpegTSFile peeks at path and reports whether it looks like an MPEG transport
// stream. The pre-1.4.11 opus selector matched YouTube's HLS variant, dumping
// MPEG-TS bytes into a .opus-named file that no browser can play. Two sync
// bytes (0x47 at offsets 0 and 188) confirm MPEG-TS while ruling out a
// coincidental leading byte on an otherwise valid container.
func IsMpegTSFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var head [189]byte
	if _, err := io.ReadFull(f, head[:]); err != nil {
		return false
	}
	return head[0] == 0x47 && head[188] == 0x47
}
