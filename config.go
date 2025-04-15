/*
Copyright © 2024, 2025  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

//lint:file-ignore ST1005 - I prefer Capitalisation
//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mwat56/apachelogger"
)

type (
	// `tHostConfig` represents the configuration for a single host.
	tHostConfig struct {
		target    *url.URL               // Target URL for the backend server
		destProxy *httputil.ReverseProxy // Reverse proxy instance for this host
	}

	// `tHostMap` represents a list of mapping of hostnames to their
	// respective configurations.
	tHostMap map[string]tHostConfig

	// `tProxyConfig` holds the configuration for the reverse proxy.
	tProxyConfig struct {
		sync.RWMutex          // For thread-safe access to configuration
		hostMappings tHostMap // Maps hostnames to their configurations
		AccessLog    string   // Path to access log file
		ErrorLog     string   // Path to error log file
		TLSCertFile  string   // Path to TLS certificate file
		TLSKeyFile   string   // Path to TLS private key file
	}

	// `tConfigFile` represents the JSON configuration file format.
	tConfigFile struct {
		Hosts     map[string]string `json:"hosts"`
		AccessLog string            `json:"access_log,omitempty"`
		ErrorLog  string            `json:"error_log,omitempty"`
		TLSCert   string            `json:"tls_cert,omitempty"`
		TLSKey    string            `json:"tls_key,omitempty"`
	}
)

// --------------------------------------------------------------------------
// helper functions:

// `absDir()` returns `aFilename` as an absolute path.
//
// If `aBaseDir` is an empty string the current directory (`./`) is used.
// Otherwise `aBaseDir` gets prepended to `aFilename` and returned after
// cleaning.
//
// If `aFilename` is an empty string the function's result will be empty.
//
// Parameters:
//   - `aBaseDir`: The base directory to prepend to `aFilename`.
//   - `aFilename`: The filename to make absolute.
//
// Returns:
//   - `string`: The absolute path of `aFilename`.
func absDir(aBaseDir, aFilename string) string {
	if "" == aFilename {
		return aFilename
	}

	if '/' == aFilename[0] {
		return filepath.Clean(aFilename)
	}

	if "" == aBaseDir {
		aBaseDir, _ = filepath.Abs(`./`)
	} else {
		aBaseDir, _ = filepath.Abs(aBaseDir)
	}

	return filepath.Join(aBaseDir, aFilename)
} // absDir()

// `isDirectory()` checks whether the given path is a directory.
//
// Parameters:
//   - `aPath` (string): The path to be checked.
//
// Returns:
//   - `bool`: `true` if the given path is a directory, `false` otherwise.
func isDirectory(aPath string) bool {
	fileInfo, err := os.Stat(aPath)
	if nil == err {
		return fileInfo.IsDir()
	}

	if os.IsNotExist(err) {
		return false
	}

	// Other error occurred
	return false
} // isDirectory()

// --------------------------------------------------------------------------
// `tProxyConfig` methods:

var (
	// Name of the running program:
	gMe = func() string {
		return filepath.Base(os.Args[0])
	}()
)

// `ErrorHandler()` handles errors occurring during the reverse proxy process.
//
// Parameters:
//   - `aWriter`: The `ResponseWriter` to write HTTP response headers.
//   - `aRequest`: Struct containing the details of the incoming HTTP request.
//   - `err`: The error that occurred during the reverse proxy process.
func (pc *tProxyConfig) ErrorHandler(aWriter http.ResponseWriter, aRequest *http.Request, err error) {
	apachelogger.Err("ReProx/ErrorHandler", err.Error())

	aWriter.WriteHeader(http.StatusBadGateway)
} // ErrorHandler()

// `getTarget()` retrieves the target URL for a requested host.
//
// Parameters:
//   - `aRequest`: Struct containing the details of the incoming HTTP request.
//
// Returns:
//   - `*url.URL`: The target URL for the given request host, or `nil` if not found.
func (pc *tProxyConfig) getTarget(aRequest *http.Request) *url.URL {
	pc.RLock()
	defer pc.RUnlock()

	if host, exists := pc.hostMappings[aRequest.Host]; exists {
		return host.target
	}

	return nil
} // getTarget()

// `loadConfigFromFile()` loads the configuration from a JSON file.
//
// Parameters:
//   - `aFilename`: The path to the JSON configuration file.
//
// Returns:
//   - `error`: An error, if the configuration could not be loaded.
func (pc *tProxyConfig) loadConfigFromFile(aFilename string) error {
	info, err := os.Stat(aFilename)
	if nil != err {
		return err
	}

	// Verify file permissions
	if mode := info.Mode().Perm(); 0 != mode&0077 {
		// return fmt.Errorf("Insecure file permissions: %#o (want 600)", mode)
		log.Printf("Warning: Insecure file permissions: %#o (want 600)", mode)
	}

	configData, err := os.ReadFile(aFilename)
	if nil != err {
		return fmt.Errorf("Failed to read config file: %w", err)
	}

	var conf tConfigFile
	if err = json.Unmarshal(configData, &conf); nil != err {
		return fmt.Errorf("Invalid JSON configuration: %w", err)
	}

	if 0 == len(conf.Hosts) {
		return errors.New("missing host mappings in configuration file")
	}

	if "" == conf.AccessLog {
		conf.AccessLog = fmt.Sprintf("%s.%s.log", "access", gMe)
	}
	if "" == conf.ErrorLog {
		conf.ErrorLog = fmt.Sprintf("%s.%s.log", "error", gMe)
	}

	// Update logs and TLS first (atomic operation)
	pc.Lock()
	defer pc.Unlock()

	pc.AccessLog = absDir("", conf.AccessLog)
	pc.ErrorLog = absDir("", conf.ErrorLog)
	pc.TLSCertFile = absDir("", conf.TLSCert)
	pc.TLSKeyFile = absDir("", conf.TLSKey)

	// Update host mappings
	tempMapping := make(tHostMap)
	targetURL := &url.URL{}
	for host, target := range conf.Hosts {
		if targetURL, err = url.Parse(target); nil != err {
			return fmt.Errorf("Invalid target URL for %s: %w", host, err)
		}
		if ("http" != targetURL.Scheme) && ("https" != targetURL.Scheme) {
			return fmt.Errorf("Invalid target URL.scheme for %s: %w", targetURL.Scheme, err)
		}

		tempMapping[host] = tHostConfig{targetURL, nil}
	}
	pc.hostMappings = tempMapping

	return nil
} // loadConfigFromFile()

// --------------------------------------------------------------------------

// `ConfDir()` returns the directory path where the configuration files
// for the running application should be stored.
//
// If the current user is root, the directory is "/etc/<program_name>".
// Otherwise, it is "~/.config/<program_name>".
//
// If the directory does not yet exist, it is created with permissions 0770.
//
// Returns:
//   - `string`: The directory path to use for application-specific configuration files.
//
// NOTE: This function is Linux-specific and considers only the
// "happy path" (i.e. no proper error handling).
func ConfDir() (rDir string) {
	if 0 == os.Getuid() { // root user
		rDir = filepath.Join("/etc/", gMe)
	} else {
		confDir, _ := os.UserConfigDir()
		rDir = filepath.Join(confDir, gMe)
	}

	if isDirectory(rDir) {
		return
	}

	if err := os.Mkdir(rDir, 0770); nil != err {
		rDir, _ = os.UserConfigDir()
	}

	return
} // ConfDir()

// `LoadConfig()` loads the configuration from a JSON file.
//
// Parameters:
//   - `aFilename`: The path to the JSON configuration file.
//
// Returns:
//   - `*tProxyConfig`: A pointer to the loaded configuration.
//   - `error`: An error, if the configuration could not be loaded.
func LoadConfig(aFilename string) (*tProxyConfig, error) {
	result := &tProxyConfig{
		hostMappings: make(tHostMap),
	}

	if err := result.loadConfigFromFile(aFilename); nil != err {
		return nil, err
	}

	return result, nil
} // LoadConfig()

// `NewReverseProxy()` creates a new reverse proxy with the specified
// configuration.
//
// Parameters:
//   - `aConfig`: The configuration for the reverse proxy.
//
// Returns:
//   - `*httputil.ReverseProxy`: A new reverse proxy instance.
func NewReverseProxy(aConfig *tProxyConfig) *httputil.ReverseProxy {
	director := func(aRequest *http.Request) {
		if target := aConfig.getTarget(aRequest); nil != target {
			aRequest.URL.Scheme = target.Scheme
			aRequest.URL.Host = target.Host
			aRequest.Host = target.Host
		} else {
			aRequest.URL.Scheme = "https"
			aRequest.URL.Host = "www.cia.gov"
			aRequest.Host = "www.cia.gov"
		}
	}

	return &httputil.ReverseProxy{
		Director:     director,
		ErrorHandler: aConfig.ErrorHandler,
		Transport: &http.Transport{
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
} // NewReverseProxy()

// `WatchConfigFile()` monitors a configuration file for changes and
// reloads it when modified.
//
// The function runs until the context is cancelled.
//
// Parameters:
//   - `aCtx`: Context for cancellation.
//   - `aPc`: The proxy configuration to update.
//   - `aFilename`: Path to the configuration file to watch.
//   - `aInterval`: How often to check for changes.
func WatchConfigFile(aCtx context.Context, aPc *tProxyConfig, aFilename string, aInterval time.Duration) {
	if nil == aPc {
		return
	}
	prevModTime := time.Time{}

	ticker := time.NewTicker(aInterval)
	defer ticker.Stop()

	for {
		select {
		case <-aCtx.Done():
			apachelogger.Err("ReProx/WatchConfigFile", aCtx.Err().Error())
			return

		case <-ticker.C:
			info, err := os.Stat(aFilename)
			if nil != err {
				apachelogger.Err("ReProx/WatchConfigFile", err.Error())
				continue
			}

			if modTime := info.ModTime(); modTime != prevModTime {
				if err = aPc.loadConfigFromFile(aFilename); nil != err {
					apachelogger.Err("ReProx/WatchConfigFile", err.Error())
				} else {
					prevModTime = modTime
					apachelogger.Log("ReProx/WatchConfigFile", "Configuration successfully reloaded")
				}
			}
		}
	}
} // WatchConfigFile()

// --------------------------------------------------------------------------

/* _EoF_ */
