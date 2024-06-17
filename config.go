/*
Copyright © 2024  M.Watermann, 10247 Berlin, Germany

	    All rights reserved
	EMail : <support@mwat.de>
*/
package reprox

import (
	"fmt"
	"net/http/httputil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/mwat56/ini"
)

//lint:file-ignore ST1017 - I prefer Yoda conditions

type (
	// Structure to pair an external hostname with the internal machine:
	tDestination struct {
		destHost  string
		destProxy *httputil.ReverseProxy
	}

	// List of proxied servers:
	tBackendServers = map[string]tDestination

	// Application specific configuration
	TSetup struct {
		AccessLog   string // (optional) name of page access logfile
		ErrorLog    string // (optional) name of page error logfile
		BackendList *tBackendServers
	}
)

var (
	// Name of the running program:
	gMe = func() string {
		return filepath.Base(os.Args[0])
	}()

	// application specific configuration
	AppSetup *TSetup
)

// `init()` initialises the application setup by reading the configuration
// from an INI file.
//
// The function is called automatically before the `main()` function starts.
func init() {
	AppSetup = readIni()
} // init()

// `readIni()` reads the application configuration from an INI file.
// It returns a pointer to a `TSetup` structure containing the required
// configuration data.
func readIni() *TSetup {
	var (
		// Regular expression to identify `HostX` sections
		isHostRE = regexp.MustCompile(`^\s*(Host\d)\s*$`)
		ok       bool
		s        string
	)

	config, inif := ini.ReadIniData(gMe)
	if (nil == config) || (nil == inif) {
		panic("can't read INI data")
	}

	setup := TSetup{}
	s, ok = config.AsString("AccessLog")
	if !ok {
		s = fmt.Sprintf("%s.%s.log", "access", gMe)
	}
	setup.AccessLog = s

	if s, ok = config.AsString("ErrorLog"); !ok {
		s = fmt.Sprintf("%s.%s.log", "error", gMe)
	}
	setup.ErrorLog = s

	//TODO: process listen port numbers

	sections, sLen := inif.Sections()
	bes := make(tBackendServers, sLen-1) // ignore default section

	for _, section := range sections {
		if "" != isHostRE.FindString(section) {
			outside, ok := inif.AsString(section, "outside")
			if !ok {
				continue
			}
			destURL, ok := inif.AsString(section, "destURL")
			if !ok {
				continue
			}
			bes[outside] = tDestination{destURL, nil}
		}
	} // for
	setup.BackendList = &bes

	return &setup
} // readIni()

/* _EoF_ */
