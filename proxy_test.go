/*
Copyright © 2025  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func Test_createReverseProxy(t *testing.T) {
	// Create test URLs
	validTarget, err := url.Parse("http://valid.example.com:8080")
	if nil != err {
		t.Fatalf("Failed to parse valid URL: %v", err)
	}

	// Create test cases
	tests := []struct {
		name      string
		target    *tHostConfig
		wantProxy bool
		wantErr   bool
	}{
		{"ValidTargetNewProxy", &tHostConfig{
			target:    validTarget,
			destProxy: nil,
		}, true, false,
		},
		{"ExistingProxy", &tHostConfig{
			target:    validTarget,
			destProxy: httputil.NewSingleHostReverseProxy(validTarget),
		}, true, false,
		},
		{"NilTarget", &tHostConfig{
			target:    nil,
			destProxy: nil,
		}, false, true,
		},
		{"NilConfig", nil, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := createReverseProxy(tt.target)

			// Check error condition
			if (nil != err) != tt.wantErr {
				t.Errorf("createReverseProxy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check proxy creation
			if tt.wantProxy {
				if nil == proxy {
					t.Error("createReverseProxy() returned nil proxy when one was expected")
					return
				}

				// If target had existing proxy, verify it's the same one
				if (nil != tt.target.destProxy) && (proxy != tt.target.destProxy) {
					t.Error("createReverseProxy() created new proxy when existing one should have been returned")
				}

				// Verify proxy configuration
				if nil != tt.target.target {
					// Create test request to verify proxy behaviour
					testReq := httptest.NewRequest("GET", "http://test.com", nil)
					proxy.Director(testReq)

					if testReq.URL.Host != tt.target.target.Host {
						t.Errorf("Proxy director set host to %v, want %v", testReq.URL.Host, tt.target.target.Host)
					}

					if testReq.URL.Scheme != tt.target.target.Scheme {
						t.Errorf("Proxy director set scheme to %v, want %v", testReq.URL.Scheme, tt.target.target.Scheme)
					}
				}
			} else if nil != proxy {
				t.Error("createReverseProxy() returned proxy when nil was expected")
			}
		})
	}

	// Test concurrent access
	t.Run("ConcurrentAccess", func(t *testing.T) {
		target := &tHostConfig{
			target:    validTarget,
			destProxy: nil,
		}

		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				proxy, err := createReverseProxy(target)
				if nil != err {
					t.Errorf("Concurrent createReverseProxy() failed: %v", err)
				}
				if nil == proxy {
					t.Error("Concurrent createReverseProxy() returned nil proxy")
				}
			}()
		}
		wg.Wait()
	})
} // Test_createReverseProxy()

func TestTProxyHandler_ServeHTTP(t *testing.T) {
	// Create test backend servers
	backend1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend1"))
	}))
	defer backend1.Close()

	backend2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("backend2"))
	}))
	defer backend2.Close()

	// Parse backend URLs
	backend1URL, _ := url.Parse(backend1.URL)
	backend2URL, _ := url.Parse(backend2.URL)

	// Create proxy configuration
	config := &tProxyConfig{
		hostMappings: tHostMap{
			"example.com": {
				target:    backend1URL,
				destProxy: nil,
			},
			"example.com:443": {
				target:    backend2URL,
				destProxy: httputil.NewSingleHostReverseProxy(backend2URL),
			},
		},
	}

	// Create proxy handler
	handler := &TProxyHandler{conf: config}
	/* */
	tests := []struct {
		name           string
		host           string
		expectedStatus int
		expectedBody   string
		modifyRequest  func(*http.Request)
	}{
		{
			name:           "ValidHost",
			host:           "example.com",
			expectedStatus: http.StatusOK,
			expectedBody:   "backend1",
		},
		{
			name:           "ValidHostWithPort",
			host:           "example.com:443",
			expectedStatus: http.StatusOK,
			expectedBody:   "backend2",
		},
		{
			name:           "UnknownHost",
			host:           "unknown.com",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "EmptyHost",
			host:           "",
			expectedStatus: http.StatusNotFound,
		},
		{
			name:           "CaseInsensitiveHost",
			host:           "EXAMPLE.COM",
			expectedStatus: http.StatusOK,
			expectedBody:   "backend1",
		},
		{
			name:           "ModifiedRequest",
			host:           "example.com",
			expectedStatus: http.StatusOK,
			expectedBody:   "backend1",
			modifyRequest: func(r *http.Request) {
				r.Header.Set("X-Custom-Header", "test")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test request
			req := httptest.NewRequest("GET", "http://"+tt.host+"/test", nil)
			req.Host = tt.host

			if nil != tt.modifyRequest {
				tt.modifyRequest(req)
			}

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve request
			handler.ServeHTTP(rr, req)

			// Check status code
			if rr.Code != tt.expectedStatus {
				t.Errorf("ServeHTTP() status = %v, want %v", rr.Code, tt.expectedStatus)
			}

			// Check response body for successful requests
			if tt.expectedStatus == http.StatusOK {
				if !strings.Contains(rr.Body.String(), tt.expectedBody) {
					t.Errorf("ServeHTTP() body = %v, want %v", rr.Body.String(), tt.expectedBody)
				}
			}
		})
	}

	// Test concurrent requests
	t.Run("ConcurrentRequests", func(t *testing.T) {
		var wg sync.WaitGroup
		for range 10 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := httptest.NewRequest("GET", "http://example.com/test", nil)
				req.Host = "example.com"
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				if rr.Code != http.StatusOK {
					t.Errorf("Concurrent ServeHTTP() status = %v, want %v", rr.Code, http.StatusOK)
				}
			}()
		}
		wg.Wait()
	})

	// Test error handling when creating reverse proxy
	t.Run("ProxyCreationError", func(t *testing.T) {
		invalidConfig := &tProxyConfig{
			hostMappings: tHostMap{
				"example.com": {
					target:    nil, // This will cause createReverseProxy to fail
					destProxy: nil,
				},
			},
		}
		handler := &TProxyHandler{conf: invalidConfig}

		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Host = "example.com"
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("ServeHTTP() status = %v, want %v", rr.Code, http.StatusInternalServerError)
		}
	})

	// Test proxy reuse
	t.Run("ProxyReuse", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Host = "example.com"
		rr := httptest.NewRecorder()

		// First request should create proxy
		handler.ServeHTTP(rr, req)

		// Get the created proxy
		firstProxy := handler.conf.hostMappings["example.com"].destProxy
		if nil == firstProxy {
			t.Fatal("Proxy was not created")
		}

		// Second request should reuse proxy
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		secondProxy := handler.conf.hostMappings["example.com"].destProxy
		if firstProxy != secondProxy {
			t.Error("Proxy was not reused")
		}
	})
	/* */

	// Test request modification
	t.Run("RequestModification", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		req.Host = "example.com"
		req.Header.Set("X-Original-Header", "original")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if req.URL.Scheme != backend1URL.Scheme {
			t.Errorf("Request scheme not modified, got '%v', want '%v'", req.URL.Scheme, backend1URL.Scheme)
		}
		/*
			if req.URL.Host != backend1URL.Host {
				t.Errorf("Request host not modified, got '%v', want '%v'", req.URL.Host, backend1URL.Host)
			}
		*/
	})
} // TestTProxyHandler_ServeHTTP()

/* _EoF_ */
