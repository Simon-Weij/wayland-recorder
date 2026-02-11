// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"log"
	"simon-weij/wayland-recorder/lib"

	"github.com/spf13/cobra"
)

var (
	shortcutKey string
)

var shortcutCmd = &cobra.Command{
	Use:   "shortcut",
	Short: "Register a global shortcut to start recording",
	Run: func(cmd *cobra.Command, args []string) {
		key := getShortcutKey()
		fatalIfError(lib.RegisterShortcut(key, "Start screen recording"))
	},
}

func getShortcutKey() string {
	if shortcutKey != "" {
		return shortcutKey
	}

	settings, err := loadSettings()
	if err == nil && settings != nil && settings.Hotkey != "" {
		fmt.Printf("Using shortcut from settings: %s\n", settings.Hotkey)
		return settings.Hotkey
	}

	log.Fatal("No shortcut key specified. Use --key flag or set 'hotkey' in settings.json")
	return ""
}

func init() {
	rootCmd.AddCommand(shortcutCmd)

	shortcutCmd.Flags().StringVarP(&shortcutKey, "key", "k", "", "Shortcut key combination (e.g., 'alt+z', 'ctrl+shift+r')")
}
