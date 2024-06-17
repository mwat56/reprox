/*
Copyright © 2024  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/mwat56/apachelogger"
)

type (
	// Page handler for proxy requests:
	TProxyHandler struct {
		backendServers tBackendServers
	}
)

// `createReverseProxy()` creates a new reverse proxy that routes
// requests to the specified target.
// The target is a URL string that represents the backend server the
// requests to which will be forwarded.
//
// The function returns a pointer to an `httputil.ReverseProxy` instance.
// If an error occurs during the parsing of the target URL, the function
// logs the error and exits the program.
//
// Parameters:
// - `aTarget` (tDestination): The URL struct representing the backend
// server to which the requests will be forwarded.
//
// Return:
// *httputil.ReverseProxy: A pointer to an `httputil.ReverseProxy` instance.
func createReverseProxy(aDestination *tDestination) (*httputil.ReverseProxy, error) {
	if nil != aDestination.destProxy {
		// there's already a running reverse proxy
		return aDestination.destProxy, nil
	}

	targetURL, err := url.ParseRequestURI(aDestination.destHost)
	if nil != err {
		msg := fmt.Sprintf("Internal Server Error [%s]", aDestination.destHost)
		apachelogger.Err("ReProx/createReverseProxy", msg)
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(targetURL), nil
} // createReverseProxy()

// `ServeHTTP()` is the main entry point for the reverse proxy server.
// It handles incoming HTTP requests and forwards them to the
// appropriate backend server.
//
// Parameters:
// - `aWriter`: The `ResponseWriter` to write HTTP response headers and body.
// - `aRequest`: The Request struct containing all the details of the
// incoming HTTP request.
func (ph *TProxyHandler) ServeHTTP(aWriter http.ResponseWriter, aRequest *http.Request) {
	// Check if a backend server is available for the requested host.
	target, ok := ph.backendServers[aRequest.Host]
	if !ok {
		msg := fmt.Sprintf("Backend server %q not found", aRequest.Host)
		apachelogger.Err("ReProx/ServeHTTP", msg)
		// If no backend server is found, send a 404 Not Found HTTP response.
		http.Error(aWriter, msg, http.StatusNotFound)
		return
	}

	// Create a new reverse proxy for the target backend server.
	proxy, err := createReverseProxy(&target)
	if nil != err {
		// If an error occurs while creating the reverse proxy,
		// send a 500 Internal Server Error HTTP response.
		msg := "Internal Server Error"
		// apachelogger.Err("ReProx/ServeHTTP", msg)
		http.Error(aWriter, msg, http.StatusInternalServerError)
		return // exit(err.Error())
	}

	target.destProxy = proxy
	ph.backendServers[aRequest.Host] = target

	// Serve the incoming HTTP request using the reverse proxy.
	proxy.ServeHTTP(aWriter, aRequest)
} // ServeHTTP()

// `NewProxyHandler()` creates a new instance of TProxyHandler.
// It initialises the internal backendServers map with the list of
// available servers.
//
// Returns:
// - *TProxyHandler: A pointer to a new instance of TProxyHandler.
func NewProxyHandler() *TProxyHandler {
	return &TProxyHandler{
		backendServers: *AppSetup.BackendList,
	}
} // NewProxyHandler()

/* _EoF_ */
