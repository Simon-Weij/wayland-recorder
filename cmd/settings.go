// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	CursorMode      string `json:"cursorMode"`
	OutputPath      string `json:"outputPath"`
	Hotkey          string `json:"hotkey"`
	Codec           string `json:"codec"`
	Container       string `json:"container"`
	EncoderSpeed    int    `json:"encoderSpeed"`
	Quality         int    `json:"quality"`
	AudioMonitor    bool   `json:"audioMonitor"`
	AudioMic        bool   `json:"audioMic"`
	BufferDuration  int    `json:"bufferDuration"`
	SegmentDuration int    `json:"segmentDuration"`
	TempDir         string `json:"tempDir"`
}

func loadSettings() (*Settings, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".config", "wayland-recorder", "settings.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}
