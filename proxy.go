/*
Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

//lint:file-ignore ST1005 - I prefer Capitalisation
//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	al "github.com/mwat56/apachelogger"
	se "github.com/mwat56/sourceerror"
)

type (
	// `TProxyHandler` is the page handler for proxy requests.
	TProxyHandler struct {
		_    struct{}
		conf *tProxyConfig
	}
)

// `createReverseProxy()` creates a new reverse proxy that routes requests
// to the specified target.
//
// `aTarget` is a `tHostConfig` struct that holds the requested hostname,
// and the backend server to which requests will be forwarded.
//
// The function returns a pointer to an `httputil.ReverseProxy` instance.
// If an error occurs during the parsing of the target URL, the function
// logs the error and exits the program.
//
// Parameters:
//   - `aTarget`: A `tHostConfig` struct holding the backend server to
//     which requests will be forwarded.
//
// Returns:
//   - `*httputil.ReverseProxy`: A pointer to an `httputil.ReverseProxy`
//     instance.
//   - `error`: An error, if the target URL is missing.
func createReverseProxy(aTarget *tHostConfig) (*httputil.ReverseProxy, error) {
	const alTxt = "ReProx/createReverseProxy"

	if nil == aTarget {
		msg := fmt.Sprintf("Missing target '%v'", aTarget)
		al.Err(alTxt, msg)

		return nil, se.New(errors.New(msg), 4)
	}

	if nil != aTarget.destProxy {
		// There's already a running reverse proxy.
		return aTarget.destProxy, nil
	}

	if nil == aTarget.target {
		msg := fmt.Sprintf("Missing target URL '%v'", aTarget)
		al.Err(alTxt, msg)

		return nil, se.New(errors.New(msg), 4)
	}

	return newReverseProxy(aTarget), nil
} // createReverseProxy()

// `newReverseProxy()` creates a new reverse proxy for the specified target.
//
// Parameters:
//   - `aTarget`: The target for the reverse proxy.
//
// Returns:
//   - `*httputil.ReverseProxy`: A new reverse proxy instance.
func newReverseProxy(aTarget *tHostConfig) *httputil.ReverseProxy {
	director := func(aRequest *http.Request) {
		targetURL := aTarget.target
		if nil != targetURL {
			aRequest.URL.Scheme = targetURL.Scheme
			aRequest.URL.Host = targetURL.Host
			targetQuery := targetURL.RawQuery
			if "" == targetQuery || "" == aRequest.URL.RawQuery {
				aRequest.URL.RawQuery = targetQuery + aRequest.URL.RawQuery
			} else {
				aRequest.URL.RawQuery = targetQuery + "&" + aRequest.URL.RawQuery
			}
		} else {
			al.Err("ReProx/director", "Missing target URL")
			aRequest.URL.Scheme = "https"
			aRequest.URL.Host = "www.cia.gov"
		}
		aRequest.Host = aRequest.URL.Host
	} // director()

	return &httputil.ReverseProxy{
		Director: director,
		ErrorHandler: func(aWriter http.ResponseWriter, aRequest *http.Request, aErr error) {
			al.Err("ReProx/ErrorHandler", se.New(aErr, 1).Error())

			aWriter.WriteHeader(http.StatusBadGateway) // 502 Bad Gateway
		},
		Transport: &http.Transport{
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			// Add connection pooling settings
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
		},
	}
} // newReverseProxy()

type (
	// `IConnCloser` is an interface for types that can close idle connections.
	IConnCloser interface {
		CloseIdleConnections()
	}
)

// `CloseIdleConnections()` closes all idle connections in the proxy's
// transport layer.
func (ph *TProxyHandler) CloseIdleConnections() {
	const alTxt = "ReProx/CloseIdleConnections"

	ph.conf.RLock()
	defer ph.conf.RUnlock()

	for _, config := range ph.conf.hostMap {
		if proxy := config.destProxy; nil != proxy {
			if transport, ok := proxy.Transport.(*http.Transport); ok {
				al.Log(alTxt, "Closing idle transport connections")
				transport.CloseIdleConnections()
			} else if closer, ok := proxy.Transport.(IConnCloser); ok {
				al.Log(alTxt, "Closing idle connections")
				closer.CloseIdleConnections()
			}
		}
	}
} // CloseIdleConnections()

// `ServeHTTP()` is the main entry point for the reverse proxy server.
// It handles incoming HTTP requests and forwards them to the
// appropriate backend server.
//
// Parameters:
//   - `aWriter`: The `ResponseWriter` to write HTTP response headers
//     and body.
//   - `aRequest`: The request containing all the details of the incoming
//     HTTP request.
func (ph *TProxyHandler) ServeHTTP(aWriter http.ResponseWriter, aRequest *http.Request) {
	const alTxt = "ReProx/ServeHTTP"
	var msg string

	requestAddress := aRequest.RemoteAddr
	requestedHost := strings.ToLower(aRequest.Host)
	requestedURL := aRequest.URL.String()
	al.AnonymiseURLs = false

	// Check if a backend server is available for the requested host.
	target, ok := ph.conf.getTarget(requestedHost)
	if !ok {
		msg = fmt.Sprintf("Server '%s' not found", requestedHost)
		al.Err(alTxt, msg)

		// No backend server found: send a `404 Not Found HTTP` response.
		http.Error(aWriter, msg, http.StatusNotFound) // 404
		return
	}

	// Get the existing reverse proxy or create a new one if needed.
	proxy := target.destProxy
	if nil == proxy {
		// No available running reverse proxy yet.
		var err error

		// Create a new reverse proxy for the target backend server.
		if proxy, err = createReverseProxy(&target); nil != err {
			// If an error occurs while creating the reverse proxy,
			// send a 500 Internal Server Error HTTP response.
			msg = http.StatusText(http.StatusInternalServerError) // 500
			al.Err(alTxt, msg)
			http.Error(aWriter, msg, http.StatusInternalServerError) // 500

			return
		}

		// Update the configuration with the new reverse proxy.
		target.destProxy = proxy
		ph.conf.setTarget(requestedHost, target)
	}
	msg = fmt.Sprintf("%s %s %s",
		requestAddress, aRequest.Method, requestedURL)
	al.Log("", msg)

	// Serve the incoming HTTP request using the reverse proxy.
	proxy.ServeHTTP(aWriter, aRequest)
} // ServeHTTP()

// ---------------------------------------------------------------------------

// `New()` creates a new instance of `TProxyHandler` with the provided
// configuration data.
//
// Parameters:
//   - `aConfig`: A pointer to the server configuration.
//
// Returns:
//   - `*TProxyHandler`: A pointer to a new instance of TProxyHandler.
func New(aConfig *tProxyConfig) *TProxyHandler {
	return &TProxyHandler{
		conf: aConfig,
	}
} // New()

/* _EoF_ */
