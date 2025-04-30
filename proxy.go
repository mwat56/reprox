/*
Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

//lint:file-ignore ST1005 - I prefer Capitalisation
//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/mwat56/apachelogger"
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
// `aTarget` is a `tHostConfig` struct that holds the requested hostname
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
	alTxt := "ReProx/createReverseProxy"
	errTxt := "Internal Server Error"

	if nil == aTarget {
		msg := fmt.Sprintf("%s [%v]", errTxt, aTarget)
		apachelogger.Err(alTxt, msg)

		return nil, fmt.Errorf("%s [%v]", errTxt, aTarget)
	}

	if nil != aTarget.destProxy {
		// There's already a running reverse proxy.
		return aTarget.destProxy, nil
	}

	targetURL := aTarget.target
	if nil == targetURL {
		msg := fmt.Sprintf("%s %q", errTxt, targetURL)
		apachelogger.Err(alTxt, msg)

		return nil, fmt.Errorf("%s %q", errTxt, targetURL)
	}

	return httputil.NewSingleHostReverseProxy(targetURL), nil
} // createReverseProxy()

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
	requestHost := strings.ToLower(aRequest.Host)

	// Check if a backend server is available for the requested host.
	ph.conf.RLock()
	target, ok := ph.conf.hostMappings[requestHost]
	ph.conf.RUnlock()
	if !ok {
		msg := fmt.Sprintf("Server %q not found", requestHost)
		apachelogger.Err("ReProx/ServeHTTP", msg)

		// No backend server found: send a `404 Not Found HTTP` response.
		http.Error(aWriter, msg, http.StatusNotFound)
		return
	}

	// Get the existing reverse proxy or create a new one if needed.
	proxy := target.destProxy
	if nil == target.destProxy {
		// No available running reverse proxy yet.
		var err error

		ph.conf.Lock()
		// Create a new reverse proxy for the target backend server.
		if proxy, err = createReverseProxy(&target); nil != err {
			ph.conf.Unlock()
			// If an error occurs while creating the reverse proxy,
			// send a 500 Internal Server Error HTTP response.
			msg := http.StatusText(http.StatusInternalServerError)
			apachelogger.Err("ReProx/ServeHTTP", msg)
			http.Error(aWriter, msg, http.StatusInternalServerError)

			return
		}

		// Update the configuration with the new reverse proxy.
		target.destProxy = proxy
		ph.conf.hostMappings[requestHost] = target
		ph.conf.Unlock()
	}

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
