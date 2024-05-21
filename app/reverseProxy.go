/*
Copyright © 2024  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package main

//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/mwat56/apachelogger"
	"github.com/mwat56/reprox"
)

var (
	// Name of the running program:
	gMe = func() string {
		return filepath.Base(os.Args[0])
	}()
)

// `createServ()` creates and returns a new HTTP server listening
// on the provided port.
// The server is configured with the provided handler and with reasonable
// timeouts.
// The server is also set up to handle graceful shutdowns when receiving
// SIGINT or SIGTERM signals.
//
// Parameters:
// - `aHandler` (http.Handler): The handler to be invoked for each
// request received by the server.
// - `aPort` (string): The TCP address for the server to listen on.
//
// Returns:
// - `*http.Server`: A pointer to the newly created and configured HTTP server.
func createServ(aHandler http.Handler, aPort string) *http.Server {
	// var once sync.Once
	ctxTimeout, cancelTimeout := context.WithTimeout(
		context.Background(), time.Second*10)
	defer cancelTimeout()

	if 0 == len(aPort) {
		aPort = ":80"
	}

	// We need a `server` reference to use it in `setup Signals()`
	// and to set some reasonable timeouts:
	server := &http.Server{
		// The TCP address for the server to listen on:
		Addr: aPort,

		// Return the base context for incoming requests on this server:
		BaseContext: func(net.Listener) context.Context {
			return ctxTimeout
		},

		// Request handler to invoke:
		Handler: aHandler,

		// Set timeouts so that a slow or malicious client
		// doesn't hold resources forever
		//
		// The maximum amount of time to wait for the next request;
		// if IdleTimeout is zero, the value of ReadTimeout is used:
		IdleTimeout: 0,

		// The amount of time allowed to read request headers:
		ReadHeaderTimeout: 10 * time.Second,

		// The maximum duration for reading the entire request,
		// including the body:
		ReadTimeout: 10 * time.Second,

		// The maximum duration before timing out writes of the response:
		// WriteTimeout: 10 * time.Second,
		WriteTimeout: -1, // see whether this eliminates "i/o timeout HTTP/1.0"
	}
	setupSignals(server)

	return server
} // createServ()

// `createServer443()` creates and returns a new HTTPS server listening
// on port 443.
// The server is configured with the provided handler and with reasonable
// timeouts.
// The server is also set up to handle graceful shutdowns when receiving
// SIGINT or SIGTERM signals.
// Additionally, the server is configured with TLS settings to enhance
// security, following Mozilla's SSL Configuration Generator recommendations.
//
// Parameters:
// - `aHandler` (http.Handler): The handler to be invoked for each
// request received by the server.
// - `aCertFile` (string): The path to the certificate file for TLS.
// - `aKeyFile` (string): The path to the private key file for TLS.
//
// Returns:
// - `*http.Server`: A pointer to the newly created and configured HTTPS server.
func createServer443(aHandler http.Handler, aCertFile, aKeyFile string) *http.Server {
	result := createServ(aHandler, ":443")

	// see:
	// https://ssl-config.mozilla.org/#server=golang&version=1.14.1&config=old&guideline=5.4
	result.TLSConfig = &tls.Config{
		MaxVersion:               tls.VersionTLS12,
		MinVersion:               tls.VersionTLS10,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_RC4_128_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_RC4_128_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
			tls.TLS_RSA_WITH_RC4_128_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			cert, err := tls.LoadX509KeyPair(aCertFile, aKeyFile)
			if nil != err {
				return nil, err
			}
			return &cert, nil
		},
	} // #nosec G402
	// server.TLSNextProto = make(map[string]func(*http.Server, *tls.Conn, http.Handler))

	return result
} // createServer443()

// `createServer80()` creates and returns a new HTTP server listening
// on port 80.
// The server is configured with the provided handler and with reasonable
// timeouts.
// The server is also set up to handle graceful shutdowns when receiving
// SIGINT or SIGTERM signals.
//
// Parameters:
// - `aHandler` (http.Handler): The handler to be invoked for each
// request received by the server.
//
// Returns:
// - `*http.Server`: A pointer to the newly created and configured HTTP server.
func createServer80(aHandler http.Handler) *http.Server {
	return createServ(aHandler, ":80")
} // createServer80()

// `exit()` logs `aMessage` and terminate the program.
//
// Parameters:
//
//	`aMessage` (string): The message to be logged and displayed.
func exit(aMessage string) {
	apachelogger.Err("ReProx/main", aMessage)
	runtime.Gosched() // let the logger write
	log.Fatalln(aMessage)
} // exit()

// `setupSignals()` configures the capture of the interrupts `SIGINT`
// It also sets up a context for the server and registers a shutdown
// function to be called when the context is canceled.
//
// Parameters:
//
//	`aServer` *http.Server - The HTTP server to be gracefully shut down.
func setupSignals(aServer *http.Server) {
	// handle `CTRL-C` and `kill(15)`:
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for signal := range c {
			msg := fmt.Sprintf("%s captured '%v', stopping program and exiting ...", gMe, signal)
			apachelogger.Err(`ReProx/catchSignals`, msg)
			log.Println(msg)
			break
		}

		ctx, cancel := context.WithCancel(context.Background())
		aServer.BaseContext = func(net.Listener) context.Context {
			return ctx
		}
		aServer.RegisterOnShutdown(cancel)

		ctxTimeout, cancelTimeout := context.WithTimeout(
			context.Background(),
			time.Second*10,
		)
		defer cancelTimeout()
		if err := aServer.Shutdown(ctxTimeout); err != nil {
			exit(fmt.Sprintf("%s: %v", gMe, err))
		}
	}()
} // setupSignals()

/*
- @title Main function for the reverse proxy server.
*/
func main() {
	var wg sync.WaitGroup

	//TODO: implement INI parsing
	ph := reprox.NewProxyHandler( /*aConfigFile string*/ )

	// setup the `ApacheLogger`:
	handler := apachelogger.Wrap(ph,
		fmt.Sprintf("%s.%s.log", "access", gMe),
		fmt.Sprintf("%s.%s.log", "error", gMe))

	wg.Add(1)
	go func() {
		defer wg.Done()

		s := fmt.Sprintf("%s listening HTTP at :80", gMe)
		log.Println(s)
		apachelogger.Log("ReProx/main", s)
		server80 := createServer80(handler)
		exit(fmt.Sprintf("%s: %v", gMe, server80.ListenAndServe()))
	}()

	/*
		wg.Add(1)
		go func() {
			defer wg.Done()

			//TODO: implement TLS-files
			var certFile, keyFile string

			s := fmt.Sprintf("%s listening HTTP at :443", gMe)
			log.Println(s)
			apachelogger.Log("ReProx/main", s)
			server443 := createServer443(handler certFile, keyFile)
			exit(fmt.Sprintf("%s: %v", gMe,
				server443.ListenAndServeTLS(certFile, keyFile)))
		}()
	*/
	wg.Wait()
} // main()

/* _EoF_ */
