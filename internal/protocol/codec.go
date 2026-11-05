/*Streaming Codec (Encoder & Decoder) par. 
Yahan hum woh functions banayenge jo in structs ko TCP pipe
mein serialize (write) aur deserialize (read) karenge.*/

package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

/*Go mein do tareeqe hote hain JSON bhejne ke:

json.Marshal(msg) ->Yeh pehle poora JSON memory (RAM) mein ek bada []byte slice 
banata hai, fir hum use socket pe bhejte hain. Isse memory
allocate hoti hai (GC pressure).
json.NewEncoder(w).Encode(msg) →Zero intermediate memory allocation! Yeh directly socket (io.Writer)
 ke andar data stream karta hai, aur sabse badhiya baat: automatic end mein \n (newline) jod deta hai!*/

// WriteMessage encodes any protocol struct as a newline-delimited JSON frame into the writer.
func WriteMessage(w io.Writer, msg any) error {
	// 1. Nil safety check
	if w == nil {
		return errors.New("cannot write to nil writer")
	}

	// 2. Stream encoder banao jo directly writer mein likhega
	encoder := json.NewEncoder(w)  // stream w - encode that struct directly to wire (automatically appends '\n')
	err := encoder.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode protocol message: %w", err)
	}

	return nil
}

// ReadMessage decodes a single newline-delimited JSON frame from the reader into type T.(struct types)

//*CONCEPT = • Generics (such as <T> in C#, Java, or TypeScript) let you write classes, interfaces, and methods
//  that work with any data type while keeping compile-time type safety.

func ReadMessage[T any](r io.Reader) (*T, error) {
	if r == nil {
		return nil, errors.New("cannot read from nil reader")
	}

	// 2. Target variable of type T 
	var target T

	// 3. Decoder banao jo stream se JSON padhega
	decoder := json.NewDecoder(r) // decoer ke ander reader stream
	err := decoder.Decode(&target)  // we decode target 
	if err != nil {
		// Agar samne wale ne socket band kar diya toh direct io.EOF return karo
		if errors.Is(err, io.EOF) {
			return nil, io.EOF
		}
		return nil, fmt.Errorf("failed to decode protocol message: %w", err)
	}

	return &target, nil
}