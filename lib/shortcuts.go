// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	GlobalShortcutsPortal = "org.freedesktop.portal.GlobalShortcuts"
)

type shortcutStruct struct {
	ID   string
	Data map[string]dbus.Variant
}

var modifierMap = map[string]string{
	"ctrl":    "Control",
	"control": "Control",
	"alt":     "Alt",
	"shift":   "Shift",
	"super":   "Super",
	"meta":    "Super",
	"win":     "Super",
}

func ParseShortcut(shortcut string) (string, error) {
	normalized := strings.TrimSpace(strings.ReplaceAll(shortcut, " + ", "+"))
	if normalized == "" {
		return "", fmt.Errorf("empty shortcut")
	}

	parts := strings.Split(normalized, "+")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid shortcut format")
	}

	var modifiers []string
	var key string

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if modifier, ok := modifierMap[strings.ToLower(part)]; ok {
			modifiers = append(modifiers, modifier)
		} else {
			if key != "" {
				return "", fmt.Errorf("multiple keys specified: %s and %s", key, part)
			}
			if len(part) == 1 {
				key = strings.ToUpper(part)
			} else {
				key = part
			}
		}
	}

	if key == "" {
		return "", fmt.Errorf("no key specified in shortcut")
	}

	var builder strings.Builder
	for _, mod := range modifiers {
		builder.WriteString("<")
		builder.WriteString(mod)
		builder.WriteString(">")
	}
	builder.WriteString(key)

	return builder.String(), nil
}

func createShortcutSession(conn *dbus.Conn, portal dbus.BusObject) (dbus.ObjectPath, error) {
	sessionOptions := map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(generateToken()),
		"handle_token":         dbus.MakeVariant(generateToken()),
	}

	var requestPath dbus.ObjectPath
	err := portal.Call("org.freedesktop.portal.GlobalShortcuts.CreateSession", 0, sessionOptions).Store(&requestPath)
	if err != nil {
		return "", fmt.Errorf("failed to create shortcuts session: %w", err)
	}

	response, err := waitForResponse(conn, requestPath)
	if err != nil {
		return "", fmt.Errorf("failed to get session response: %w", err)
	}

	sessionHandle := response["session_handle"].Value().(string)
	return dbus.ObjectPath(sessionHandle), nil
}

func bindShortcut(conn *dbus.Conn, portal dbus.BusObject, sessionPath dbus.ObjectPath, parsedShortcut, description string) error {
	shortcuts := []shortcutStruct{{
		ID: "record-shortcut",
		Data: map[string]dbus.Variant{
			"description":       dbus.MakeVariant(description),
			"preferred_trigger": dbus.MakeVariant(parsedShortcut),
		},
	}}

	bindOptions := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(generateToken()),
	}

	var bindRequestPath dbus.ObjectPath
	err := portal.Call("org.freedesktop.portal.GlobalShortcuts.BindShortcuts", 0,
		sessionPath, shortcuts, "", bindOptions).Store(&bindRequestPath)
	if err != nil {
		return fmt.Errorf("failed to bind shortcut: %w", err)
	}

	bindResponse, err := waitForResponse(conn, bindRequestPath)
	if err != nil {
		return fmt.Errorf("failed to get bind response: %w", err)
	}

	printBindResult(bindResponse, parsedShortcut)
	return nil
}

func printBindResult(bindResponse map[string]dbus.Variant, parsedShortcut string) {
	shortcutsVariant, ok := bindResponse["shortcuts"]
	if !ok {
		return
	}

	bindShortcuts, ok := shortcutsVariant.Value().([]map[string]dbus.Variant)
	if ok && len(bindShortcuts) > 0 {
		fmt.Printf("New shortcut registered: %s\n", parsedShortcut)
	}
}

func listenForActivation(conn *dbus.Conn, sessionPath dbus.ObjectPath, execPath string) error {
	signalChannel := make(chan *dbus.Signal, 10)
	conn.Signal(signalChannel)

	matchRule := fmt.Sprintf("type='signal',interface='org.freedesktop.portal.GlobalShortcuts',member='Activated',path='%s'", sessionPath)
	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule)
	defer conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, matchRule)

	fmt.Println("Listening for shortcut activation... (Press Ctrl+C to stop)")

	for signal := range signalChannel {
		if signal.Name == "org.freedesktop.portal.GlobalShortcuts.Activated" {
			handleShortcutActivation(execPath)
		}
	}

	return nil
}

func handleShortcutActivation(execPath string) {
	fmt.Println("Shortcut activated! Sending clip signal...")

	pid, err := findRecordingProcess()
	if err != nil {
		fmt.Printf("Failed to find recording process: %v\n", err)
		fmt.Println("Is the recording process running with --clip-mode?")
		return
	}

	if err := sendClipSignal(pid); err != nil {
		fmt.Printf("Failed to send signal to process %d: %v\n", pid, err)
		return
	}

	fmt.Printf("Clip signal sent to process %d\n", pid)
}

func sendClipSignal(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	err = process.Signal(syscall.SIGUSR1)
	if err == nil && notificationsEnabled(pid) {
		cmd := exec.Command(
			"notify-send",
			"wayland-recorder",
			"New clip!",
			"-u",
			"normal",
			"-t",
			"5000",
		)
		_ = cmd.Run()
	}

	return err
}

func notificationsEnabled(pid int) bool {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err == nil {
		cmdStr := string(cmdline)
		if strings.Contains(cmdStr, "--no-notifications") {
			return false
		}
	}

	if settings, err := LoadSettings(); err == nil {
		return settings.Notifications
	}

	return true
}

func findRecordingProcess() (int, error) {
	if pid, err := pidFromFile(); err == nil {
		return pid, nil
	}

	if pid, err := pidFromProcScan(); err == nil {
		return pid, nil
	}

	return 0, fmt.Errorf("no clip-mode recording found; start with --clip-mode")
}

func pidFromFile() (int, error) {
	data, err := os.ReadFile(getPidFilePath())
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, err
	}

	if !processLooksLikeClipMode(pid) {
		return 0, fmt.Errorf("pid file does not point to clip-mode process")
	}

	return pid, nil
}

func pidFromProcScan() (int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		var pid int
		if _, err := fmt.Sscanf(entry.Name(), "%d", &pid); err != nil {
			continue
		}

		if processLooksLikeClipMode(pid) {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("no clip-mode process in /proc")
}

func processLooksLikeClipMode(pid int) bool {
	cmdline, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}

	cmdStr := string(cmdline)
	return strings.Contains(cmdStr, "wayland-recorder") &&
		strings.Contains(cmdStr, "record") &&
		strings.Contains(cmdStr, "clip-mode")
}

func RegisterShortcut(shortcut, description string) error {
	if shortcut == "" {
		return fmt.Errorf("empty shortcut")
	}

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return fmt.Errorf("failed to connect to session bus: %w", err)
	}
	defer conn.Close()

	parsedShortcut, err := ParseShortcut(shortcut)
	if err != nil {
		return fmt.Errorf("failed to parse shortcut: %w", err)
	}

	portal := conn.Object(PortalServiceName, PortalObjectPath)

	sessionPath, err := createShortcutSession(conn, portal)
	if err != nil {
		return err
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	if err := bindShortcut(conn, portal, sessionPath, parsedShortcut, description); err != nil {
		return err
	}

	return listenForActivation(conn, sessionPath, execPath)
}
