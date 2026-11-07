/*
Go community mein unit tests likhne ka gold standard pattern hota hai 
Table-Driven Tests. Dusri languages mein hum har case ke liye alag function likhte hain:
test_simple_subdomain()
test_with_port()
test_error_ip()
Lekin Go mein hum ek slice of structs banate hain (ek "table") jisme test case ka name, inputs (host, rootDomain), aur expected output (want, wantErr) likh dete hain. Phir ek single for loop aur t.Run() se
 saare cases execute hote hain! Kal ko agar koi naya corner case mile, toh bas table mein 1 line add karni padti hai.
*/

package router_test

import (
	"testing"
	"github.com/apoorvslst/go-ngrok/internal/server/router"
)

func TestExtractSubdomain(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		rootDomain string
		want       string
		wantErr    bool
	}{
		// 1. Happy path cases
		{name: "simple subdomain", host: "gaurav.tunnel.dev", rootDomain: "tunnel.dev", want: "gaurav", wantErr: false},
		{name: "with port", host: "gaurav.tunnel.dev:8080", rootDomain: "tunnel.dev", want: "gaurav", wantErr: false},
		{name: "uppercase mixed", host: "GAURAV.Tunnel.Dev", rootDomain: "tunnel.dev", want: "gaurav", wantErr: false},
		// 2. Error cases (invalid hosts)
		{name: "root domain only", host: "tunnel.dev", rootDomain: "tunnel.dev", want: "", wantErr: true},
		{name: "wrong domain", host: "gaurav.google.com", rootDomain: "tunnel.dev", want: "", wantErr: true},
		{name: "empty host", host: "", rootDomain: "tunnel.dev", want: "", wantErr: true},
		{name: "IP address", host: "192.168.1.1", rootDomain: "tunnel.dev", want: "", wantErr: true},

		// 3. Edge cases
		{name: "hyphenated subdomain", host: "my-app.tunnel.dev", rootDomain: "tunnel.dev", want: "my-app", wantErr: false},
		{name: "numeric subdomain", host: "123.tunnel.dev", rootDomain: "tunnel.dev", want: "123", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := router.ExtractSubdomain(tt.host, tt.rootDomain)

			if tt.wantErr && err == nil {
				t.Errorf("expected error but got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}