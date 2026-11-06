package ytdlp

import (
	"os"
	"slices"
	"strings"
	"testing"
)

func FuzzParse(f *testing.F) {
	f.Add([]byte(`{"id":"ZEcqHA7dbwM","title":"x","duration":120,"thumbnails":[{"url":"https://yt3.googleusercontent.com/a=w60-h60"}]}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		info, err := Parse(raw)
		if err != nil {
			return
		}
		_ = info.IsMusic()
		for _, th := range info.MusicArtThumbnails() {
			if !strings.Contains(th.URL, ytMusicArtHost) {
				t.Fatalf("music art thumbnail from another host: %q", th.URL)
			}
		}
	})
}

// Every record the converter writes must survive yt-dlp's Netscape parser,
// which splits on tabs and newlines.
func FuzzConvertJSONToNetscape(f *testing.F) {
	f.Add(`[{"domain":".youtube.com","name":"SID","value":"abc","path":"/","secure":true,"expirationDate":1893456000}]`)
	f.Add(`{"cookies":[{"domain":"youtube.com","name":"a","value":"b","session":true}]}`)
	f.Add(`[{"domain":".youtube.com","name":"x","value":"line\nbreak"}]`)
	f.Fuzz(func(t *testing.T, content string) {
		out, err := convertJSONToNetscape(content)
		if err != nil {
			return
		}
		if !looksLikeNetscape(out) {
			t.Fatalf("converted output is not recognised as Netscape:\n%q", out)
		}
		for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
			if strings.HasPrefix(line, "#") {
				continue
			}
			if n := len(strings.Split(line, "\t")); n != 7 {
				t.Fatalf("record has %d fields, want 7: %q (input %q)", n, line, content)
			}
		}
	})
}

func FuzzAugmentPATH(f *testing.F) {
	sep := string(os.PathListSeparator)
	f.Add("/usr/bin"+sep+"/bin", "/opt/homebrew/bin", "/data/bin", true)
	f.Add("", "", "/data/bin", false)
	f.Add("/a"+sep+sep+"/a", "/a", "/b", true)
	f.Fuzz(func(t *testing.T, path, extraA, extraB string, hasPath bool) {
		env := []string{"HOME=/h"}
		if hasPath {
			env = append(env, "PATH="+path)
		}
		extras := []string{extraA, extraB}
		out := augmentPATH(env, extras)

		var got string
		count := 0
		for _, kv := range out {
			if p, ok := strings.CutPrefix(kv, "PATH="); ok {
				got = p
				count++
			}
		}
		if count > 1 {
			t.Fatalf("PATH appears %d times: %q", count, out)
		}
		entries := strings.Split(got, sep)
		for _, e := range extras {
			if e != "" && !strings.Contains(e, sep) && !slices.Contains(entries, e) {
				t.Fatalf("extra %q missing from PATH %q", e, got)
			}
		}
		if hasPath && path != "" && !strings.HasSuffix(got, path) {
			t.Fatalf("original PATH %q not preserved at the end of %q", path, got)
		}
	})
}
