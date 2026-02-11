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
	PortalServiceName = "org.freedesktop.portal.Desktop"
	PortalObjectPath  = "/org/freedesktop/portal/desktop"
)

func generateToken() string {
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(randomBytes)
}

func waitForResponse(conn *dbus.Conn, path dbus.ObjectPath) (map[string]dbus.Variant, error) {
	if conn == nil {
		return nil, fmt.Errorf("nil connection")
	}
	if path == "" {
		return nil, fmt.Errorf("empty object path")
	}

	signalChannel := make(chan *dbus.Signal, 1)
	conn.Signal(signalChannel)

	listenRule := fmt.Sprintf("type='signal',path='%s'", path)
	conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, listenRule)
	defer conn.BusObject().Call("org.freedesktop.DBus.RemoveMatch", 0, listenRule)

	for signal := range signalChannel {
		if signal.Path == path {
			if len(signal.Body) < 2 {
				return nil, fmt.Errorf("invalid signal body")
			}
			responseCode := signal.Body[0].(uint32)
			if responseCode != 0 {
				return nil, fmt.Errorf("portal request failed with code %d", responseCode)
			}
			responseData := signal.Body[1].(map[string]dbus.Variant)
			return responseData, nil
		}
	}
	return nil, fmt.Errorf("no response received from portal")
}
