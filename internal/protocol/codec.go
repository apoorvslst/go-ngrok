package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

func WriteMessage(w io.Writer, msg any) error {
	if w == nil {
		return errors.New("cannot write to nil writer")
	}

	encoder := json.NewEncoder(w)

	err := encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode protocol message: %w", err)
	}

	return nil
}

func ReadMessage[T any](r io.Reader) (*T, error) {
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