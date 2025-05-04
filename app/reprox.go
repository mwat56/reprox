/*
Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany

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

	al "github.com/mwat56/apachelogger"
	"github.com/mwat56/ratelimit"
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
//
// The returned server is configured with the provided handler and with
// reasonable timeouts. It is also set up to handle graceful shutdowns
// when receiving `SIGINT` or `SIGTERM` signals.
//
// Parameters:
//   - `aHandler`: The handler to be invoked for each request received by the server.
//   - `aPort`: The TCP address for the server to listen on.
//
// Returns:
//   - `*http.Server`: A pointer to the newly created and configured HTTP server.
func createServ(aHandler http.Handler, aPort string) *http.Server {
	if "" == aPort {
		aPort = ":80"
	}

	// We need a `server` reference to use it in `setupSignals()`
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

	al.SetErrorLog(server)
	setupSignals(server)

	return server
} // createServ()

// `createServer443()` creates and returns a new HTTPS server listening
// on port 443.
//
// The returned server is configured with the provided handler and with
// reasonable timeouts. It is also set up to handle graceful shutdowns
// when receiving `SIGINT` or `SIGTERM` signals.
//
// Additionally, the server is configured with TLS settings to enhance
// security, following Mozilla's SSL Configuration Generator recommendations.
//
// Parameters:
//   - `aHandler`: The handler to be invoked for each request received by the server.
//   - `aCertificate`: The TLS certificate to be used for secure communication.
//
// Returns:
//   - `*http.Server`: A pointer to the newly created and configured HTTPS server.
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
//
// The returned server is configured with the provided handler and with
// reasonable timeouts. It is also set up to handle graceful shutdowns
// when receiving `SIGINT` or `SIGTERM` signals.
//
// Parameters:
//   - `aHandler`: The handler to be invoked for each request received by the server.
//
// Returns:
//   - `*http.Server`: A pointer to the newly created and configured HTTP server.
func createServer80(aHandler http.Handler) *http.Server {
	return createServ(aHandler, ":80")
} // createServer80()

// `exit()` logs `aMessage` and terminate the program.
//
// Parameters:
//   - `aMessage`: The message to be logged and displayed.
func exit(aMessage string) {
	al.Err("ReProx/exit", aMessage)
	runtime.Gosched() // let the logger write
	log.Fatalln(aMessage)
} // exit()

// `setupSignals()` configures the capture of the interrupts `SIGINT`
// and `SIGTERM`.
//
// The function also sets up a context for the server and registers
// a shutdown function to be called when the context is canceled.
//
// Parameters:
//   - `aServer`: The HTTP server to be gracefully shut down.
func setupSignals(aServer *http.Server) {
	// handle `CTRL-C` and `kill(15)` signals
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for signal := range sigChan {
			msg := fmt.Sprintf("%s captured '%v', stopping server at %q and exiting ...", gMe, signal, aServer.Addr)
			al.Err(`ReProx/catchSignals`, msg)
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
		if err := aServer.Shutdown(ctxTimeout); nil != err {
			exit(fmt.Sprintf("%s: '%v'", gMe, err))
		}
	}()
} // setupSignals()

/*
- @title Main function for the reverse proxy server.
*/
func main() {
	// First check whether we're actually running as root:
	if 0 != os.Getuid() {
		exit("\n\troot privileges required to bind to ports 80 and 443; terminating ...\n")
	}
	const alTxt = "ReProx/main"

	// Load the configuration
	configFile := filepath.Join(reprox.ConfDir(), gMe+".json")
	proxyConfig, err := reprox.LoadConfig(configFile)
	if nil != err {
		log.Fatalf("%s: Configuration load error: '%v'", alTxt, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the config watcher
	go reprox.WatchConfigFile(ctx, proxyConfig, configFile, time.Minute)
	// Wait for watcher to start
	runtime.Gosched()

	// Create a new proxy handler with the configuration
	ph := reprox.New(proxyConfig)

	// setup the `ApacheLogger`:
	handler := al.Wrap(ph, proxyConfig.AccessLog, proxyConfig.ErrorLog)

	// include rate limiting, if configured
	var getMetrics ratelimit.TMetricsFunc
	maxReq := uint(0)
	if 0 < proxyConfig.MaxRequests { // rate limiting enabled
		maxReq = uint(proxyConfig.MaxRequests) //#nosec G115
	}
	handler, getMetrics = ratelimit.Wrap(handler, maxReq, proxyConfig.WindowSize)

	var (
		listenErr error
		wg        sync.WaitGroup
	)
	if (0 < proxyConfig.MaxRequests) && (nil != getMetrics) {
		al.Log(alTxt, "Rate limiting enabled")

		wg.Add(1)
		go func(aCtx context.Context) { // periodically log metrics
			defer wg.Done()

			var (
				metrics   ratelimit.TMetrics
				prevTotal uint64
				msg       string
			)
			ticker := time.NewTicker(proxyConfig.WindowSize * 2)
			defer ticker.Stop()

			for {
				select {
				case <-aCtx.Done():
					al.Err(alTxt, aCtx.Err().Error())
					return

				case <-ticker.C:
					metrics = getMetrics()
					if prevTotal != metrics.TotalRequests {
						prevTotal = metrics.TotalRequests
						msg = fmt.Sprintf("Rate Limiter Metrics:\n"+
							"Total Requests: %d\n"+
							"Blocked Requests: %d\n"+
							"Active Clients: %d\n"+
							"Cleanup Interval: %v\n",
							metrics.TotalRequests,
							metrics.BlockedRequests,
							metrics.ActiveClients,
							metrics.CleanupDuration)
						al.Log(alTxt, msg)
					}
				}
			}
		}(ctx)
	}

	if ("" != proxyConfig.TLSCertFile) && ("" != proxyConfig.TLSKeyFile) {
		wg.Add(1)
		go func() { // HTTPS server at port 443
			defer wg.Done()
			// if "" == proxyConfig.TLSCertFile {
			// 	if err := generateTLS("", ""); nil != err {
			// 		cancel()
			// 		exit(fmt.Sprintf("%s:443 %v", gMe, err))
			// 	}
			// }

			certificate, err := tls.LoadX509KeyPair(proxyConfig.TLSCertFile, proxyConfig.TLSKeyFile)
			if nil != err {
				return
			}

			s := fmt.Sprintf("%s listening HTTPS at :443", gMe)
			log.Println(s)
			al.Log(alTxt, s)

			server443 := createServer443(handler, certificate)
			if listenErr = server443.ListenAndServeTLS(proxyConfig.TLSCertFile, proxyConfig.TLSKeyFile); nil != listenErr {
				return
			}
		}()
	}

	wg.Add(1)
	go func() { // HTTP server at port 80
		defer wg.Done()

		s := fmt.Sprintf("%s listening HTTP at :80", gMe)
		log.Println(s)
		al.Log(alTxt, s)

		server80 := createServer80(handler)
		if listenErr = server80.ListenAndServe(); nil != listenErr {
			return
		}
	}()

	wg.Wait()
	exit(fmt.Sprintf("%s: %s (%v)", gMe, "terminating ...", listenErr))
} // main()

/* _EoF_ */
