// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/godbus/dbus/v5"
)

const (
	DefaultFilePermissions = 0755
	GStreamerCommand       = "gst-launch-1.0"
	PortalServiceName      = "org.freedesktop.portal.Desktop"
	PortalObjectPath       = "/org/freedesktop/portal/desktop"
	CreateSessionMethod    = "org.freedesktop.portal.ScreenCast.CreateSession"
	SelectSourcesMethod    = "org.freedesktop.portal.ScreenCast.SelectSources"
	StartRecordingMethod   = "org.freedesktop.portal.ScreenCast.Start"
)

type Stream struct {
	NodeID uint32
}

type CaptureOptions struct {
	OutputPath   string
	Codec        string
	Container    string
	EncoderSpeed int
	Quality      int
}

func token() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return hex.EncodeToString(randomBytes)
}

func waitResponse(conn *dbus.Conn, path dbus.ObjectPath) (map[string]dbus.Variant, error) {
	signalChannel := make(chan *dbus.Signal, 1)
	conn.Signal(signalChannel)

	listenRule := fmt.Sprintf("type='signal',path='%s'", path)
	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, listenRule)
	defer conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, listenRule)

	for signal := range signalChannel {
		if signal.Path == path {
			responseCode := signal.Body[0].(uint32)
			if responseCode != 0 {
				return nil, fmt.Errorf("failed: %d", responseCode)
			}
			responseData := signal.Body[1].(map[string]dbus.Variant)
			return responseData, nil
		}
	}
	return nil, fmt.Errorf("no response")
}

func CreateSession() (*dbus.Conn, dbus.ObjectPath, error) {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, "", err
	}

	desktopPortal := connection.Object(PortalServiceName, PortalObjectPath)

	options := map[string]dbus.Variant{
		"session_handle_token": dbus.MakeVariant(token()),
		"handle_token":         dbus.MakeVariant(token()),
	}

	requestPath := dbus.ObjectPath("")
	err = desktopPortal.Call(CreateSessionMethod, 0, options).Store(&requestPath)
	if err != nil {
		connection.Close()
		return nil, "", err
	}

	response, err := waitResponse(connection, requestPath)
	if err != nil {
		connection.Close()
		return nil, "", err
	}

	sessionHandle := response["session_handle"].Value().(string)
	sessionPath := dbus.ObjectPath(sessionHandle)
	return connection, sessionPath, nil
}

func SelectSources(conn *dbus.Conn, session dbus.ObjectPath, sourceType uint32, cursorMode uint32) error {
	desktopPortal := conn.Object(PortalServiceName, PortalObjectPath)

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(token()),
		"types":        dbus.MakeVariant(sourceType),
		"cursor_mode":  dbus.MakeVariant(cursorMode),
	}

	var requestPath dbus.ObjectPath
	err := desktopPortal.Call(SelectSourcesMethod, 0, session, options).Store(&requestPath)
	if err != nil {
		return err
	}

	_, err = waitResponse(conn, requestPath)
	return err
}

func StartRecording(conn *dbus.Conn, session dbus.ObjectPath) ([]Stream, error) {
	desktopPortal := conn.Object(PortalServiceName, PortalObjectPath)

	options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(token())}

	var requestPath dbus.ObjectPath
	err := desktopPortal.Call(StartRecordingMethod, 0, session, "", options).Store(&requestPath)
	if err != nil {
		return nil, err
	}

	response, err := waitResponse(conn, requestPath)
	if err != nil {
		return nil, err
	}

	var streams []Stream
	streamsData := response["streams"].Value()

	if streamArray, isCorrectType := streamsData.([][]interface{}); isCorrectType {
		for _, streamInfo := range streamArray {
			if nodeID, isUint32 := streamInfo[0].(uint32); isUint32 {
				streams = append(streams, Stream{NodeID: nodeID})
			}
		}
	}

	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found")
	}
	return streams, nil
}

func ensureOutputDirectory(outputPath string) error {
	directoryPath := filepath.Dir(outputPath)
	return os.MkdirAll(directoryPath, DefaultFilePermissions)
}

func buildEncoderArgs(codec string, encoderSpeed int, quality int) ([]string, error) {
	var args []string

	switch codec {
	case "vp8":
		args = []string{"!", "vp8enc", fmt.Sprintf("deadline=%d", encoderSpeed)}
		if quality > 0 {
			args = append(args, fmt.Sprintf("target-bitrate=%d", quality))
		}
	case "vp9":
		args = []string{"!", "vp9enc", fmt.Sprintf("deadline=%d", encoderSpeed)}
		if quality > 0 {
			args = append(args, fmt.Sprintf("target-bitrate=%d", quality))
		}
	case "h264", "x264":
		args = []string{"!", "x264enc", fmt.Sprintf("speed-preset=%d", encoderSpeed)}
		if quality > 0 {
			bitrateKbps := quality / 1000
			args = append(args, fmt.Sprintf("bitrate=%d", bitrateKbps))
		}
	default:
		return nil, fmt.Errorf("unsupported codec: %s (use: vp8, vp9, h264, or x264)", codec)
	}

	return args, nil
}

func buildMuxerArgs(container string) ([]string, error) {
	switch container {
	case "webm":
		return []string{"!", "webmmux", "streamable=true"}, nil
	case "mp4":
		return []string{"!", "mp4mux", "fragment-duration=1000", "streamable=true"}, nil
	case "mkv":
		return []string{"!", "matroskamux", "streamable=true"}, nil
	default:
		return nil, fmt.Errorf("unsupported container: %s (use: webm, mp4, or mkv)", container)
	}
}

func buildSinkArgs(outputPath string) []string {
	return []string{"!", "filesink", fmt.Sprintf("location=%s", outputPath)}
}

func buildGStreamerArgs(nodeID uint32, opts CaptureOptions) ([]string, error) {
	args := []string{
		"-e",
		"pipewiresrc", fmt.Sprintf("path=%d", nodeID),
		"!", "videoconvert",
	}

	encoderArgs, err := buildEncoderArgs(opts.Codec, opts.EncoderSpeed, opts.Quality)
	if err != nil {
		return nil, err
	}
	args = append(args, encoderArgs...)

	muxerArgs, err := buildMuxerArgs(opts.Container)
	if err != nil {
		return nil, err
	}
	args = append(args, muxerArgs...)

	sinkArgs := buildSinkArgs(opts.OutputPath)
	args = append(args, sinkArgs...)

	return args, nil
}

func executeRecording(args []string, outputPath string) error {
	command := exec.Command(GStreamerCommand, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr

	err := command.Start()
	if err != nil {
		return err
	}

	fmt.Printf("Recording to: %s\nPress Ctrl+C to stop\n", outputPath)

	interruptSignal := make(chan os.Signal, 1)
	signal.Notify(interruptSignal, os.Interrupt, syscall.SIGTERM)

	finished := make(chan error, 1)
	go func() {
		finished <- command.Wait()
	}()

	select {
	case <-interruptSignal:
		fmt.Println("\nStopping recording and finalizing file...")
		command.Process.Signal(syscall.SIGINT)
		<-finished
		fmt.Println("Stopped")
	case err := <-finished:
		if err != nil {
			return err
		}
		fmt.Println("Done")
	}
	return nil
}

func Capture(nodeID uint32, opts CaptureOptions) error {
	err := ensureOutputDirectory(opts.OutputPath)
	if err != nil {
		return err
	}

	args, err := buildGStreamerArgs(nodeID, opts)
	if err != nil {
		return err
	}

	return executeRecording(args, opts.OutputPath)
}
