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

	for _, seg := range segments {
		if _, err := os.Stat(seg.Path); os.IsNotExist(err) {
			continue
		}
		fmt.Fprintf(f, "file '%s'\n", seg.Path)
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
