// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	GlobalShortcutsPortal = "org.freedesktop.portal.GlobalShortcuts"
	GlobalShortcutsPath   = "/org/freedesktop/portal/desktop"
)

type shortcutStruct struct {
	ID   string
	Data map[string]dbus.Variant
}

func ParseShortcut(shortcut string) (string, error) {
	normalized := normalizeShortcutString(shortcut)
	
	if normalized == "" {
		return "", fmt.Errorf("empty shortcut")
	}
	
	parts := strings.Split(normalized, "+")
	if len(parts) == 0 {
		return "", fmt.Errorf("invalid shortcut format")
	}
	
	modifiers, key, err := parseShortcutParts(parts)
	if err != nil {
		return "", err
	}
	
	return buildShortcutString(modifiers, key), nil
}

func normalizeShortcutString(shortcut string) string {
	normalized := strings.ReplaceAll(shortcut, " + ", "+")
	return strings.TrimSpace(normalized)
}

func parseShortcutParts(parts []string) ([]string, string, error) {
	var modifiers []string
	var key string
	
	for _, part := range parts {
		part = strings.TrimSpace(part)
		partLower := strings.ToLower(part)
		
		modifier, isModifier := parseModifier(partLower)
		if isModifier {
			modifiers = append(modifiers, modifier)
		} else {
			if key != "" {
				return nil, "", fmt.Errorf("multiple keys specified: %s and %s", key, part)
			}
			key = part
		}
	}
	
	if key == "" {
		return nil, "", fmt.Errorf("no key specified in shortcut")
	}
	
	if len(key) == 1 {
		key = strings.ToUpper(key)
	}
	
	return modifiers, key, nil
}

func parseModifier(mod string) (string, bool) {
	switch mod {
	case "ctrl", "control":
		return "Control", true
	case "alt":
		return "Alt", true
	case "shift":
		return "Shift", true
	case "super", "meta", "win":
		return "Super", true
	default:
		return "", false
	}
}

func buildShortcutString(modifiers []string, key string) string {
	result := ""
	for _, mod := range modifiers {
		result += "<" + mod + ">"
	}
	result += key
	return result
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
	shortcutData := map[string]dbus.Variant{
		"description":       dbus.MakeVariant(description),
		"preferred_trigger": dbus.MakeVariant(parsedShortcut),
	}

	shortcuts := []shortcutStruct{
		{
			ID:   "record-shortcut",
			Data: shortcutData,
		},
	}

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
	if shortcutsVariant, ok := bindResponse["shortcuts"]; ok {
		if bindShortcuts, ok := shortcutsVariant.Value().([]map[string]dbus.Variant); ok {
			if len(bindShortcuts) > 0 {
				fmt.Printf("✓ Shortcut registered: %s\n", parsedShortcut)
			}
		}
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
	
	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Printf("Failed to find process %d: %v\n", pid, err)
		return
	}
	
	if err := process.Signal(syscall.SIGUSR1); err != nil {
		fmt.Printf("Failed to send signal to process %d: %v\n", pid, err)
		return
	}
	
	fmt.Printf("Clip signal sent to process %d\n", pid)
}

func findRecordingProcess() (int, error) {
	pid, err := pidFromFile()
	if err == nil {
		return pid, nil
	}

	pid, err = pidFromProcScan()
	if err != nil {
		return 0, fmt.Errorf("no clip-mode recording found; start with --clip-mode: %w", err)
	}
	return pid, nil
}

func pidFromFile() (int, error) {
	pidFile := getPidFilePath()
	data, err := os.ReadFile(pidFile)
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
	file, err := os.Open("/proc")
	if err != nil {
		return 0, err
	}
	defer file.Close()

	entries, err := file.Readdirnames(-1)
	if err != nil {
		return 0, err
	}

	for _, entry := range entries {
		var pid int
		if _, err := fmt.Sscanf(entry, "%d", &pid); err != nil {
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