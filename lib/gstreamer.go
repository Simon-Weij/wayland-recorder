// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	gstreamerCommand = "gst-launch-1.0"
	minSegmentSize   = 1024
)

type CaptureOptions struct {
	OutputPath      string
	Codec           string
	Container       string
	EncoderSpeed    int
	Quality         int
	AudioMonitor    bool
	AudioMic        bool
	BufferDuration  int
	SegmentDuration int
	ClipMode        bool
	TempDir         string
	Notifications   bool
}

func BuildGStreamerArgs(nodeID uint32, opts CaptureOptions) ([]string, error) {
	if nodeID == 0 {
		return nil, fmt.Errorf("invalid node ID: 0")
	}

	args := []string{"-e"}

	args = append(args, "pipewiresrc", fmt.Sprintf("path=%d", nodeID), "!", "videoconvert", "!", "queue")

	encoderArgs, err := buildEncoderArgs(opts.Codec, opts.EncoderSpeed, opts.Quality)
	if err != nil {
		return nil, fmt.Errorf("encoder configuration error: %w", err)
	}
	args = append(args, encoderArgs...)

	args = appendAudioAndOutput(args, opts)

	return args, nil
}

func buildEncoderArgs(codec string, encoderSpeed int, quality int) ([]string, error) {
	var args []string

	switch codec {
	case "vp8":
		args = []string{"!", "vp8enc", fmt.Sprintf("deadline=%d", encoderSpeed)}
		if quality > 0 {
			args = append(args, fmt.Sprintf("target-bitrate=%d", quality))
		}
	case "vp9":
		args = []string{"!", "vp9enc", fmt.Sprintf("deadline=%d", encoderSpeed)}
		if quality > 0 {
			args = append(args, fmt.Sprintf("target-bitrate=%d", quality))
		}
	case "h264", "x264":
		args = []string{"!", "x264enc", fmt.Sprintf("speed-preset=%d", encoderSpeed)}
		if quality > 0 {
			if quality < 1000 {
				return nil, fmt.Errorf("quality for h264/x264 must be >= 1000 bps")
			}
			bitrateKbps := quality / 1000
			args = append(args, fmt.Sprintf("bitrate=%d", bitrateKbps))
		}
	default:
		return nil, fmt.Errorf("unsupported codec: %s (use: vp8, vp9, h264, or x264)", codec)
	}

	return args, nil
}

type muxerConfig struct {
	name             string
	streamableParams string
}

var muxerConfigs = map[string]muxerConfig{
	"webm": {"webmmux", "streamable=true"},
	"mp4":  {"mp4mux", "fragment-duration=1000 streamable=true faststart=true"},
	"mkv":  {"matroskamux", "streamable=true"},
}

func getMuxerConfig(container string) (muxerConfig, error) {
	config, exists := muxerConfigs[container]
	if !exists {
		return muxerConfig{}, fmt.Errorf("unsupported container: %s (use: webm, mp4, or mkv)", container)
	}
	return config, nil
}

func buildMuxerArgs(container string) ([]string, error) {
	config, err := getMuxerConfig(container)
	if err != nil {
		return nil, err
	}
	args := []string{"!", config.name}
	if config.streamableParams != "" {
		args = append(args, strings.Fields(config.streamableParams)...)
	}
	args = append(args, "name=mux")
	return args, nil
}

func buildAudioPipeline(opts CaptureOptions) []string {
	if opts.ClipMode || (!opts.AudioMonitor && !opts.AudioMic) {
		return nil
	}

	var pipeline []string

	if opts.AudioMonitor && opts.AudioMic {
		pipeline = []string{
			"audiomixer", "name=mix",
			"pulsesrc", "device=@DEFAULT_MONITOR@", "!", "queue", "!", "audioconvert", "!", "mix.",
			"pulsesrc", "device=@DEFAULT_SOURCE@", "!", "queue", "!", "audioconvert", "!", "mix.",
			"mix.", "!", "audioresample", "!", "opusenc",
		}
	} else {
		device := "@DEFAULT_MONITOR@"
		if opts.AudioMic {
			device = "@DEFAULT_SOURCE@"
		}
		pipeline = []string{
			"pulsesrc", fmt.Sprintf("device=%s", device),
			"!", "queue", "!", "audioconvert", "!", "audioresample", "!", "opusenc",
		}
	}

	return pipeline
}

func appendAudioAndOutput(args []string, opts CaptureOptions) []string {
	audioPipeline := buildAudioPipeline(opts)

	if len(audioPipeline) == 0 {
		return appendOutputSink(args, opts, "")
	}

	if opts.ClipMode {
		args = append(args, audioPipeline...)
		return appendOutputSink(args, opts, "")
	}

	muxerArgs, err := buildMuxerArgs(opts.Container)
	if err != nil {
		return appendOutputSink(args, opts, "")
	}

	args = append(args, muxerArgs...)
	args = append(args, audioPipeline...)
	args = append(args, "!", "mux.")
	return appendOutputSink(args, opts, "mux.")
}

func appendOutputSink(args []string, opts CaptureOptions, prefix string) []string {
	if prefix != "" {
		args = append(args, prefix, "!")
	} else {
		args = append(args, "!")
	}

	if opts.ClipMode {
		config, err := getMuxerConfig(opts.Container)
		if err != nil {
			args = append(args, "filesink", fmt.Sprintf("location=%s", opts.OutputPath))
			return args
		}

		segmentPattern := filepath.Join(opts.TempDir, "segment_%05d."+opts.Container)
		maxSizeTime := opts.SegmentDuration * 1000000000

		args = append(args, "splitmuxsink",
			fmt.Sprintf("muxer=%s", config.name),
			fmt.Sprintf("location=%s", segmentPattern),
			fmt.Sprintf("max-size-time=%d", maxSizeTime))
	} else {
		args = append(args, "filesink", fmt.Sprintf("location=%s", opts.OutputPath))
	}

	return args
}
