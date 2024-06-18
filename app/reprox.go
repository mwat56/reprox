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
	if "" == aPort {
		aPort = ":80"
	}

	// ctxTimeout, cancelTimeout := context.WithTimeout(
	// 	context.Background(), time.Second << 3)
	// defer cancelTimeout()

	// We need a `server` reference to use it in `setup Signals()`
	// and to set some reasonable timeouts:
	server := &http.Server{
		// The TCP address for the server to listen on:
		Addr: aPort,

		// Return the base context for incoming requests on this server:
		// BaseContext: func(net.Listener) context.Context {
		// 	return ctxTimeout
		// },

		// Request handler to invoke:
		Handler: aHandler,

		// Set timeouts so that a slow or malicious client
		// doesn't hold resources forever
		//
		// The maximum amount of time to wait for the next request;
		// if IdleTimeout is zero, the value of ReadTimeout is used:
		IdleTimeout: 0,

		// The amount of time allowed to read request headers:
		ReadHeaderTimeout: time.Second << 1,

		// The maximum duration for reading the entire request,
		// including the body:
		ReadTimeout: time.Second << 2,

		// The maximum duration before timing out writes of the response:
		WriteTimeout: -1, // disable
	}

	apachelogger.SetErrLog(server)
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
// - `aHandler`: The handler to be invoked for each request received
// by the server.
// - `aCertificate`: The TLS certificate to be used for secure
// communication.
//
// Returns:
// - `*http.Server`: A pointer to the newly created and configured HTTPS server.
func createServer443(aHandler http.Handler, aCertificate tls.Certificate) *http.Server {
	result := createServ(aHandler, ":443")

	// see:
	// https://ssl-config.mozilla.org/#server=golang&version=1.14.1&config=old&guideline=5.4
	result.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{aCertificate},
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
		InsecureSkipVerify:       true, // avoid certificate validation
		MaxVersion:               tls.VersionTLS12,
		MinVersion:               tls.VersionTLS10,
		PreferServerCipherSuites: true,
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
			context.Background(), time.Second<<3,
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
	var (
		wg sync.WaitGroup
	)

	ph := reprox.NewProxyHandler()

	// setup the `ApacheLogger`:
	handler := apachelogger.Wrap(ph,
		reprox.AppSetup.AccessLog, reprox.AppSetup.ErrorLog)

	wg.Add(1)
	go func() { // HTTP server
		defer wg.Done()

		s := fmt.Sprintf("%s listening HTTP at :80", gMe)
		log.Println(s)
		apachelogger.Log("ReProx/main", s)

		server80 := createServer80(handler)
		if err := server80.ListenAndServe(); nil != err {
			exit(fmt.Sprintf("%s:80 %v", gMe, err))
		}
	}()

	wg.Add(1)
	go func() { // HTTPS server
		defer wg.Done()

		s := fmt.Sprintf("%s listening HTTPS at :443", gMe)
		log.Println(s)
		apachelogger.Log("ReProx/main", s)

		serverName := "private.proxy"
		certPath := certConfDir()
		certFile, keyFile := certFilenames(serverName, certPath)
		certificate, err := certGet(certFile, keyFile, serverName, certPath)
		if nil != err {
			exit(fmt.Sprintf("%s:443 %v", gMe, err))
		}

		server443 := createServer443(handler, certificate)
		if err := server443.ListenAndServeTLS(certFile, keyFile); nil != err {
			exit(fmt.Sprintf("%s:443 %v", gMe, err))
		}
	}()

	wg.Wait()
} // main()

/* _EoF_ */
