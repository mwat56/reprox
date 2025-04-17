/*
Copyright © 2025  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func Test_absDir(t *testing.T) {
	type tArgs struct {
		aBaseDir string
		aDirFile string
	}
	tests := []struct {
		name string
		args tArgs
		want string
	}{
		{"EmptyArgs", tArgs{"", ""}, ""},
		{"EmptyBase", tArgs{"", "tc1"}, "/home/matthias/devel/Go/src/github.com/mwat56/reprox/tc1"},
		{"EmptyFile", tArgs{"tc2", ""}, ""},
		{"NoEmpty", tArgs{"tc3", "tc4"}, "/home/matthias/devel/Go/src/github.com/mwat56/reprox/tc3/tc4"},
		{"Slash", tArgs{"tc5", "/tc6"}, "/tc6"},
		{"DoubleSlash", tArgs{"tc7", "//tc8"}, "/tc8"},
		{"RootDir", tArgs{"/", "tc9"}, "/tc9"},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := absDir(tt.args.aBaseDir, tt.args.aDirFile); got != tt.want {
				t.Errorf("absDir() = %v, want %v", got, tt.want)
			}
		})
	}
} // Test_absDir()

func Test_ConfDir(t *testing.T) {
	tests := []struct {
		name     string
		wantRDir string
	}{
		{"ConfDir", "/home/matthias/.config/reprox.test"},
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotRDir := ConfDir(); gotRDir != tt.wantRDir {
				t.Errorf("ConfDir() = %v, want %v", gotRDir, tt.wantRDir)
			}
		})
	}
} // Test_ConfDir()

func Test_getTarget(t *testing.T) {
	// Create test URLs
	target1, _ := url.Parse("http://backend1.local:8080")
	target2, _ := url.Parse("https://backend2.local:9090")

	// Setup test config
	pc := &tProxyConfig{
		hostMappings: tHostMap{
			"example.com": tHostConfig{
				target:    target1,
				destProxy: nil,
			},
			"example.com:443": tHostConfig{
				target:    target2,
				destProxy: nil,
			},
		},
	}

	tests := []struct {
		name    string
		request *http.Request
		want    *url.URL
	}{
		{"ExistingHost", &http.Request{Host: "example.com"}, target1},
		{"ExistingHostWithPort", &http.Request{Host: "example.com:443"}, target2},
		{"NonExistentHost", &http.Request{Host: "unknown.com"}, nil},
		{"EmptyHost", &http.Request{Host: ""}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pc.getTarget(tt.request)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getTarget() = %v, want %v", got, tt.want)
			}
		})
	}
} // Test_getTarget()

func TestLoadConfig(t *testing.T) {
	pc := &tProxyConfig{}

	tests := []struct {
		name     string
		filename string
		want     *tProxyConfig
		wantErr  bool
	}{
		{"Empty", "", nil, true},
		{"MissingLog", "FileNotFound.json", nil, true},
		{"EmptyLog", "tc2.json", nil, true},
		{"InvalidLog", "tc3.json", nil, true},
		{"CorrectConf", "tc4.json", pc, false},
		{"NoMappings", "tc5.json", nil, true},
		{"NoLogs", "tc6.json", pc, false},
		{"InvalidMappings", "tc7.json", nil, true},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if nil == got {
				if nil != tt.want {
					t.Errorf("LoadConfig() = %v, want %v", got, tt.want)
				}
				return
			}

			//TODO: compare `got` with `tt.want`

		})
	}
} // TestLoadConfig()

func Test_loadConfigFile(t *testing.T) {
	var pc *tProxyConfig

	tests := []struct {
		name     string
		filename string
		wantErr  bool
	}{
		{"Empty", "", true},
		{"MissingLog", "FileNotFound.json", true},
		{"EmptyLog", "tc2.json", true},
		{"InvalidLog", "tc3.json", true},
		{"CorrectConf", "tc4.json", false},
		{"NoMappings", "tc5.json", true},
		{"NoLogs", "tc6.json", false},
		{"InvalidMappings", "tc7.json", true},

		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc = &tProxyConfig{}
			if err := pc.loadConfigFile(tt.filename); (err != nil) != tt.wantErr {
				t.Errorf("tProxyConfig.loadConfigFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
} // Test_loadConfigFile()

func TestNewReverseProxy(t *testing.T) {
	// Create test URLs
	target1, _ := url.Parse("http://backend1.local:8080")
	target2, _ := url.Parse("https://backend2.local:9090")

	// Setup test config
	pc := &tProxyConfig{
		hostMappings: tHostMap{
			"example.com":        tHostConfig{target1, nil},
			"secure.example.com": tHostConfig{target2, nil},
		},
	}

	// Create the reverse proxy
	proxy := NewReverseProxy(pc)

	// Test cases
	tests := []struct {
		name          string
		request       *http.Request
		wantScheme    string
		wantHost      string
		wantTransport bool
	}{
		{"ExistingHost", &http.Request{
			Host: "example.com",
			URL:  &url.URL{},
		}, "http", "backend1.local:8080", true,
		},
		{"SecureHost", &http.Request{
			Host: "secure.example.com",
			URL:  &url.URL{},
		}, "https", "backend2.local:9090", true,
		},
		{"UnknownHost", &http.Request{
			Host: "unknown.com",
			URL:  &url.URL{},
		}, "https", "www.cia.gov", true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the director function
			proxy.Director(tt.request)

			// Check URL scheme
			if tt.request.URL.Scheme != tt.wantScheme {
				t.Errorf("Scheme = %v, want %v", tt.request.URL.Scheme, tt.wantScheme)
			}

			// Check Host
			if tt.request.URL.Host != tt.wantHost {
				t.Errorf("Host = %v, want %v", tt.request.URL.Host, tt.wantHost)
			}

			// Check if Transport is configured
			if tt.wantTransport {
				transport, ok := proxy.Transport.(*http.Transport)
				if !ok {
					t.Error("Transport not configured correctly")
				}

				// Verify transport timeouts
				if transport.IdleConnTimeout != 90*time.Second {
					t.Errorf("IdleConnTimeout = %v, want %v", transport.IdleConnTimeout, 90*time.Second)
				}
				if transport.TLSHandshakeTimeout != 10*time.Second {
					t.Errorf("TLSHandshakeTimeout = %v, want %v", transport.TLSHandshakeTimeout, 10*time.Second)
				}
				if transport.ExpectContinueTimeout != 1*time.Second {
					t.Errorf("ExpectContinueTimeout = %v, want %v", transport.ExpectContinueTimeout, 1*time.Second)
				}
			}

			// Verify ErrorHandler is set
			if proxy.ErrorHandler == nil {
				t.Error("ErrorHandler not configured")
			}
		})
	}

	// Test error handling
	errorTestRequest := httptest.NewRequest("GET", "http://example.com", nil)
	errorTestWriter := httptest.NewRecorder()
	testError := errors.New("test error")

	proxy.ErrorHandler(errorTestWriter, errorTestRequest, testError)

	if errorTestWriter.Code != http.StatusBadGateway {
		t.Errorf("ErrorHandler status = %d, want %d", errorTestWriter.Code, http.StatusBadGateway)
	}
} // TestNewReverseProxy()

func TestSaveConfig(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "config_test_*")
	if nil != err {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test configuration
	testConfig := &tProxyConfig{
		hostMappings: tHostMap{
			"example.com": tHostConfig{
				target: &url.URL{
					Scheme: "http",
					Host:   "localhost:8080",
				},
			},
			"test.com": tHostConfig{
				target: &url.URL{
					Scheme: "https",
					Host:   "backend:9000",
				},
			},
		},
		AccessLog:   "/var/log/access.log",
		ErrorLog:    "/var/log/error.log",
		TLSCertFile: "/etc/ssl/cert.pem",
		TLSKeyFile:  "/etc/ssl/key.pem",
		MaxRequests: 150,
		WindowSize:  time.Duration(120) * time.Second,
	}

	// Test cases
	tests := []struct {
		name     string
		config   *tProxyConfig
		wantErr  bool
		validate func(*testing.T, string)
	}{
		{
			name:    "ValidConfig",
			config:  testConfig,
			wantErr: false,
			validate: func(t *testing.T, filename string) {
				// Read and parse the saved file
				data, err := os.ReadFile(filename)
				if nil != err {
					t.Errorf("Failed to read saved config: %v", err)
					return
				}

				var saved tConfigFile
				if err := json.Unmarshal(data, &saved); nil != err {
					t.Errorf("Failed to parse saved config: %v", err)
					return
				}

				// Verify contents
				if len(saved.Hosts) != 2 {
					t.Errorf("Expected 2 hosts, got %d", len(saved.Hosts))
				}
				if saved.Hosts["example.com"] != "http://localhost:8080" {
					t.Errorf("Unexpected host mapping: %s", saved.Hosts["example.com"])
				}
				if saved.AccessLog != "/var/log/access.log" {
					t.Errorf("Unexpected access log: %s", saved.AccessLog)
				}
				if saved.MaxRequests != 150 {
					t.Errorf("Unexpected max requests: %d", saved.MaxRequests)
				}
				if saved.WindowSize != 120 {
					t.Errorf("Unexpected window size: %d", saved.WindowSize)
				}

				// Verify file permissions
				info, err := os.Stat(filename)
				if nil != err {
					t.Errorf("Failed to stat config file: %v", err)
					return
				}
				if mode := info.Mode().Perm(); mode != 0600 {
					t.Errorf("Unexpected file permissions: %o", mode)
				}
			},
		},
		{
			name:    "InvalidPath",
			config:  testConfig,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := filepath.Join(tmpDir, tt.name+".json")
			if tt.name == "InvalidPath" {
				filename = filepath.Join(tmpDir, "non-existent", "config.json")
			}

			err := tt.config.SaveConfig(filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("SaveConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, filename)
			}
		})
	}
} // TestSaveConfig()

func TestWatchConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpFile, err := os.CreateTemp("", "config_*.json")
	if nil != err {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	// Initial config content
	initialConfig := `{
		"hosts": {
		"example.com": "http://backend1.local:1111"
	},
	"accessLog": "access.log",
	"errorLog": "error.log"
}`
	if err := os.WriteFile(tmpName, []byte(initialConfig), 0600); nil != err {
		t.Fatalf("Failed to write initial config: %v", err)
	}

	// Create proxy config
	pc, err := LoadConfig(tmpName)
	if nil != err {
		t.Fatalf("Failed to load initial config: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Start watching in a goroutine
	go WatchConfigFile(ctx, pc, tmpName, 100*time.Millisecond)

	// Wait for watcher to start
	runtime.Gosched()

	// Test cases
	tests := []struct {
		name       string
		config     string
		wantHost   string
		wantTarget string
		wantError  bool
	}{
		{
			name: "UpdateValidConfig",
			config: `{
			"hosts": {
				"example.com": "http://backend2.local:2222"
			},
			"accessLog": "access.log",
			"errorLog": "error.log"
		}`,
			wantHost:   "example.com",
			wantTarget: "http://backend2.local:2222",
			wantError:  false,
		},
		// ---
		{
			name: "InvalidConfig",
			config: `{
			"hosts": {
				"example.com": "invalid:url"
			}
		}`,
			wantHost:   "example.com",
			wantTarget: "http://backend2.local:2222", // Should retain previous valid config
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write new config
			if err := os.WriteFile(tmpName, []byte(tt.config), 0600); nil != err {
				t.Fatalf("Failed to write config: %v", err)
			}

			// Wait for config reload
			time.Sleep(800 * time.Millisecond)

			// Verify config update
			pc.RLock()
			host, exists := pc.hostMappings[tt.wantHost]
			pc.RUnlock()

			if !exists {
				t.Errorf("Host %s not found in config", tt.wantHost)
				return
			}

			if host.target.String() != tt.wantTarget {
				t.Errorf("got = '%v', want '%v'",
					host.target.String(), tt.wantTarget)
			}
		})
	}

	// Test nil config
	t.Run("NilConfig", func(t *testing.T) {
		// Should return immediately without panic
		WatchConfigFile(ctx, nil, tmpName, 100*time.Millisecond)
	})

	// Test non-existent file
	t.Run("NonExistentFile", func(t *testing.T) {
		nonExistentCtx, nonExistentCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer nonExistentCancel()

		WatchConfigFile(nonExistentCtx, pc, "non-existent.json", 100*time.Millisecond)
		// Should log error but not panic
	})

	// Test context cancellation
	t.Run("ContextCancellation", func(t *testing.T) {
		cancelCtx, cancelFunc := context.WithCancel(context.Background())
		done := make(chan struct{})

		go func() {
			WatchConfigFile(cancelCtx, pc, tmpName, 100*time.Millisecond)
			close(done)
		}()

		// Cancel context after a short delay
		time.Sleep(200 * time.Millisecond)
		cancelFunc()

		// Wait for WatchConfigFile to return
		select {
		case <-done:
			// Success - function returned after context cancellation
		case <-time.After(500 * time.Millisecond):
			t.Error("WatchConfigFile did not return after context cancellation")
		}
	})
} // TestWatchConfigFile()

/* _EoF_ */
