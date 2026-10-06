package rtmp

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/datarhei/joy4/av"
	"github.com/datarhei/joy4/format/flv"
	"github.com/stretchr/testify/require"
)

// TestFFmpegReadsOurMultiTrackFLV writes a multi-track FLV with the joy4
// muxer and verifies that the real FFmpeg demuxer exposes all audio tracks
// and applies the E-RTMP track 1 audio config as extradata. Skipped when
// ffprobe is not available.
func TestFFmpegReadsOurMultiTrackFLV(t *testing.T) {
	require.Equal(t, []byte{0x12, 0x10}, testAAC(t, 44100, 4).MPEG4AudioConfigBytes(), "ASC for AAC-LC 44100 stereo")
	require.Equal(t, []byte{0x11, 0x90}, testAAC(t, 48000, 3).MPEG4AudioConfigBytes(), "ASC for AAC-LC 48000 stereo")

	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not available")
	}

	aac0 := testAAC(t, 44100, 4)
	aac1 := testAAC(t, 48000, 3)
	h264 := testH264(t)

	streams := []av.CodecData{h264, aac0, aac1}

	path := filepath.Join(t.TempDir(), "multitrack.flv")
	out, err := os.Create(path)
	require.NoError(t, err)
	defer out.Close()

	muxer := flv.NewMuxer(out)
	require.NoError(t, muxer.WriteHeader(streams))

	for i := 0; i < 10; i++ {
		for idx := range streams {
			var data []byte
			switch idx {
			case 0:
				data = []byte{0x00, 0x00, 0x01, 0x65}
			case 1:
				data = []byte{0xa0, byte(i)}
			case 2:
				data = []byte{0xb0, byte(i)}
			}
			require.NoError(t, muxer.WritePacket(av.Packet{
				Idx:        int8(idx),
				Data:       data,
				Time:       int64(i) * 40,
				IsKeyFrame: idx == 0,
			}))
		}
	}
	require.NoError(t, muxer.WriteTrailer())
	require.NoError(t, out.Close())

	// probe without frame analysis: ex-audio sample rates are only learned
	// from decodable frames, the synthetic payloads above are not decodable
	cmd := exec.Command(ffprobe, "-v", "error", "-probesize", "32", "-analyzeduration", "0",
		"-show_entries", "stream=index,codec_type,codec_name,extradata", "-show_data", "-of", "json", path)
	stdout, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffprobe failed: %s", stdout)

	require.Equal(t, 2, countOccurrences(string(stdout), `"codec_type": "audio"`),
		"expected FFmpeg to see 2 audio streams:\n%s", stdout)

	// the E-RTMP track 1 sequence start must arrive as stream extradata
	require.Contains(t, string(stdout), "1190",
		"expected the AAC config (0x11 0x90) of track 1 as extradata:\n%s", stdout)
	require.Contains(t, string(stdout), "1210",
		"expected the AAC config (0x12 0x10) of track 0:\n%s", stdout)
}

// TestFFmpegRTMPMultiTrackRoundTrip is the full end-to-end check with real
// media: FFmpeg encodes video + two AAC tracks and publishes them in one
// E-RTMP multi-track stream (exactly like OBS with a Twitch VOD track), the
// real RTMP server accepts and re-muxes it, and ffprobe pulls the loopback
// stream and must see all three streams with their audio configs. Skipped
// when ffmpeg/ffprobe are not available.
func TestFFmpegRTMPMultiTrackRoundTrip(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not available")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not available")
	}

	addr := "127.0.0.1:21936"

	server, err := New(Config{
		Addr: addr,
		App:  "/live",
	})
	require.NoError(t, err)

	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Close()

	// OBS-style publisher: video + legacy AAC track 0 + E-RTMP track 1 in one
	// FLV/RTMP stream
	pub := exec.Command(ffmpeg,
		"-re",
		"-f", "lavfi", "-i", "testsrc=size=128x96:rate=5:duration=6",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=6",
		"-f", "lavfi", "-i", "sine=frequency=880:duration=6",
		"-map", "0:v", "-map", "1:a", "-map", "2:a",
		"-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-g", "5",
		"-c:a", "aac", "-ar:a:0", "44100", "-ar:a:1", "48000", "-b:a", "64k",
		"-f", "flv", "rtmp://"+addr+"/live/test.stream")

	pubErr := make(chan error, 1)
	if err := pub.Start(); err != nil {
		t.Skipf("cannot start ffmpeg: %v", err)
	}
	defer pub.Process.Kill()

	go func() {
		pubErr <- pub.Wait()
	}()

	// wait until the server accepted the publish stream
	require.Eventually(t, func() bool {
		return len(server.Channels()) == 1
	}, 15*time.Second, 100*time.Millisecond, "publisher never registered at the RTMP server")

	cmd := exec.Command(ffprobe, "-v", "error",
		"-show_entries", "stream=index,codec_type,codec_name,sample_rate", "-of", "json",
		"rtmp://"+addr+"/live/test.stream")
	stdout, err := cmd.CombinedOutput()
	require.NoError(t, err, "ffprobe failed: %s", stdout)

	require.Equal(t, 2, countOccurrences(string(stdout), `"codec_type": "audio"`),
		"expected the loopback stream to carry 2 audio tracks:\n%s", stdout)
	require.Contains(t, string(stdout), `"sample_rate": "44100"`,
		"expected track 0 (live audio):\n%s", stdout)
	require.Contains(t, string(stdout), `"sample_rate": "48000"`,
		"expected track 1 (VOD audio):\n%s", stdout)

	_ = pub.Process.Kill()
	<-pubErr
}

func countOccurrences(s, sub string) int {
	count := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			count++
		}
	}
	return count
}
