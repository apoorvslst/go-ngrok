package protocol // Declares this file belongs to the 'protocol' package

import (
	"encoding/json" // Go standard library for converting data to and from JSON format
	"errors"        // Go standard library for creating and comparing error values
	"fmt"           // Go standard library for formatted I/O and wrapping errors
	"io"            // Go standard library defining basic I/O interfaces like Reader and Writer
)

// WriteMessage encodes any Go data structure into a JSON frame and sends it over a writer.
// 'w io.Writer' is where the bytes will be written (e.g., a network connection net.Conn).
// 'msg any' means you can pass any type of message struct (HelloMessage, AuthResponseMessage, etc.).
func WriteMessage(w io.Writer, msg any) error {
	// Guard Check: Verify that the writer is not nil to prevent a nil-pointer panic.
	if w == nil {
		return errors.New("cannot write to nil writer")
	}

	// json.NewEncoder creates a streaming encoder attached directly to writer 'w'.
	// Unlike json.Marshal, this writes directly to the stream without allocating intermediate buffers.
	encoder := json.NewEncoder(w)

	// encoder.Encode serializes 'msg' into JSON and automatically adds a trailing '\n' (newline).
	// This newline '\n' serves as our delimiter so the receiver knows where the message ends.
	err := encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode protocol message: %w", err)
	}

	return nil
}

// ReadMessage reads a single newline-delimited JSON frame from reader 'r' and decodes it into type T.
// '[T any]' is a Go Generic type parameter, allowing this function to decode into ANY struct type.
// 'r io.Reader' is the source of incoming bytes (e.g., our TCP network socket).
// '(*T, error)' returns a pointer to the decoded struct of type T, or an error if reading failed.
func ReadMessage[T any](r io.Reader) (*T, error) {
	// Guard Check: Make sure the incoming reader is not nil.
	if r == nil {
		return nil, errors.New("cannot read from nil reader")
	}

	var target T

	// Read bytes until the newline delimiter '\n' without over-buffering future frames in 'r'
	var buf []byte
	singleByte := make([]byte, 1)
	for {
		n, err := r.Read(singleByte)
		if n > 0 {
			buf = append(buf, singleByte[0])
			if singleByte[0] == '\n' {
				break
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(buf) > 0 {
					break
				}
				return nil, io.EOF
			}
			return nil, fmt.Errorf("failed to read protocol frame: %w", err)
		}
	}

	if len(buf) == 0 {
		return nil, io.EOF
	}

	// Unmarshal the single framed JSON payload into target
	if err := json.Unmarshal(buf, &target); err != nil {
		return nil, fmt.Errorf("failed to decode protocol message: %w", err)
	}

	return &target, nil
}
