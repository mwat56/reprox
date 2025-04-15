/*
Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany

	All rights reserved
	EMail : <support@mwat.de>
*/
package main

//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
)

func Test_certFilenames(t *testing.T) {
	type tArgs struct {
		aServername string
		aPath       string
	}

	tests := []struct {
		name     string
		args     tArgs
		wantCert string
		wantKey  string
	}{
		{"EmptyArgs", tArgs{"", ""}, "/home/matthias/.config/app.test/app.test.cert", "/home/matthias/.config/app.test/app.test.key"},
		{"EmptyDir", tArgs{"tc2", ""}, "/home/matthias/.config/app.test/tc2.cert", "/home/matthias/.config/app.test/tc2.key"},
		{"EmptyServer", tArgs{"", "/tmp"}, "/tmp/app.test.cert", "/tmp/app.test.key"},
		{"NoEmpty", tArgs{"tc4", "/tmp"}, "/tmp/tc4.cert", "/tmp/tc4.key"},

		// TODO: Add test cases.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1 := certFilenames(tt.args.aServername, tt.args.aPath)
			if got != tt.wantCert {
				t.Errorf("certFilenames() Cert = %v, want %v", got, tt.wantCert)
			}
			if got1 != tt.wantKey {
				t.Errorf("certFilenames() Key = %v, want %v", got1, tt.wantKey)
			}
		})
	}
} // Test_certFilenames()

func Test_generateTLS(t *testing.T) {
	type tArgs struct {
		aServername string
		aPath       string
	}

	tc1 := tArgs{"", ""}
	tc2 := tArgs{"", "/tmp"}
	tc3 := tArgs{"tc31", "/tmp"}

	tests := []struct {
		name    string
		args    tArgs
		wantErr bool
	}{
		{"EmptyArgs", tc1, false},
		{"EmptyServer", tc2, false},
		{"BothArgs", tc3, false},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := generateTLS(tt.args.aServername, tt.args.aPath); (nil != err) != tt.wantErr {
				t.Errorf("generateTLS() error = %v, wantErr %v", err, tt.wantErr)
			} else {
				certFilename, keyFilename := certFilenames(tt.args.aServername, tt.args.aPath)

				certData, err := os.ReadFile(certFilename)
				if nil != err {
					t.Fatalf("Failed to read %s: %v", certFilename, err)
				}

				block, _ := pem.Decode(certData)
				if nil == block || block.Type != "CERTIFICATE" {
					t.Fatalf("Failed to decode PEM block containing certificate")
				}

				cert, err := x509.ParseCertificate(block.Bytes)
				if nil != err {
					t.Fatalf("Failed to parse certificate: %v", err)
				}

				if cert.Subject.CommonName != "localhost" {
					t.Errorf("Unexpected CommonName in certificate. Got %s, want 'localhost'", cert.Subject.CommonName)
				}

				keyData, err := os.ReadFile(keyFilename)
				if nil != err {
					t.Fatalf("Failed to read %s: %v", keyFilename, err)
				}

				block, _ = pem.Decode(keyData)
				if nil == block || block.Type != "EC PRIVATE KEY" {
					t.Fatalf("Failed to decode PEM block containing private key")
				}

				// t.Log("generateTLS() OK")
			}
		})
	}
} // Test_generateTLS()

/* _EoF_ */
