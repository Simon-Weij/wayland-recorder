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
	kept := make([]SegmentInfo, 0, len(sm.segments))

	for _, seg := range sm.segments {
		if seg.StartTime.After(cutoffTime) {
			kept = append(kept, seg)
		} else {
			os.Remove(seg.Path)
		}
	}
	sm.segments = kept
}

func (sm *SegmentManager) GetRecentSegments(duration time.Duration) []SegmentInfo {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cutoffTime := time.Now().Add(-duration)
	recent := make([]SegmentInfo, 0)

	for _, seg := range sm.segments {
		if seg.StartTime.After(cutoffTime) {
			recent = append(recent, seg)
		}
	}
	return recent
}

func MonitorSegments(tempDir string, container string, manager *SegmentManager) {
	if manager == nil || tempDir == "" || container == "" {
		return
	}

	pattern := filepath.Join(tempDir, "segment_*."+container)
	tracker := newSegmentTracker()

	for {
		time.Sleep(1 * time.Second)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			tracker.processSegment(match, manager)
		}
	}
}

type segmentTracker struct {
	seen        map[string]bool
	fileSize    map[string]int64
	nextNumber  int
}

func newSegmentTracker() *segmentTracker {
	return &segmentTracker{
		seen:       make(map[string]bool),
		fileSize:   make(map[string]int64),
		nextNumber: 0,
	}
}

func (st *segmentTracker) processSegment(path string, manager *SegmentManager) bool {
	if st.seen[path] {
		return false
	}

	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	prevSize, exists := st.fileSize[path]
	if exists && prevSize == info.Size() && info.Size() > minSegmentSize {
		manager.AddSegment(path, st.nextNumber)
		st.seen[path] = true
		delete(st.fileSize, path)
		st.nextNumber++
		return true
	}
	
	st.fileSize[path] = info.Size()
	return false
}
