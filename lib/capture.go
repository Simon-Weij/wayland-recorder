// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	defaultFilePermissions = 0755
	defaultSegmentDuration = 5
	defaultBufferDuration  = 30
)

func Capture(nodeID uint32, opts CaptureOptions) error {
	if err := ensureOutputDirectory(opts.OutputPath); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	applyDefaults(&opts)

	segmentManager := setupSegmentManager(&opts)
	args, err := BuildGStreamerArgs(nodeID, opts)
	if err != nil {
		return fmt.Errorf("failed to build GStreamer arguments: %w", err)
	}

	return startRecording(args, opts, segmentManager)
}

func ensureOutputDirectory(outputPath string) error {
	directoryPath := filepath.Dir(outputPath)
	return os.MkdirAll(directoryPath, defaultFilePermissions)
}

func applyDefaults(opts *CaptureOptions) {
	if !opts.ClipMode {
		return
	}

	if opts.Container == "mp4" || opts.Container == "mkv" {
		fmt.Println("Note: Using WebM container for clip mode")
		opts.Container = "webm"
	}
	if opts.Codec == "h264" || opts.Codec == "x264" {
		opts.Codec = "vp9"
	}

	if opts.TempDir == "" {
		opts.TempDir = filepath.Join(os.TempDir(), fmt.Sprintf("wayland-recorder-%d", os.Getpid()))
	}
	if opts.SegmentDuration == 0 {
		opts.SegmentDuration = defaultSegmentDuration
	}
	if opts.BufferDuration == 0 {
		opts.BufferDuration = defaultBufferDuration
	}
}

func setupSegmentManager(opts *CaptureOptions) *SegmentManager {
	if !opts.ClipMode {
		return nil
	}

	if err := os.MkdirAll(opts.TempDir, defaultFilePermissions); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create temp directory: %v\n", err)
		return nil
	}

	maxDuration := time.Duration(opts.BufferDuration+opts.SegmentDuration) * time.Second
	return NewSegmentManager(maxDuration, opts.TempDir)
}

func startRecording(args []string, opts CaptureOptions, segmentManager *SegmentManager) error {
	cmd := exec.Command(gstreamerCommand, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GStreamer: %w", err)
	}

	if opts.ClipMode {
		writePidFile()
	}

	printRecordingInfo(opts)

	if opts.ClipMode && segmentManager != nil {
		go MonitorSegments(opts.TempDir, opts.Container, segmentManager)
	}

	signals := setupSignalChannels(opts.ClipMode)
	go func() {
		signals.finished <- cmd.Wait()
	}()

	return processSignals(cmd, opts, segmentManager, signals)
}

func printRecordingInfo(opts CaptureOptions) {
	if opts.ClipMode {
		fmt.Printf("Recording with %d second buffer...\n", opts.BufferDuration)
		fmt.Printf("Segments stored in: %s\n", opts.TempDir)
		fmt.Printf("Send SIGUSR1 to create a clip: kill -SIGUSR1 %d\n", os.Getpid())
		fmt.Printf("PID: %d\n", os.Getpid())
		fmt.Println("Press Ctrl+C to stop recording")
	} else {
		fmt.Printf("Recording to: %s\n", opts.OutputPath)
		fmt.Println("Press Ctrl+C to stop")
	}
}

type signalChannels struct {
	interrupt chan os.Signal
	clip      chan os.Signal
	finished  chan error
}

func setupSignalChannels(clipMode bool) signalChannels {
	channels := signalChannels{
		interrupt: make(chan os.Signal, 1),
		clip:      make(chan os.Signal, 1),
		finished:  make(chan error, 1),
	}
	
	signal.Notify(channels.interrupt, os.Interrupt, syscall.SIGTERM)
	if clipMode {
		signal.Notify(channels.clip, syscall.SIGUSR1)
	}
	
	return channels
}

func processSignals(cmd *exec.Cmd, opts CaptureOptions, segmentManager *SegmentManager, signals signalChannels) error {
	clipCounter := 1
	for {
		select {
		case <-signals.clip:
			handleClipRequest(opts, segmentManager, &clipCounter)

		case <-signals.interrupt:
			return handleInterrupt(cmd, opts)

		case err := <-signals.finished:
			return handleFinished(err)
		}
	}
}

func handleClipRequest(opts CaptureOptions, segmentManager *SegmentManager, clipCounter *int) {
	if segmentManager == nil {
		return
	}

	fmt.Printf("\n[CLIP] Creating clip of last %d seconds...\n", opts.BufferDuration)
	duration := time.Duration(opts.BufferDuration) * time.Second
	segments := segmentManager.GetRecentSegments(duration)

	if len(segments) == 0 {
		fmt.Println("[CLIP] No segments available yet, wait a bit longer")
		return
	}

	clipPath := generateClipPath(opts.OutputPath, opts.Container, *clipCounter)
	*clipCounter++

	go createClipAsync(segments, clipPath)
}

func generateClipPath(basePath, container string, counter int) string {
	dir := filepath.Dir(basePath)
	base := strings.TrimSuffix(filepath.Base(basePath), filepath.Ext(basePath))
	return filepath.Join(dir, fmt.Sprintf("%s-clip-%03d.%s", base, counter, container))
}

func createClipAsync(segments []SegmentInfo, outputPath string) {
	if err := MergeSegments(segments, outputPath); err != nil {
		fmt.Printf("[CLIP] Error creating clip: %v\n", err)
	}
}

func handleInterrupt(cmd *exec.Cmd, opts CaptureOptions) error {
	fmt.Println("\nStopping recording and finalizing...")
	
	if cmd.Process == nil {
		return fmt.Errorf("process not started")
	}
	
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		return fmt.Errorf("failed to send interrupt signal: %w", err)
	}

	cmd.Wait()

	if opts.ClipMode && opts.TempDir != "" {
		cleanupTempFiles(opts.TempDir)
		cleanupPidFile()
	}

	fmt.Println("Stopped")
	return nil
}

func handleFinished(err error) error {
	if err != nil {
		return err
	}
	fmt.Println("Done")
	cleanupPidFile()
	return nil
}

func cleanupTempFiles(tempDir string) {
	fmt.Println("Cleaning up temporary segments...")
	os.RemoveAll(tempDir)
}

func writePidFile() {
	_ = os.WriteFile(getPidFilePath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func cleanupPidFile() {
	_ = os.Remove(getPidFilePath())
}

func getPidFilePath() string {
	if runtimeDir := os.Getenv("XDG_RUNTIME_DIR"); runtimeDir != "" {
		return filepath.Join(runtimeDir, "wayland-recorder.pid")
	}
	return "/tmp/wayland-recorder.pid"
}
