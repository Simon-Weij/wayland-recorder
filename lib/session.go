// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/godbus/dbus/v5"
)

const (
	PortalServiceName    = "org.freedesktop.portal.Desktop"
	PortalObjectPath     = "/org/freedesktop/portal/desktop"
	CreateSessionMethod  = "org.freedesktop.portal.ScreenCast.CreateSession"
	SelectSourcesMethod  = "org.freedesktop.portal.ScreenCast.SelectSources"
	StartRecordingMethod = "org.freedesktop.portal.ScreenCast.Start"
)

type Stream struct {
	NodeID uint32
}

func generateToken() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(randomBytes)
}

func waitForResponse(conn *dbus.Conn, path dbus.ObjectPath) (map[string]dbus.Variant, error) {
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
		"session_handle_token": dbus.MakeVariant(generateToken()),
		"handle_token":         dbus.MakeVariant(generateToken()),
	}

	requestPath := dbus.ObjectPath("")
	err = desktopPortal.Call(CreateSessionMethod, 0, options).Store(&requestPath)
	if err != nil {
		connection.Close()
		return nil, "", err
	}

	response, err := waitForResponse(connection, requestPath)
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
		"handle_token": dbus.MakeVariant(generateToken()),
		"types":        dbus.MakeVariant(sourceType),
		"cursor_mode":  dbus.MakeVariant(cursorMode),
	}

	var requestPath dbus.ObjectPath
	err := desktopPortal.Call(SelectSourcesMethod, 0, session, options).Store(&requestPath)
	if err != nil {
		return err
	}

	_, err = waitForResponse(conn, requestPath)
	return err
}

func StartRecording(conn *dbus.Conn, session dbus.ObjectPath) ([]Stream, error) {
	desktopPortal := conn.Object(PortalServiceName, PortalObjectPath)

	options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(generateToken())}

	var requestPath dbus.ObjectPath
	err := desktopPortal.Call(StartRecordingMethod, 0, session, "", options).Store(&requestPath)
	if err != nil {
		return nil, err
	}

	response, err := waitForResponse(conn, requestPath)
	if err != nil {
		return nil, err
	}

	streams := parseStreams(response)
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams found")
	}
	return streams, nil
}

func parseStreams(response map[string]dbus.Variant) []Stream {
	var streams []Stream
	streamsData := response["streams"].Value()

	if streamArray, isCorrectType := streamsData.([][]interface{}); isCorrectType {
		for _, streamInfo := range streamArray {
			if nodeID, isUint32 := streamInfo[0].(uint32); isUint32 {
				streams = append(streams, Stream{NodeID: nodeID})
			}
		}
	}
	return streams
}
