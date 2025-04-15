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
	// Page handler for proxy requests:
	TProxyHandler struct {
		conf *tProxyConfig
	}
)

// `createReverseProxy()` creates a new reverse proxy that routes
// requests to the specified target.
//
// The target is a URL struct that holds the request hostname and
// the backend server to which the requests will be forwarded.
//
// The function returns a pointer to an `httputil.ReverseProxy` instance.
// If an error occurs during the parsing of the target URL, the function
// logs the error and exits the program.
//
// Parameters:
//   - `aTarget`: The URL struct holding the backend server to which the requests will be forwarded.
//
// Return:
//   - `*httputil.ReverseProxy`: A pointer to an `httputil.ReverseProxy` instance.
//   - `error`: An error, if the target URL is missing.
func createReverseProxy(aTarget *tHostConfig) (*httputil.ReverseProxy, error) {
	if nil == aTarget {
		msg := fmt.Sprintf("Internal Server Error [%v]", aTarget)
		apachelogger.Err("ReProx/createReverseProxy", msg)
		return nil, fmt.Errorf("Internal Server Error [%v]", aTarget)
	}
	if nil != aTarget.destProxy {
		// there's already a running reverse proxy
		return aTarget.destProxy, nil
	}

	targetURL := aTarget.target
	if nil == targetURL {
		msg := fmt.Sprintf("Internal Server Error [%s]", targetURL)
		apachelogger.Err("ReProx/createReverseProxy", msg)
		return nil, fmt.Errorf("Internal Server Error [%s]", targetURL)
	}

	return httputil.NewSingleHostReverseProxy(targetURL), nil
} // createReverseProxy()

// `ServeHTTP()` is the main entry point for the reverse proxy server.
// It handles incoming HTTP requests and forwards them to the
// appropriate backend server.
//
// Parameters:
//   - `aWriter`: The `ResponseWriter` to write HTTP response headers and body.
//   - `aRequest`: The Request struct containing all the details of the incoming HTTP request.
func (ph *TProxyHandler) ServeHTTP(aWriter http.ResponseWriter, aRequest *http.Request) {
	requestHost := strings.ToLower(aRequest.Host)

	ph.conf.Lock()
	defer ph.conf.Unlock()

	// Check if a backend server is available for the requested host.
	target, ok := ph.conf.hostMappings[requestHost]
	if !ok {
		msg := fmt.Sprintf("Server %q not found", requestHost)
		apachelogger.Err("ReProx/ServeHTTP", msg)

		// No backend server found: send a `404 Not Found HTTP` response
		http.Error(aWriter, msg, http.StatusNotFound)
		return
	}

	proxy := target.destProxy
	if nil == target.destProxy {
		// no available running reverse proxy
		var err error

		// Create a new reverse proxy for the target backend server.
		proxy, err = createReverseProxy(&target)
		if nil != err {
			// If an error occurs while creating the reverse proxy,
			// send a 500 Internal Server Error HTTP response.
			msg := http.StatusText(http.StatusInternalServerError)
			apachelogger.Err("ReProx/ServeHTTP", msg)
			http.Error(aWriter, msg, http.StatusInternalServerError)

			return
		}

		target.destProxy = proxy
		ph.conf.hostMappings[requestHost] = target
	}

	// Serve the incoming HTTP request using the reverse proxy.
	proxy.ServeHTTP(aWriter, aRequest)
} // ServeHTTP()

// ---------------------------------------------------------------------------

// `NewProxyHandler()` creates a new instance of `TProxyHandler`with the
// internal configuration data.
//
// Parameters:
//   - `aConfig`: A pointer to the server configuration.
//
// Returns:
//   - `*TProxyHandler`: A pointer to a new instance of TProxyHandler.
func NewProxyHandler(aConfig *tProxyConfig) *TProxyHandler {
	return &TProxyHandler{
		conf: aConfig,
	}
} // NewProxyHandler()

/* _EoF_ */
