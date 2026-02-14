// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"simon-weij/wayland-recorder/lib"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	sourceTypeStr   string
	cursorModeStr   string
	outputPath      string
	codec           string
	container       string
	encoderSpeed    int
	quality         int
	audioMonitor    bool
	audioMic        bool
	clipMode        bool
	bufferDuration  int
	segmentDuration int
	tempDir         string
	noNotifications bool
)

const (
	sourceTypeMonitor uint32 = 1
	sourceTypeWindow  uint32 = 2
	sourceTypeBoth    uint32 = 3
)

func parseSourceType(s string) (uint32, error) {
	switch strings.ToLower(s) {
	case "monitor":
		return sourceTypeMonitor, nil
	case "window":
		return sourceTypeWindow, nil
	case "both":
		return sourceTypeBoth, nil
	default:
		return 0, fmt.Errorf("invalid source type: %s (use: monitor, window, or both)", s)
	}
}

const (
	cursorModeHidden   uint32 = 1
	cursorModeEmbedded uint32 = 2
	cursorModeMetadata uint32 = 4
)

func parseCursorMode(s string) (uint32, error) {
	switch strings.ToLower(s) {
	case "hidden":
		return cursorModeHidden, nil
	case "embedded":
		return cursorModeEmbedded, nil
	case "metadata":
		return cursorModeMetadata, nil
	default:
		return 0, fmt.Errorf("invalid cursor mode: %s (use: hidden, embedded, or metadata)", s)
	}
}

func fatalIfError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Start recording",
	Run: func(cmd *cobra.Command, args []string) {
		sourceType, err := parseSourceType(sourceTypeStr)
		fatalIfError(err)

		cursorMode, err := parseCursorMode(cursorModeStr)
		fatalIfError(err)

		conn, session, err := lib.CreateSession()
		fatalIfError(err)
		defer conn.Close()

		fatalIfError(lib.SelectSources(conn, session, sourceType, cursorMode))

		streams, err := lib.StartRecording(conn, session)
		fatalIfError(err)

		fmt.Printf("Recording stream %d\n", streams[0].NodeID)

		captureOpts := lib.CaptureOptions{
			OutputPath:      outputPath,
			Codec:           codec,
			Container:       container,
			EncoderSpeed:    encoderSpeed,
			Quality:         quality,
			AudioMonitor:    audioMonitor,
			AudioMic:        audioMic,
			ClipMode:        clipMode,
			BufferDuration:  bufferDuration,
			SegmentDuration: segmentDuration,
			TempDir:         tempDir,
			Notifications:   !noNotifications,
		}

		fatalIfError(lib.Capture(streams[0].NodeID, captureOpts))
	},
}

type recordDefaults struct {
	cursorMode      string
	codec           string
	container       string
	encoderSpeed    int
	quality         int
	audioMonitor    bool
	audioMic        bool
	bufferDuration  int
	segmentDuration int
	tempDir         string
	output          string
	notifications   bool
}

func getRecordDefaults() recordDefaults {
	defaults := recordDefaults{
		cursorMode:      "embedded",
		codec:           "h264",
		container:       "mp4",
		encoderSpeed:    6,
		quality:         5000000,
		audioMonitor:    true,
		audioMic:        true,
		bufferDuration:  30,
		segmentDuration: 5,
		tempDir:         "",
		output:          filepath.Join(os.Getenv("HOME"), "Videos", "recordings", "recording-"+time.Now().Format("2006-01-02-15-04-05")+".mp4"),
		notifications:   true,
	}

	settings, err := lib.LoadSettings()
	if err != nil || settings == nil {
		return defaults
	}

	if settings.CursorMode != "" {
		defaults.cursorMode = settings.CursorMode
	}
	if settings.Codec != "" {
		defaults.codec = settings.Codec
	}
	if settings.Container != "" {
		defaults.container = settings.Container
	}
	if settings.EncoderSpeed != 0 {
		defaults.encoderSpeed = settings.EncoderSpeed
	}
	if settings.Quality != 0 {
		defaults.quality = settings.Quality
	}
	if settings.BufferDuration != 0 {
		defaults.bufferDuration = settings.BufferDuration
	}
	if settings.SegmentDuration != 0 {
		defaults.segmentDuration = settings.SegmentDuration
	}
	if settings.TempDir != "" {
		defaults.tempDir = settings.TempDir
	}

	defaults.audioMonitor = settings.AudioMonitor
	defaults.audioMic = settings.AudioMic
	defaults.notifications = settings.Notifications

	if settings.OutputPath != "" {
		defaults.output = filepath.Join(settings.OutputPath, "recording-"+time.Now().Format("2006-01-02-15-04-05")+"."+settings.Container)
	}

	return defaults
}

func init() {
	rootCmd.AddCommand(recordCmd)
	defaults := getRecordDefaults()

	recordCmd.Flags().StringVarP(&sourceTypeStr, "source", "s", "monitor", "Source type: monitor, window, or both")
	recordCmd.Flags().StringVarP(&cursorModeStr, "cursor", "c", defaults.cursorMode, "Cursor mode: hidden, embedded, or metadata")
	recordCmd.Flags().StringVarP(&outputPath, "output", "o", defaults.output, "Output file path")
	recordCmd.Flags().StringVar(&codec, "codec", defaults.codec, "Video codec: vp8, vp9, h264, x264")
	recordCmd.Flags().StringVar(&container, "container", defaults.container, "Container format: webm, mp4, mkv")
	recordCmd.Flags().IntVar(&encoderSpeed, "speed", defaults.encoderSpeed, "Encoder speed/deadline (higher = better quality, slower)")
	recordCmd.Flags().IntVar(&quality, "quality", defaults.quality, "Target bitrate in bits/second (0=codec default)")
	recordCmd.Flags().BoolVar(&audioMonitor, "audio-monitor", defaults.audioMonitor, "Record system audio (monitor)")
	recordCmd.Flags().BoolVar(&audioMic, "audio-mic", defaults.audioMic, "Record microphone audio")
	recordCmd.Flags().BoolVar(&clipMode, "clip-mode", false, "Enable clip mode (buffer recording and save clips on signal)")
	recordCmd.Flags().IntVar(&bufferDuration, "buffer-duration", defaults.bufferDuration, "Duration in seconds to keep buffered for clipping")
	recordCmd.Flags().IntVar(&segmentDuration, "segment-duration", defaults.segmentDuration, "Duration in seconds for each segment file")
	recordCmd.Flags().StringVar(&tempDir, "temp-dir", defaults.tempDir, "Temporary directory for segments (default: system temp)")
	recordCmd.Flags().BoolVar(&noNotifications, "no-notifications", !defaults.notifications, "Disable notifications")
}
