// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package lib

import (
	"fmt"

	"github.com/godbus/dbus/v5"
)

const (
	CreateSessionMethod  = "org.freedesktop.portal.ScreenCast.CreateSession"
	SelectSourcesMethod  = "org.freedesktop.portal.ScreenCast.SelectSources"
	StartRecordingMethod = "org.freedesktop.portal.ScreenCast.Start"
)

type Stream struct {
	NodeID uint32
}

func CreateSession() (*dbus.Conn, dbus.ObjectPath, error) {
	connection, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect to session bus: %w", err)
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
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	response, err := waitForResponse(connection, requestPath)
	if err != nil {
		connection.Close()
		return nil, "", fmt.Errorf("failed to get session response: %w", err)
	}

	sessionHandle := response["session_handle"].Value().(string)
	sessionPath := dbus.ObjectPath(sessionHandle)
	return connection, sessionPath, nil
}

func SelectSources(conn *dbus.Conn, session dbus.ObjectPath, sourceType uint32, cursorMode uint32) error {
	if conn == nil {
		return fmt.Errorf("nil connection")
	}

	desktopPortal := conn.Object(PortalServiceName, PortalObjectPath)

	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(generateToken()),
		"types":        dbus.MakeVariant(sourceType),
		"cursor_mode":  dbus.MakeVariant(cursorMode),
	}

	var requestPath dbus.ObjectPath
	err := desktopPortal.Call(SelectSourcesMethod, 0, session, options).Store(&requestPath)
	if err != nil {
		return fmt.Errorf("failed to select sources: %w", err)
	}

	_, err = waitForResponse(conn, requestPath)
	return err
}

func StartRecording(conn *dbus.Conn, session dbus.ObjectPath) ([]Stream, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil connection")
	}

	desktopPortal := conn.Object(PortalServiceName, PortalObjectPath)

	options := map[string]dbus.Variant{"handle_token": dbus.MakeVariant(generateToken())}

	var requestPath dbus.ObjectPath
	err := desktopPortal.Call(StartRecordingMethod, 0, session, "", options).Store(&requestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to start recording: %w", err)
	}

	response, err := waitForResponse(conn, requestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get start recording response: %w", err)
	}

	streams := parseStreams(response)
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams available")
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
