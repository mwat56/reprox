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
		sync.RWMutex               // For thread-safe access to configuration
		hostMappings tHostMap      // Maps hostnames to their configurations
		AccessLog    string        // Path to access log file
		ErrorLog     string        // Path to error log file
		TLSCertFile  string        // Path to TLS certificate file
		TLSKeyFile   string        // Path to TLS private key file
		MaxRequests  uint          // Maximum number of requests allowed in the time window
		WindowSize   time.Duration // Time window in seconds for rate limiting
	}

	// `tConfigFile` represents the JSON configuration file format.
	tConfigFile struct {
		Hosts       map[string]string `json:"hosts"`
		AccessLog   string            `json:"access_log,omitempty"`
		ErrorLog    string            `json:"error_log,omitempty"`
		TLSCert     string            `json:"tls_cert,omitempty"`
		TLSKey      string            `json:"tls_key,omitempty"`
		MaxRequests uint              `json:"max_requests,omitempty"`
		WindowSize  uint              `json:"window_size,omitempty"`
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

	aWriter.WriteHeader(http.StatusBadGateway) // 502 Bad Gateway
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

// `loadConfigFile()` loads the configuration from a JSON file.
//
// Parameters:
//   - `aFilename`: The path/name of the JSON configuration file.
//
// Returns:
//   - `error`: An error, if the configuration could not be loaded.
func (pc *tProxyConfig) loadConfigFile(aFilename string) error {
	errTxt := "ReProx/loadConfigFile"

	// Check if the file exists and is not a directory
	fileInfo, err := os.Stat(aFilename)
	if nil != err {
		err = fmt.Errorf("Failed to accessed config file '%s': %w",
			aFilename, err)
		apachelogger.Err(errTxt, err.Error())
		return err
	}
	if fileInfo.IsDir() {
		msg := fmt.Sprintf("Configuration name points to a directory: %s",
			aFilename)
		apachelogger.Err(errTxt, msg)
		return errors.New(msg)
	}

	// Verify file permissions
	if mode := fileInfo.Mode().Perm(); 0 != mode&0077 {
		msg := fmt.Sprintf("Warning: Insecure file permissions: %#o (want 600)",
			mode)
		apachelogger.Log(errTxt, msg)
	}

	configData, err := os.ReadFile(aFilename)
	if nil != err {
		err = fmt.Errorf("Failed to read config file '%s': %w",
			aFilename, err)
		apachelogger.Err(errTxt, err.Error())
		return err
	}

	var fconf tConfigFile
	if err = json.Unmarshal(configData, &fconf); nil != err {
		err = fmt.Errorf("Failed to parse config file: '%s': %w",
			aFilename, err)
		apachelogger.Err(errTxt, err.Error())
		return err
	}

	if 0 == len(fconf.Hosts) {
		err = fmt.Errorf("Missing host mappings in config file: '%s'",
			aFilename)
		apachelogger.Err(errTxt, err.Error())
		return err
	}

	if "" == fconf.AccessLog {
		fconf.AccessLog = fmt.Sprintf("%s.%s.log", "access", gMe)
	}
	if "" == fconf.ErrorLog {
		fconf.ErrorLog = fmt.Sprintf("%s.%s.log", "error", gMe)
	}
	if "" == fconf.TLSCert {
		fconf.TLSCert = fmt.Sprintf("/etc/ssl/%s.pem", gMe)
	}
	if "" == fconf.TLSKey {
		fconf.TLSKey = fmt.Sprintf("/etc/ssl/%s.key", gMe)
	}

	// Set rate limiting defaults if not specified
	if fconf.MaxRequests <= 0 {
		fconf.MaxRequests = 100 // default to 100 requests
	}
	if fconf.WindowSize <= 0 {
		fconf.WindowSize = 60 // default to 60 seconds
	}

	// Update logs and TLS first (atomic operation)
	pc.Lock()
	defer pc.Unlock()

	// Update host mappings
	tempMapping := make(tHostMap)
	targetURL := &url.URL{}
	for host, target := range fconf.Hosts {
		if targetURL, err = url.Parse(target); nil != err {
			err = fmt.Errorf("Invalid target URL in config file: '%s': %w",
				aFilename, err)
			apachelogger.Err(errTxt, err.Error())
			return err
		}
		if ("http" != targetURL.Scheme) && ("https" != targetURL.Scheme) {
			err = fmt.Errorf("Invalid target URL scheme for '%s'",
				targetURL.Scheme)
			apachelogger.Err(errTxt, err.Error())
			return err
		}

		tempMapping[host] = tHostConfig{targetURL, nil}
	}
	pc.hostMappings = tempMapping

	pc.AccessLog = absDir("", fconf.AccessLog)
	pc.ErrorLog = absDir("", fconf.ErrorLog)
	pc.TLSCertFile = absDir("", fconf.TLSCert)
	pc.TLSKeyFile = absDir("", fconf.TLSKey)
	pc.MaxRequests = fconf.MaxRequests
	pc.WindowSize = time.Duration(fconf.WindowSize) * time.Second

	return nil
} // loadConfigFile()

// `SaveConfig()` writes the current configuration to a JSON file.
//
// Parameters:
//   - `aFilename`: The path/name of the JSON configuration file.
//
// Returns:
//   - `error`: An error if the configuration could not be saved.
func (pc *tProxyConfig) SaveConfig(aFilename string) error {
	pc.RLock()
	defer pc.RUnlock()

	// Convert internal host mappings to the format used in the config file
	hosts := make(map[string]string)
	for host, config := range pc.hostMappings {
		hosts[host] = config.target.String()
	}

	// Create config file structure
	conf := tConfigFile{
		Hosts:       hosts,
		AccessLog:   pc.AccessLog,
		ErrorLog:    pc.ErrorLog,
		TLSCert:     pc.TLSCertFile,
		TLSKey:      pc.TLSKeyFile,
		MaxRequests: pc.MaxRequests,
		WindowSize:  uint(pc.WindowSize.Seconds()),
	}

	// Convert to JSON with indentation
	configData, err := json.MarshalIndent(conf, "", "\t")
	if nil != err {
		err = fmt.Errorf("Failed to marshal configuration to JSON: '%s': %w", aFilename, err)
		apachelogger.Err("ReProx/SaveConfig", err.Error())
		return err
	}

	// Create temporary file in the same directory
	dir := filepath.Dir(aFilename)
	tmpFile, err := os.CreateTemp(dir, "*.tmp")
	if nil != err {
		err = fmt.Errorf("Failed to create temporary file: %w", err)
		apachelogger.Err("ReProx/SaveConfig", err.Error())
		return err
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName) // Clean up in case of failure

	// Write configuration to temporary file
	if _, err = tmpFile.Write(configData); nil != err {
		_ = tmpFile.Close()
		err = fmt.Errorf("Failed to write config to temporary file '%s': %w", tmpName, err)
		apachelogger.Err("ReProx/SaveConfig", err.Error())
		return err
	}
	if err = tmpFile.Close(); nil != err {
		err = fmt.Errorf("Failed to close temporary file '%s': %w", tmpName, err)
		apachelogger.Err("ReProx/SaveConfig", err.Error())
		return err
	}

	// Set file permissions to match `loadConfigFile()` expectations
	if err = os.Chmod(tmpName, 0600); nil != err {
		err = fmt.Errorf("Failed to set file permissions for '%s': %w", tmpName, err)
		apachelogger.Err("ReProx/SaveConfig", err.Error())
		return err
	}

	// Atomically replace the old config file
	if err = os.Rename(tmpName, aFilename); nil != err {
		err = fmt.Errorf("Failed to save configuration file '%s': %w", aFilename, err)
		apachelogger.Err("ReProx/SaveConfig", err.Error())
		return err
	}

	return nil
} // SaveConfig()

// --------------------------------------------------------------------------

// `ConfDir()` returns the directory path where the configuration files
// for the running application should be stored.
//
// If the current user is root, the directory is "/etc/<program_name>".
// Otherwise, it is "~/.config/<program_name>".
//
// If the directory does not yet exist, it is created with permissions 0750.
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

	if err := os.Mkdir(rDir, 0750); nil != err {
		rDir, _ = os.UserConfigDir()
	}

	return
} // ConfDir()

// `LoadConfig()` loads the configuration from a JSON file.
//
// Parameters:
//   - `aFilename`: The path/name of the JSON configuration file.
//
// Returns:
//   - `*tProxyConfig`: A pointer to the loaded configuration.
//   - `error`: An error, if the configuration could not be loaded.
func LoadConfig(aFilename string) (*tProxyConfig, error) {
	result := &tProxyConfig{
		hostMappings: make(tHostMap),
	}

	if err := result.loadConfigFile(aFilename); nil != err {
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
//   - `aFilename`: Path/name of the configuration file to watch.
//   - `aInterval`: How often to check for changes.
func WatchConfigFile(aCtx context.Context, aPc *tProxyConfig, aFilename string, aInterval time.Duration) {
	if nil == aPc {
		return
	}
	errTxt := "ReProx/WatchConfigFile"

	fileInfo, err := os.Stat(aFilename)
	if nil != err {
		apachelogger.Err(errTxt, err.Error())
		return
	}

	if fileInfo.IsDir() {
		apachelogger.Err(errTxt, "Config name points to a directory")
		return
	}

	var modTime time.Time
	prevModTime := fileInfo.ModTime()
	ticker := time.NewTicker(aInterval)
	defer ticker.Stop()

	for {
		select {
		case <-aCtx.Done():
			apachelogger.Err(errTxt, aCtx.Err().Error())
			return

		case <-ticker.C:
			if fileInfo, err = os.Stat(aFilename); nil != err {
				apachelogger.Err(errTxt, err.Error())
				continue
			}

			if modTime = fileInfo.ModTime(); modTime != prevModTime {
				if err = aPc.loadConfigFile(aFilename); nil != err {
					apachelogger.Err(errTxt, err.Error())
				} else {
					prevModTime = modTime
					apachelogger.Log(errTxt, "Configuration successfully reloaded")
				}
			} // if
		} // select
	} // for
} // WatchConfigFile()

/* _EoF_ */
