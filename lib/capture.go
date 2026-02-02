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
	DefaultFilePermissions = 0755
)

func Capture(nodeID uint32, opts CaptureOptions) error {
	if err := ensureOutputDirectory(opts.OutputPath); err != nil {
		return err
	}

	applyDefaults(&opts)

	segmentManager := setupSegmentManager(&opts)
	args, err := BuildGStreamerArgs(nodeID, opts)
	if err != nil {
		return err
	}

	return startRecording(args, opts, segmentManager)
}

func ensureOutputDirectory(outputPath string) error {
	directoryPath := filepath.Dir(outputPath)
	return os.MkdirAll(directoryPath, DefaultFilePermissions)
}

func applyDefaults(opts *CaptureOptions) {
	if !opts.ClipMode {
		return
	}

	if opts.TempDir == "" {
		opts.TempDir = filepath.Join(os.TempDir(), fmt.Sprintf("wayland-recorder-%d", os.Getpid()))
	}
	if opts.SegmentDuration == 0 {
		opts.SegmentDuration = 5
	}
	if opts.BufferDuration == 0 {
		opts.BufferDuration = 30
	}
}

func setupSegmentManager(opts *CaptureOptions) *SegmentManager {
	if !opts.ClipMode {
		return nil
	}

	if err := os.MkdirAll(opts.TempDir, DefaultFilePermissions); err != nil {
		return nil
	}

	maxDuration := time.Duration(opts.BufferDuration+opts.SegmentDuration) * time.Second
	return NewSegmentManager(maxDuration, opts.TempDir)
}

func startRecording(args []string, opts CaptureOptions, segmentManager *SegmentManager) error {
	cmd := exec.Command(GStreamerCommand, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	printRecordingInfo(opts)

	if opts.ClipMode && segmentManager != nil {
		go MonitorSegments(opts.TempDir, opts.Container, segmentManager)
	}

	return handleRecordingSignals(cmd, opts, segmentManager)
}

func printRecordingInfo(opts CaptureOptions) {
	if opts.ClipMode {
		fmt.Printf("Recording with %d second buffer...\n", opts.BufferDuration)
		fmt.Printf("Segments stored in: %s\n", opts.TempDir)
		fmt.Printf("Send SIGUSR1 to create a clip: kill -SIGUSR1 %d\n", os.Getpid())
		fmt.Println("Press Ctrl+C to stop recording")
	} else {
		fmt.Printf("Recording to: %s\n", opts.OutputPath)
		fmt.Println("Press Ctrl+C to stop")
	}
}

func handleRecordingSignals(cmd *exec.Cmd, opts CaptureOptions, segmentManager *SegmentManager) error {
	interruptSignal := make(chan os.Signal, 1)
	clipSignal := make(chan os.Signal, 1)
	finished := make(chan error, 1)

	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)
	if opts.ClipMode {
		signal.Notify(clipSignal, syscall.SIGUSR1)
	}

	go func() {
		finished <- cmd.Wait()
	}()

	clipCounter := 1
	for {
		select {
		case <-clipSignal:
			handleClipRequest(opts, segmentManager, &clipCounter)

		case <-interruptSignal:
			return handleInterrupt(cmd, opts)

		case err := <-finished:
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

	clipPath := generateClipPath(opts.OutputPath, *clipCounter)
	*clipCounter++

	go createClipAsync(segments, clipPath)
}

func generateClipPath(basePath string, counter int) string {
	dir := filepath.Dir(basePath)
	ext := filepath.Ext(basePath)
	base := strings.TrimSuffix(filepath.Base(basePath), ext)
	return filepath.Join(dir, fmt.Sprintf("%s-clip-%03d%s", base, counter, ext))
}

func createClipAsync(segments []SegmentInfo, outputPath string) {
	if err := MergeSegments(segments, outputPath); err != nil {
		fmt.Printf("[CLIP] Error creating clip: %v\n", err)
	}
}

func handleInterrupt(cmd *exec.Cmd, opts CaptureOptions) error {
	fmt.Println("\nStopping recording and finalizing...")
	cmd.Process.Signal(syscall.SIGINT)

	cmd.Wait()

	if opts.ClipMode && opts.TempDir != "" {
		cleanupTempFiles(opts.TempDir)
	}

	fmt.Println("Stopped")
	return nil
}

func handleFinished(err error) error {
	if err != nil {
		return err
	}
	fmt.Println("Done")
	return nil
}

func cleanupTempFiles(tempDir string) {
	fmt.Println("Cleaning up temporary segments...")
	os.RemoveAll(tempDir)
}
