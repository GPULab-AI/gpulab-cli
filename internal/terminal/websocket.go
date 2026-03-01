package terminal

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/gorilla/websocket"
	"golang.org/x/term"
)

// HTTPToWS converts an HTTP(S) URL to a WebSocket URL
func HTTPToWS(url string) string {
	url = strings.Replace(url, "https://", "wss://", 1)
	url = strings.Replace(url, "http://", "ws://", 1)
	return url
}

// Connect opens an interactive terminal via WebSocket to a ttyd server.
// ttyd binary protocol:
//   - Client->Server: 0 + data = stdin input
//   - Client->Server: 1 + JSON = resize {"columns":N,"rows":N}
//   - Server->Client: 0 + data = stdout output
//   - Server->Client: 1 + JSON = window title
func Connect(wsURL string) error {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to terminal: %w", err)
	}
	defer conn.Close()

	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set raw terminal mode: %w", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	// Send initial window size
	sendResize(conn)

	// Handle window resize signals
	sigWinch := make(chan os.Signal, 1)
	signal.Notify(sigWinch, syscall.SIGWINCH)
	go func() {
		for range sigWinch {
			sendResize(conn)
		}
	}()

	done := make(chan struct{})
	errChan := make(chan error, 2)

	// Read from WebSocket, write to stdout
	go func() {
		defer close(done)
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				errChan <- err
				return
			}
			if len(message) < 1 {
				continue
			}
			// prefix 0 = stdout data
			if message[0] == '0' {
				os.Stdout.Write(message[1:])
			}
			// prefix 1 = window title (ignore)
		}
	}()

	// Read from stdin, write to WebSocket
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				errChan <- err
				return
			}
			// prefix 0 + data for stdin
			msg := append([]byte{'0'}, buf[:n]...)
			if err := conn.WriteMessage(websocket.BinaryMessage, msg); err != nil {
				errChan <- err
				return
			}
		}
	}()

	select {
	case <-done:
	case err := <-errChan:
		// Check if it's a clean close
		if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
			return nil
		}
		// Ignore read errors on stdin (happens on Ctrl+D)
		return nil
	}

	return nil
}

func sendResize(conn *websocket.Conn) {
	width, height, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		return
	}
	msg := fmt.Sprintf(`1{"columns":%d,"rows":%d}`, width, height)
	conn.WriteMessage(websocket.TextMessage, []byte(msg))
}
