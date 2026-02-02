// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SegmentInfo struct {
	Path      string
	StartTime time.Time
	Number    int
}

type SegmentManager struct {
	segments    []SegmentInfo
	maxDuration time.Duration
	tempDir     string
	mu          sync.Mutex
}

func NewSegmentManager(maxDuration time.Duration, tempDir string) *SegmentManager {
	return &SegmentManager{
		segments:    make([]SegmentInfo, 0),
		maxDuration: maxDuration,
		tempDir:     tempDir,
	}
}

func (sm *SegmentManager) AddSegment(path string, number int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	segment := SegmentInfo{
		Path:      path,
		StartTime: time.Now(),
		Number:    number,
	}
	sm.segments = append(sm.segments, segment)

	sm.cleanupOldSegments()
}

func (sm *SegmentManager) cleanupOldSegments() {
	cutoffTime := time.Now().Add(-sm.maxDuration)
	newSegments := make([]SegmentInfo, 0)

	for _, seg := range sm.segments {
		if seg.StartTime.After(cutoffTime) {
			newSegments = append(newSegments, seg)
		} else {
			os.Remove(seg.Path)
		}
	}
	sm.segments = newSegments
}

func (sm *SegmentManager) GetRecentSegments(duration time.Duration) []SegmentInfo {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoffTime := time.Now().Add(-duration)
	recentSegments := make([]SegmentInfo, 0)

	for _, seg := range sm.segments {
		if seg.StartTime.After(cutoffTime) {
			recentSegments = append(recentSegments, seg)
		}
	}
	return recentSegments
}

func MonitorSegments(tempDir string, container string, manager *SegmentManager) {
	pattern := filepath.Join(tempDir, "segment_*."+container)
	seenSegments := make(map[string]bool)
	segmentNumber := 0

	for {
		time.Sleep(1 * time.Second)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			if !seenSegments[match] {
				info, err := os.Stat(match)
				if err != nil {
					continue
				}
				if time.Since(info.ModTime()) > 2*time.Second {
					manager.AddSegment(match, segmentNumber)
					seenSegments[match] = true
					segmentNumber++
				}
			}
		}
	}
}
