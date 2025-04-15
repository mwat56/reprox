/*
Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany

	All rights reserved
	EMail : <support@mwat.de>
*/
package main

//lint:file-ignore ST1005 - I prefer Capitalisation
//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/mwat56/reprox"
)

// `certFilenames()` generates the filenames for the certificate
// and key files.
// It takes two parameters: `aServername` and `aPath`.
// If `aServername` is empty, it defaults to current app's name.
// If `aPath` is empty, it defaults to the user's config directory.
//
// The function returns the filenames for the certificate and key files.
func certFilenames(aServername, aPath string) (string, string) {
	if "" == aServername {
		aServername = filepath.Base(os.Args[0])
	}
	if "" == aPath {
		aPath = reprox.ConfDir()
	} else {
		// make sure `aPath` is absolute
		aPath, _ = filepath.Abs(aPath)
	}

	return fmt.Sprintf("%s/%s.cert", aPath, aServername),
		fmt.Sprintf("%s/%s.key", aPath, aServername)
} // certFilenames()

// `generateTLS()` generates a self-signed certificate and key pair.
// It takes two parameters: `aServername` and `aPath`.
// If `aPath` is empty, it defaults to the default directory.
//
// The function returns an error if any occurs during the generation process.
func generateTLS(aServername, aPath string) error {
	var (
		certBytes    []byte
		certOut      *os.File
		err          error
		keyBytes     []byte
		keyOut       *os.File
		privateKey   *ecdsa.PrivateKey
		serialNumber *big.Int
	)
	// Generate a private key
	if privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); nil != err {
		return fmt.Errorf("Failed to generate private key: %v", err)
	}

	if serialNumber, err = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128)); nil != err {
		return fmt.Errorf("Failed to generate serial number: %v", err)
	}

	// Create a certificate template
	template := x509.Certificate{
		BasicConstraintsValid: true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		// KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		NotBefore:    time.Now(),
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"private server"},
			CommonName:   "localhost",
		},
	}

	// Generate a self-signed certificate
	if certBytes, err = x509.CreateCertificate(rand.Reader,
		&template, &template, &privateKey.PublicKey, privateKey); nil != err {
		return fmt.Errorf("Failed to create certificate: %v", err)
	}

	// build the filenames to use for certificate and private key
	certFilename, keyFilename := certFilenames(aServername, aPath)

	// create the certificate's file
	if certOut, err = os.OpenFile(certFilename,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660); nil != err {
		return fmt.Errorf("Failed to create certificate file: %v", err)
	}
	defer certOut.Close()

	// write the certificate's PEM encoding to `certOut`
	if err := pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	}); nil != err {
		return fmt.Errorf("Failed to write data to %s: %v", certFilename, err)
	}

	// create the key's file
	if keyOut, err = os.OpenFile(keyFilename,
		os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0660); nil != err {
		return fmt.Errorf("Failed to open key.pem for writing: %v", err)
	}
	defer keyOut.Close()

	// convert the private key to PKCS #8, ASN.1 DER form
	if keyBytes, err = x509.MarshalPKCS8PrivateKey(privateKey); nil != err {
		return fmt.Errorf("Unable to marshal private key: %v", err)
	}

	// write the key's PEM encoding to `keyOut`
	if err = pem.Encode(keyOut, &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}); nil != err {
		return fmt.Errorf("Failed to write data to key.pem: %v", err)
	}

	return nil
} // generateTLS()

// `certGet()` generates a TLS certificate from the provided certificate
// and key files.
//
// It takes four parameters: `aCertFile`, `aKeyFile`, `aServerName`, and
// `aPath`.
// `aCertFile` and `aKeyFile` are the paths to the certificate and key
// files, respectively.
// `aServerName` is the name of the server for which the certificate
// is generated.
// `aPath` is the default directory to store/load the certificate files.
//
// If an error occurs while loading the certificate and key files, the
// function will attempt to generate a new self-signed certificate and
// key pair using the `generateTLS` function.
//
// The function returns a `tls.Certificate` object representing the
// loaded or generated certificate and key pair, along with any
// encountered error.
func certGet(aCertFile, aKeyFile, aServerName, aPath string) (rCertificate tls.Certificate, rErr error) {
	var err error

	rCertificate, err = tls.LoadX509KeyPair(aCertFile, aKeyFile)
	if nil == err {
		return
	}

	if "" == aPath {
		aPath = reprox.ConfDir()
	}

	e2 := generateTLS(aServerName, aPath)
	if nil != e2 {
		rErr = fmt.Errorf("%s: %w", err.Error(), e2)
		return
	}

	// try again:
	rCertificate, rErr = tls.LoadX509KeyPair(aCertFile, aKeyFile)

	return
} // certGet()

/* _EoF_ */
