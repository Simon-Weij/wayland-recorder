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
)

func parseSourceType(s string) (uint32, error) {
	switch strings.ToLower(s) {
	case "monitor":
		return 1, nil
	case "window":
		return 2, nil
	case "both":
		return 3, nil
	default:
		return 0, fmt.Errorf("invalid source type: %s (use: monitor, window, or both)", s)
	}
}

func parseCursorMode(s string) (uint32, error) {
	switch strings.ToLower(s) {
	case "hidden":
		return 1, nil
	case "embedded":
		return 2, nil
	case "metadata":
		return 4, nil
	default:
		return 0, fmt.Errorf("invalid cursor mode: %s (use: hidden, embedded, or metadata)", s)
	}
}

var recordCmd = &cobra.Command{
	Use:   "record",
	Short: "Start recording",
	Run: func(cmd *cobra.Command, args []string) {
		sourceType, err := parseSourceType(sourceTypeStr)
		if err != nil {
			log.Fatal(err)
		}

		cursorMode, err := parseCursorMode(cursorModeStr)
		if err != nil {
			log.Fatal(err)
		}

		conn, session, err := lib.CreateSession()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		if err := lib.SelectSources(conn, session, sourceType, cursorMode); err != nil {
			log.Fatal(err)
		}

		streams, err := lib.StartRecording(conn, session)
		if err != nil {
			log.Fatal(err)
		}

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
		}
		if err := lib.Capture(streams[0].NodeID, captureOpts); err != nil {
			log.Fatal(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(recordCmd)

	settings, err := loadSettings()
	if err != nil {
		settings = nil
	}
	
	defaultCursorMode := "embedded"
	defaultCodec := "h264"
	defaultContainer := "mp4"
	defaultEncoderSpeed := 6
	defaultQuality := 5000000
	defaultAudioMonitor := true
	defaultAudioMic := true
	defaultBufferDuration := 30
	defaultSegmentDuration := 5
	defaultTempDir := ""
	defaultOutput := filepath.Join(os.Getenv("HOME"), "Videos", "recordings", "recording-"+time.Now().Format("2006-01-02-15-04-05")+".mp4")

	if err == nil && settings != nil {
		if settings.CursorMode != "" {
			defaultCursorMode = settings.CursorMode
		}
		if settings.OutputPath != "" {
			defaultOutput = filepath.Join(settings.OutputPath, "recording-"+time.Now().Format("2006-01-02-15-04-05")+"."+settings.Container)
		}
		if settings.Codec != "" {
			defaultCodec = settings.Codec
		}
		if settings.Container != "" {
			defaultContainer = settings.Container
		}
		if settings.EncoderSpeed != 0 {
			defaultEncoderSpeed = settings.EncoderSpeed
		}
		if settings.Quality != 0 {
			defaultQuality = settings.Quality
		}

		defaultAudioMonitor = settings.AudioMonitor
		defaultAudioMic = settings.AudioMic
		if settings.BufferDuration != 0 {
			defaultBufferDuration = settings.BufferDuration
		}
		if settings.SegmentDuration != 0 {
			defaultSegmentDuration = settings.SegmentDuration
		}
		if settings.TempDir != "" {
			defaultTempDir = settings.TempDir
		}
	}

	recordCmd.Flags().StringVarP(&sourceTypeStr, "source", "s", "monitor", "Source type: monitor, window, or both")
	recordCmd.Flags().StringVarP(&cursorModeStr, "cursor", "c", defaultCursorMode, "Cursor mode: hidden, embedded, or metadata")

	recordCmd.Flags().StringVarP(&outputPath, "output", "o", defaultOutput, "Output file path")
	recordCmd.Flags().StringVar(&codec, "codec", defaultCodec, "Video codec: vp8, vp9, h264, x264")
	recordCmd.Flags().StringVar(&container, "container", defaultContainer, "Container format: webm, mp4, mkv")

	recordCmd.Flags().IntVar(&encoderSpeed, "speed", defaultEncoderSpeed, "Encoder speed/deadline (higher = better quality, slower)")
	recordCmd.Flags().IntVar(&quality, "quality", defaultQuality, "Target bitrate in bits/second (0=codec default)")

	recordCmd.Flags().BoolVar(&audioMonitor, "audio-monitor", defaultAudioMonitor, "Record system audio (monitor)")
	recordCmd.Flags().BoolVar(&audioMic, "audio-mic", defaultAudioMic, "Record microphone audio")

	recordCmd.Flags().BoolVar(&clipMode, "clip-mode", false, "Enable clip mode (buffer recording and save clips on signal)")
	recordCmd.Flags().IntVar(&bufferDuration, "buffer-duration", defaultBufferDuration, "Duration in seconds to keep buffered for clipping")
	recordCmd.Flags().IntVar(&segmentDuration, "segment-duration", defaultSegmentDuration, "Duration in seconds for each segment file")
	recordCmd.Flags().StringVar(&tempDir, "temp-dir", defaultTempDir, "Temporary directory for segments (default: system temp)")
}
