/*
Copyright © 2024  M.Watermann, 10247 Berlin, Germany

	All rights reserved
	EMail : <support@mwat.de>
*/
package main

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
)

// `certConfDir()` returns the directory path where the configuration
// files for the application are stored.
// It uses `os.UserConfigDir()` to get the user's configuration directory.
//
// The function then joins this directory with the base name of the
// running executable using `filepath.Join()`.
// This ensures that the configuration files are stored in a
// predictable location for the user.
//
// NOTE: This implementation uses only the "happy path" i.e. all
// possible errors are ignored!
func certConfDir() string {
	confDir, _ := os.UserConfigDir()

	return filepath.Join(confDir, filepath.Base(os.Args[0]))
} // certConfDir()

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
		aPath = certConfDir()
	}

	return fmt.Sprintf("%s/%s.cert", aPath, aServername),
		fmt.Sprintf("%s/%s.key", aPath, aServername)
} // filenames()

// `generateTLS()` generates a self-signed certificate and key pair.
// It takes two parameters: `aServername` and `aPath`.
// If `aPath` is empty, it defaults to the default directory.
//
// The function returns an error if any occurs during the generation process.
func generateTLS(aServername, aPath string) error {
	var (
		certBytes  []byte
		certOut    *os.File
		err        error
		keyBytes   []byte
		keyOut     *os.File
		privateKey *ecdsa.PrivateKey
	)
	// Generate a private key
	privateKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if nil != err {
		return err
	}

	// Create a certificate template
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"private server"},
			CommonName:   aServername,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Generate a self-signed certificate
	certBytes, err = x509.CreateCertificate(rand.Reader,
		&template, &template, &privateKey.PublicKey, privateKey)
	if nil != err {
		return err
	}

	// build the filenames to use fr certificate and private key
	certFilename, keyFilename := certFilenames(aServername, aPath)

	// create the certificate's file
	certOut, err = os.Create(certFilename)
	if nil != err {
		return err
	}
	defer certOut.Close()

	// write the certificate's PEM encoding to `certOut`
	pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	// create the key's file
	keyOut, err = os.Create(keyFilename)
	if nil != err {
		return err
	}
	defer keyOut.Close()

	// convert the private key to PKCS #8, ASN.1 DER form
	keyBytes, err = x509.MarshalPKCS8PrivateKey(privateKey)
	if nil != err {
		return err
	}

	// write the key's PEM encoding to `keyOut`
	err = pem.Encode(keyOut, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: keyBytes,
	})
	if nil != err {
		return err
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

	if "" == aPath {
		aPath = certConfDir()
	}

	rCertificate, err = tls.LoadX509KeyPair(aCertFile, aKeyFile)
	if nil != err {
		e2 := generateTLS(aServerName, aPath)
		if nil != e2 {
			rErr = fmt.Errorf("%s: %w", err.Error(), e2)
			return
		}
	} else {
		return
	}

	rCertificate, rErr = tls.LoadX509KeyPair(aCertFile, aKeyFile)
	return
} // certGet()

/* _EoF_ */
