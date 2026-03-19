package admin

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// SendCommand connects to the admin socket, sends a request, and prints
// streamed responses until the connection closes.
func SendCommand(req Request) error {
	conn, err := net.DialTimeout("unix", SocketPath(), 5*time.Second)
	if err != nil {
		return fmt.Errorf("cannot connect to admin socket at %s — is the server running?\n%w", SocketPath(), err)
	}
	defer conn.Close()

	// Long deadline — updateinference may wait for health.
	conn.SetDeadline(time.Now().Add(15 * time.Minute))

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("send command: %w", err)
	}

	dec := json.NewDecoder(conn)
	var lastErr error
	for dec.More() {
		var resp Response
		if err := dec.Decode(&resp); err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		fmt.Println(resp.Message)
		if !resp.OK {
			lastErr = fmt.Errorf("server returned error")
		}
	}

	return lastErr
}
