// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "wayland-recorder",
	Short: "Record your screen on Wayland",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if os.Getenv("WAYLAND_DISPLAY") == "" {
			return fmt.Errorf("requires Wayland session")
		}
		if exec.Command("pgrep", "-f", "pipewire").Run() != nil {
			return fmt.Errorf("requires PipeWire")
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
