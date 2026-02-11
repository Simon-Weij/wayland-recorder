// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func MergeSegments(segments []SegmentInfo, outputPath string) error {
	if len(segments) == 0 {
		return fmt.Errorf("no segments to merge")
	}

	fmt.Printf("Creating clip from %d segments...\n", len(segments))

	concatFilePath, err := createConcatFile(segments)
	if err != nil {
		return err
	}
	defer os.Remove(concatFilePath)

	if err := runFFmpegConcat(concatFilePath, outputPath); err != nil {
		return err
	}

	fmt.Printf("Clip saved to: %s\n", outputPath)
	return nil
}

func createConcatFile(segments []SegmentInfo) (string, error) {
	concatFile := filepath.Join(filepath.Dir(segments[0].Path), "concat_list.txt")

	f, err := os.Create(concatFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	validSegments := 0
	for _, seg := range segments {
		info, err := os.Stat(seg.Path)
		if os.IsNotExist(err) {
			continue
		}
		if info.Size() < 1024 {
			continue
		}
		fmt.Fprintf(f, "file '%s'\n", seg.Path)
		validSegments++
	}

	if validSegments == 0 {
		return "", fmt.Errorf("no valid segments found")
	}

	return concatFile, nil
}

func runFFmpegConcat(concatFile, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", concatFile,
		"-c", "copy",
		"-y",
		outputPath,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to merge segments: %v", err)
	}

	return nil
}
