package rtmp

import (
	"bytes"
	"testing"
	"time"

	"github.com/datarhei/joy4/av"
	"github.com/datarhei/joy4/codec/aacparser"
	"github.com/datarhei/joy4/codec/h264parser"
	"github.com/datarhei/joy4/format/flv"
	"github.com/datarhei/joy4/format/flv/flvio"
	joy4rtmp "github.com/datarhei/joy4/format/rtmp"
	"github.com/stretchr/testify/require"
)

// AVCDecoderConfigurationRecord of a real x264 baseline 128x96 encode.
var testAVCC = []byte{
	0x01, 0x42, 0xc0, 0x1e, 0xff, 0xe1,
	0x00, 0x17,
	0x67, 0x42, 0xc0, 0x1e, 0xd9, 0x02, 0x0d, 0xb0, 0x11, 0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x00, 0x03, 0x00, 0x0a, 0x0f, 0x16, 0x2e, 0x48,
	0x01,
	0x00, 0x05,
	0x68, 0xcb, 0x83, 0xcb, 0x20,
}

func testH264(t *testing.T) h264parser.CodecData {
	t.Helper()

	codec, err := h264parser.NewCodecDataFromAVCDecoderConfRecord(testAVCC)
	require.NoError(t, err)

	return codec
}

func testAAC(t *testing.T, sampleRate int, sampleRateIndex uint) aacparser.CodecData {
	t.Helper()

	config := aacparser.MPEG4AudioConfig{
		SampleRate:      sampleRate,
		ChannelLayout:   av.CH_STEREO,
		ObjectType:      2,
		SampleRateIndex: sampleRateIndex,
		ChannelConfig:   2,
	}

	codec, err := aacparser.NewCodecDataFromMPEG4AudioConfig(config)
	require.NoError(t, err)
	require.Equal(t, sampleRate, codec.SampleRate())

	return codec
}

func fillParseTag(t *testing.T, tag flvio.Tag) flvio.Tag {
	t.Helper()

	buf := make([]byte, 64)
	n := tag.FillHeader(buf)
	buf = append(buf[:n], tag.Data...)

	parsed := flvio.Tag{Type: tag.Type}
	pn, err := parsed.ParseHeader(buf)
	require.NoError(t, err)
	require.Equal(t, n, pn, "consumed header length differs")
	parsed.Data = buf[pn:]

	return parsed
}

func TestAudioTagRoundTrip(t *testing.T) {
	payload := []byte{0xde, 0xad, 0xbe, 0xef}

	cases := []struct {
		name string
		tag  flvio.Tag
	}{
		{
			name: "legacy AAC sequence start",
			tag: flvio.Tag{
				Type:          flvio.TAG_AUDIO,
				SoundFormat:   flvio.SOUND_AAC,
				SoundRate:     flvio.SOUND_44Khz,
				SoundSize:     flvio.SOUND_16BIT,
				SoundType:     flvio.SOUND_STEREO,
				AACPacketType: flvio.AAC_SEQHDR,
				PacketType:    flvio.PKTTYPE_SEQUENCE_START,
				Data:          payload,
			},
		},
		{
			name: "legacy AAC raw",
			tag: flvio.Tag{
				Type:          flvio.TAG_AUDIO,
				SoundFormat:   flvio.SOUND_AAC,
				SoundRate:     flvio.SOUND_44Khz,
				SoundSize:     flvio.SOUND_16BIT,
				SoundType:     flvio.SOUND_MONO,
				AACPacketType: flvio.AAC_RAW,
				PacketType:    flvio.PKTTYPE_CODED_FRAMES,
				Data:          payload,
			},
		},
		{
			name: "legacy Speex",
			tag: flvio.Tag{
				Type:        flvio.TAG_AUDIO,
				SoundFormat: flvio.SOUND_SPEEX,
				SoundRate:   flvio.SOUND_11Khz,
				SoundSize:   flvio.SOUND_16BIT,
				SoundType:   flvio.SOUND_MONO,
				PacketType:  flvio.PKTTYPE_CODED_FRAMES,
				Data:        payload,
			},
		},
		{
			name: "ex audio non-wrapped sequence start (track 0)",
			tag: flvio.Tag{
				Type:        flvio.TAG_AUDIO,
				SoundFormat: flvio.SOUND_EXHEADER,
				IsExHeader:  true,
				PacketType:  flvio.PKTTYPE_SEQUENCE_START,
				FourCC:      flvio.FOURCC_MP4A,
				Data:        payload,
			},
		},
		{
			name: "ex audio non-wrapped coded frames (track 0)",
			tag: flvio.Tag{
				Type:        flvio.TAG_AUDIO,
				SoundFormat: flvio.SOUND_EXHEADER,
				IsExHeader:  true,
				PacketType:  flvio.PKTTYPE_CODED_FRAMES,
				FourCC:      flvio.FOURCC_MP4A,
				Data:        payload,
			},
		},
		{
			name: "ex audio multitrack sequence start track 1",
			tag: flvio.Tag{
				Type:           flvio.TAG_AUDIO,
				SoundFormat:    flvio.SOUND_EXHEADER,
				IsExHeader:     true,
				IsMultitrack:   true,
				MultitrackType: flvio.MULTITRACK_ONETRACK,
				PacketType:     flvio.PKTTYPE_SEQUENCE_START,
				FourCC:         flvio.FOURCC_MP4A,
				TrackID:        1,
				Data:           payload,
			},
		},
		{
			name: "ex audio multitrack coded frames track 1",
			tag: flvio.Tag{
				Type:           flvio.TAG_AUDIO,
				SoundFormat:    flvio.SOUND_EXHEADER,
				IsExHeader:     true,
				IsMultitrack:   true,
				MultitrackType: flvio.MULTITRACK_ONETRACK,
				PacketType:     flvio.PKTTYPE_CODED_FRAMES,
				FourCC:         flvio.FOURCC_MP4A,
				TrackID:        1,
				Data:           payload,
			},
		},
		{
			name: "ex audio multitrack coded frames track 255",
			tag: flvio.Tag{
				Type:           flvio.TAG_AUDIO,
				SoundFormat:    flvio.SOUND_EXHEADER,
				IsExHeader:     true,
				IsMultitrack:   true,
				MultitrackType: flvio.MULTITRACK_ONETRACK,
				PacketType:     flvio.PKTTYPE_CODED_FRAMESX,
				FourCC:         flvio.FOURCC_MP4A,
				TrackID:        255,
				Data:           payload,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed := fillParseTag(t, c.tag)
			require.Equal(t, c.tag, parsed)
		})
	}
}

// TestAudioMultitrackWireFormat pins the exact Enhanced RTMP v2 byte layout
// as emitted by OBS >= 30.2 ("Twitch VOD track") and FFmpeg 8's flv muxer.
func TestAudioMultitrackWireFormat(t *testing.T) {
	payload := []byte{0xde, 0xad, 0xbe, 0xef}

	// byte0: SoundFormat=9 (ExHeader) | AudioPacketType=5 (Multitrack) -> 0x95
	// byte1: AvMultitrackType=0 (OneTrack) | inner AudioPacketType
	// bytes2-5: FOURCC 'mp4a'
	// byte6: AudioTrackId
	seq := []byte{0x95, 0x00, 'm', 'p', '4', 'a', 0x01}
	frames := []byte{0x95, 0x01, 'm', 'p', '4', 'a', 0x01}

	for _, tc := range []struct {
		wire       []byte
		packetType uint8
		content    []byte
	}{
		{seq, flvio.PKTTYPE_SEQUENCE_START, payload},
		{frames, flvio.PKTTYPE_CODED_FRAMES, payload},
	} {
		msg := append(append([]byte{}, tc.wire...), tc.content...)

		tag := flvio.Tag{Type: flvio.TAG_AUDIO}
		n, err := tag.ParseHeader(msg)
		require.NoError(t, err)
		require.Equal(t, tc.wire, msg[:n])
		require.Equal(t, tc.content, msg[n:])

		require.Equal(t, uint8(flvio.SOUND_EXHEADER), tag.SoundFormat)
		require.True(t, tag.IsExHeader)
		require.True(t, tag.IsMultitrack)
		require.Equal(t, uint8(flvio.MULTITRACK_ONETRACK), tag.MultitrackType)
		require.Equal(t, tc.packetType, tag.PacketType)
		require.Equal(t, flvio.FOURCC_MP4A, tag.FourCC)
		require.Equal(t, uint8(1), tag.TrackID)

		out := make([]byte, 64)
		filled := tag.FillHeader(out)
		require.Equal(t, tc.wire, out[:filled])
	}
}

func TestAudioTagMalformed(t *testing.T) {
	cases := []struct {
		name string
		msg  []byte
	}{
		{"empty", []byte{}},
		{"legacy AAC without packet type", []byte{0xaf}},
		{"ex audio without fourcc", []byte{0x95, 0x00}},
		{"ex audio truncated fourcc", []byte{0x95, 0x00, 'm', 'p'}},
		{"multitrack without trackid", []byte{0x95, 0x00, 'm', 'p', '4', 'a'}},
		{"multitrack unsupported ManyTracks", []byte{0x95, 0x11, 'm', 'p', '4', 'a', 0x01}},
		{"multitrack unsupported ManySubtracks", []byte{0x95, 0x21, 'm', 'p', '4', 'a', 0x01}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tag := flvio.Tag{Type: flvio.TAG_AUDIO}
			_, err := tag.ParseHeader(c.msg)
			require.Error(t, err)
		})
	}
}

func TestVideoAvc1ExHeaderRoundTrip(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x01, 0x65}

	seq := flvio.Tag{
		Type:       flvio.TAG_VIDEO,
		FrameType:  flvio.FRAME_KEY,
		IsExHeader: true,
		PacketType: flvio.PKTTYPE_SEQUENCE_START,
		FourCC:     flvio.FOURCC_AVC1,
		Data:       testAVCC,
	}
	require.Equal(t, seq, fillParseTag(t, seq))

	frames := flvio.Tag{
		Type:            flvio.TAG_VIDEO,
		FrameType:       flvio.FRAME_KEY,
		IsExHeader:      true,
		PacketType:      flvio.PKTTYPE_CODED_FRAMES,
		FourCC:          flvio.FOURCC_AVC1,
		CompositionTime: 40,
		Data:            payload,
	}
	require.Equal(t, frames, fillParseTag(t, frames))

	// without composition time (CodedFramesX)
	framesx := flvio.Tag{
		Type:       flvio.TAG_VIDEO,
		FrameType:  flvio.FRAME_INTER,
		IsExHeader: true,
		PacketType: flvio.PKTTYPE_CODED_FRAMESX,
		FourCC:     flvio.FOURCC_AVC1,
		Data:       payload,
	}
	require.Equal(t, framesx, fillParseTag(t, framesx))
}

func exAudioTag(trackID uint8, packetType uint8, data []byte) flvio.Tag {
	return flvio.Tag{
		Type:           flvio.TAG_AUDIO,
		SoundFormat:    flvio.SOUND_EXHEADER,
		IsExHeader:     true,
		IsMultitrack:   true,
		MultitrackType: flvio.MULTITRACK_ONETRACK,
		PacketType:     packetType,
		FourCC:         flvio.FOURCC_MP4A,
		TrackID:        trackID,
		Data:           data,
	}
}

func legacyAudioTag(data []byte) flvio.Tag {
	return flvio.Tag{
		Type:          flvio.TAG_AUDIO,
		SoundFormat:   flvio.SOUND_AAC,
		SoundRate:     flvio.SOUND_44Khz,
		SoundSize:     flvio.SOUND_16BIT,
		SoundType:     flvio.SOUND_STEREO,
		AACPacketType: flvio.AAC_RAW,
		PacketType:    flvio.PKTTYPE_CODED_FRAMES,
		Data:          data,
	}
}

func TestProberMultiTrackCanonicalOrder(t *testing.T) {
	aac0 := testAAC(t, 44100, 4)
	aac1 := testAAC(t, 48000, 3)
	h264 := testH264(t)

	// arrival order deliberately shuffled: audio track 1, video, audio track 0
	prober := flv.NewProber(64)
	prober.HasAudio = true
	prober.HasVideo = true

	pushes := []flvio.Tag{
		exAudioTag(1, flvio.PKTTYPE_SEQUENCE_START, aac1.MPEG4AudioConfigBytes()),
		{
			Type:          flvio.TAG_VIDEO,
			FrameType:     flvio.FRAME_KEY,
			AVCPacketType: flvio.AVC_SEQHDR,
			CodecID:       flvio.VIDEO_H264,
			Data:          h264.AVCDecoderConfRecordBytes(),
		},
		{
			Type:          flvio.TAG_AUDIO,
			SoundFormat:   flvio.SOUND_AAC,
			AACPacketType: flvio.AAC_SEQHDR,
			PacketType:    flvio.PKTTYPE_SEQUENCE_START,
			Data:          aac0.MPEG4AudioConfigBytes(),
		},
		// media frames close the probe quiet window
		legacyAudioTag([]byte{0x01}),
		exAudioTag(1, flvio.PKTTYPE_CODED_FRAMES, []byte{0x02}),
		legacyAudioTag([]byte{0x03}),
		exAudioTag(1, flvio.PKTTYPE_CODED_FRAMES, []byte{0x04}),
	}

	for i, tag := range pushes {
		require.NoError(t, prober.PushTag(tag, int64(i)*20))
	}

	require.True(t, prober.Probed())

	// canonical order: video first, audio by ascending wire track ID
	streams := prober.Streams
	require.Equal(t, 3, len(streams))
	require.Equal(t, av.H264, streams[0].Type())
	require.Equal(t, av.AAC, streams[1].Type())
	require.Equal(t, av.AAC, streams[2].Type())
	require.Equal(t, 44100, streams[1].(av.AudioCodecData).SampleRate())
	require.Equal(t, 48000, streams[2].(av.AudioCodecData).SampleRate())
	require.Equal(t, []int{1, 2}, prober.AudioStreamIdxs)
	require.Equal(t, 0, prober.VideoStreamIdx)
	require.Equal(t, 1, prober.AudioStreamIdx)

	// cached media packets were mapped onto the canonical indexes
	require.False(t, prober.Empty())

	idxs := map[int8][]byte{}
	for !prober.Empty() {
		p, ok := prober.PopPacket()
		require.True(t, ok)
		idxs[p.Idx] = append(idxs[p.Idx], p.Data...)
	}
	require.Equal(t, []byte{0x01, 0x03}, idxs[int8(1)])
	require.Equal(t, []byte{0x02, 0x04}, idxs[int8(2)])
}

func TestProberQuietWindow(t *testing.T) {
	aac0 := testAAC(t, 44100, 4)
	aac1 := testAAC(t, 48000, 3)
	h264 := testH264(t)

	prober := flv.NewProber(64)
	prober.HasAudio = true
	prober.HasVideo = true

	videoCfg := flvio.Tag{
		Type:          flvio.TAG_VIDEO,
		FrameType:     flvio.FRAME_KEY,
		AVCPacketType: flvio.AVC_SEQHDR,
		CodecID:       flvio.VIDEO_H264,
		Data:          h264.AVCDecoderConfRecordBytes(),
	}
	audio0Cfg := flvio.Tag{
		Type:          flvio.TAG_AUDIO,
		SoundFormat:   flvio.SOUND_AAC,
		AACPacketType: flvio.AAC_SEQHDR,
		PacketType:    flvio.PKTTYPE_SEQUENCE_START,
		Data:          aac0.MPEG4AudioConfigBytes(),
	}
	audio1Cfg := exAudioTag(1, flvio.PKTTYPE_SEQUENCE_START, aac1.MPEG4AudioConfigBytes())

	require.NoError(t, prober.PushTag(videoCfg, 0))
	require.NoError(t, prober.PushTag(audio0Cfg, 0))
	require.NoError(t, prober.PushTag(audio1Cfg, 0))

	// A/V complete but the quiet window waits for possibly more tracks
	require.False(t, prober.Probed())

	// frames within the quiet window keep probing open ...
	require.NoError(t, prober.PushTag(legacyAudioTag([]byte{0x01}), 20))
	require.NoError(t, prober.PushTag(legacyAudioTag([]byte{0x02}), 40))
	require.NoError(t, prober.PushTag(exAudioTag(1, flvio.PKTTYPE_CODED_FRAMES, []byte{0x03}), 40))
	require.False(t, prober.Probed())

	// ... and a track announcing itself inside the window is captured
	require.NoError(t, prober.PushTag(exAudioTag(2, flvio.PKTTYPE_SEQUENCE_START, aac1.MPEG4AudioConfigBytes()), 60))
	require.False(t, prober.Probed())

	for i := 0; i < flv.ProbeQuietWindow; i++ {
		require.NoError(t, prober.PushTag(legacyAudioTag([]byte{0x04}), 80))
	}

	require.True(t, prober.Probed())
	require.Equal(t, 4, len(prober.Streams))
	require.Equal(t, []int{1, 2, 3}, prober.AudioStreamIdxs)

	// a track appearing after the freeze is dropped and counted, not mis-mapped
	require.NoError(t, prober.PushTag(exAudioTag(9, flvio.PKTTYPE_SEQUENCE_START, aac0.MPEG4AudioConfigBytes()), 100))
	pkt, ok := prober.TagToPacket(exAudioTag(9, flvio.PKTTYPE_CODED_FRAMES, []byte{0x05}), 120)
	require.False(t, ok)
	require.Equal(t, 1, prober.DroppedUnknownTrack)

	// registered tracks keep their mapping
	pkt, ok = prober.TagToPacket(exAudioTag(2, flvio.PKTTYPE_CODED_FRAMES, []byte{0x06}), 120)
	require.True(t, ok)
	require.Equal(t, int8(3), pkt.Idx)
}

func TestProberLegacyQuietWindow(t *testing.T) {
	aac0 := testAAC(t, 44100, 4)
	h264 := testH264(t)

	prober := flv.NewProber(64)
	prober.HasAudio = true
	prober.HasVideo = true

	require.NoError(t, prober.PushTag(flvio.Tag{
		Type:          flvio.TAG_VIDEO,
		FrameType:     flvio.FRAME_KEY,
		AVCPacketType: flvio.AVC_SEQHDR,
		CodecID:       flvio.VIDEO_H264,
		Data:          h264.AVCDecoderConfRecordBytes(),
	}, 0))
	require.NoError(t, prober.PushTag(flvio.Tag{
		Type:          flvio.TAG_AUDIO,
		SoundFormat:   flvio.SOUND_AAC,
		AACPacketType: flvio.AAC_SEQHDR,
		PacketType:    flvio.PKTTYPE_SEQUENCE_START,
		Data:          aac0.MPEG4AudioConfigBytes(),
	}, 0))

	// the quiet window stays open briefly: a stream that STARTS like a plain
	// legacy stream may still announce further audio tracks (OBS sends the
	// Twitch VOD track as E-RTMP audio right after the legacy track)
	require.False(t, prober.Probed())

	for i := 0; i < flv.ProbeQuietWindow; i++ {
		require.NoError(t, prober.PushTag(legacyAudioTag([]byte{0x01}), 20))
	}

	require.True(t, prober.Probed())
	require.Equal(t, 2, len(prober.Streams))
}

func TestProberHardStopWithPartialTracks(t *testing.T) {
	h264 := testH264(t)

	prober := flv.NewProber(4)
	prober.HasAudio = true
	prober.HasVideo = true

	videoCfg := flvio.Tag{
		Type:          flvio.TAG_VIDEO,
		FrameType:     flvio.FRAME_KEY,
		AVCPacketType: flvio.AVC_SEQHDR,
		CodecID:       flvio.VIDEO_H264,
		Data:          h264.AVCDecoderConfRecordBytes(),
	}

	// audio never announces itself: hard stop completes with what we have
	for i := 0; i < 4; i++ {
		require.NoError(t, prober.PushTag(videoCfg, 0))
	}

	require.True(t, prober.Probed())
	require.Equal(t, 1, len(prober.Streams))
}

func TestMuxerDemuxerMultiTrackRoundTrip(t *testing.T) {
	aac0 := testAAC(t, 44100, 4)
	aac1 := testAAC(t, 48000, 3)
	h264 := testH264(t)

	streams := []av.CodecData{h264, aac0, aac1}

	buf := &bytes.Buffer{}
	muxer := flv.NewMuxer(buf)
	require.NoError(t, muxer.WriteHeader(streams))

	// media packets
	samples := []struct {
		idx  int8
		data []byte
		ts   int64
		key  bool
	}{
		{0, []byte{0x00, 0x00, 0x01, 0x65}, 0, true},
		{1, []byte{0xa0}, 0, false},
		{2, []byte{0xb0}, 0, false},
		{0, []byte{0x00, 0x00, 0x01, 0x41}, 20, false},
		{1, []byte{0xa1}, 20, false},
		{2, []byte{0xb1}, 20, false},
	}

	for _, s := range samples {
		require.NoError(t, muxer.WritePacket(av.Packet{
			Idx:             s.idx,
			Data:            s.data,
			Time:            s.ts,
			IsKeyFrame:      s.key,
			CompositionTime: 0,
		}))
	}
	require.NoError(t, muxer.WriteTrailer())

	// legacy-first framing: track 0 legacy, track 1 E-RTMP multitrack
	raw := buf.Bytes()
	// skip FLV file header (9) + PreviousTagSize0 (4)
	configs := bytes.NewReader(raw[13:])

	videoTag, _, err := flvio.ReadTag(configs, make([]byte, 64))
	require.NoError(t, err)
	require.Equal(t, uint8(flvio.TAG_VIDEO), videoTag.Type)
	require.False(t, videoTag.IsExHeader)
	require.Equal(t, uint8(flvio.AVC_SEQHDR), videoTag.AVCPacketType)

	audio0Tag, _, err := flvio.ReadTag(configs, make([]byte, 64))
	require.NoError(t, err)
	require.Equal(t, uint8(flvio.TAG_AUDIO), audio0Tag.Type)
	require.False(t, audio0Tag.IsExHeader)
	require.Equal(t, uint8(flvio.SOUND_AAC), audio0Tag.SoundFormat)

	audio1Tag, _, err := flvio.ReadTag(configs, make([]byte, 64))
	require.NoError(t, err)
	require.Equal(t, uint8(flvio.TAG_AUDIO), audio1Tag.Type)
	require.True(t, audio1Tag.IsExHeader)
	require.True(t, audio1Tag.IsMultitrack)
	require.Equal(t, flvio.FOURCC_MP4A, audio1Tag.FourCC)
	require.Equal(t, uint8(1), audio1Tag.TrackID)
	require.Equal(t, uint8(flvio.PKTTYPE_SEQUENCE_START), audio1Tag.PacketType)

	// demux back
	demuxer := flv.NewDemuxer(bytes.NewReader(buf.Bytes()))
	demuxStreams, err := demuxer.Streams()
	require.NoError(t, err)
	require.Equal(t, 3, len(demuxStreams))
	require.Equal(t, av.H264, demuxStreams[0].Type())
	require.Equal(t, 44100, demuxStreams[1].(av.AudioCodecData).SampleRate())
	require.Equal(t, 48000, demuxStreams[2].(av.AudioCodecData).SampleRate())

	for _, s := range samples {
		pkt, err := demuxer.ReadPacket()
		require.NoError(t, err)
		require.Equal(t, s.idx, pkt.Idx, "packet index mismatch")
		require.Equal(t, s.data, pkt.Data)
	}
}

// TestRTMPMultiTrackPublishPlay publishes a synthetic OBS-style stream
// (video + legacy AAC track 0 + E-RTMP multitrack audio track 1) through the
// real RTMP server and asserts that a subscriber sees all three streams with
// stable indexes.
func TestRTMPMultiTrackPublishPlay(t *testing.T) {
	addr := "127.0.0.1:21935"

	server, err := New(Config{
		Addr: addr,
		App:  "/live",
	})
	require.NoError(t, err)

	go func() {
		_ = server.ListenAndServe()
	}()
	defer server.Close()

	aac0 := testAAC(t, 44100, 4)
	aac1 := testAAC(t, 48000, 3)
	h264 := testH264(t)

	streams := []av.CodecData{h264, aac0, aac1}

	var pub *joy4rtmp.Conn
	// retry the connect briefly until the listener is up
	for i := 0; i < 50; i++ {
		pub, err = joy4rtmp.Dial("rtmp://"+addr+"/live/test.stream", joy4rtmp.DialOptions{})
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	require.NoError(t, err)
	defer pub.Close()

	require.NoError(t, pub.WriteHeader(streams))

	stop := make(chan struct{})
	pubDone := make(chan error, 1)

	go func() {
		var terr error
		defer func() { pubDone <- terr }()

		for i := 0; i < 400; i++ {
			select {
			case <-stop:
				return
			default:
			}

			ts := int64(i) * 20
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

				terr = pub.WritePacket(av.Packet{
					Idx:        int8(idx),
					Data:       data,
					Time:       ts,
					IsKeyFrame: idx == 0,
				})
				if terr != nil {
					return
				}
			}

			time.Sleep(5 * time.Millisecond)
		}
	}()

	// wait until the server accepted the publish stream (its channel exists),
	// otherwise handlePlay rejects the subscriber as "not found"
	require.Eventually(t, func() bool {
		return len(server.Channels()) == 1
	}, 5*time.Second, 25*time.Millisecond)

	sub, err := joy4rtmp.Dial("rtmp://"+addr+"/live/test.stream", joy4rtmp.DialOptions{})
	require.NoError(t, err)
	defer sub.Close()

	got, err := sub.Streams()
	require.NoError(t, err)
	require.Equal(t, 3, len(got), "expected video + 2 audio streams")
	require.Equal(t, av.H264, got[0].Type())
	require.Equal(t, av.AAC, got[1].Type())
	require.Equal(t, av.AAC, got[2].Type())
	require.Equal(t, 44100, got[1].(av.AudioCodecData).SampleRate())
	require.Equal(t, 48000, got[2].(av.AudioCodecData).SampleRate())

	seen := map[int8]bool{}
	for i := 0; i < 30; i++ {
		pkt, err := sub.ReadPacket()
		require.NoError(t, err)
		require.True(t, pkt.Idx >= 0 && pkt.Idx <= 2)
		seen[pkt.Idx] = true
	}

	require.True(t, seen[0] && seen[1] && seen[2], "expected packets from all three streams, got %v", seen)

	close(stop)
	<-pubDone
}
