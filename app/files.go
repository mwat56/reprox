/*
Copyright © 2024  M.Watermann, 10247 Berlin, Germany

	All rights reserved
	EMail : <support@mwat.de>
*/
package main

//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"os"
	"path/filepath"
)

// `ConfDir()` returns the directory path where the configuration files
// for the running application should be stored.
//
// If the current user is root, the directory is "/etc/<program_name>".
// Otherwise, it is "~/.config/<program_name>".
//
// If the directory does not yet exist, it is created with permissions 0770.
//
// Returns:
// - string: The directory path as a string.
//
// NOTE: This function is Linux-specific and only considers only the
// "happy path" (i.e. no proper error handling).
func ConfDir() (rDir string) {
	if 0 == os.Getuid() { // root user
		rDir = filepath.Join("/etc/", filepath.Base(os.Args[0]))
	} else {
		confDir, _ := os.UserConfigDir()
		rDir = filepath.Join(confDir, filepath.Base(os.Args[0]))
	}

	if isDirectory(rDir) {
		return
	}

	if err := os.Mkdir(rDir, 0770); nil != err {
		rDir, _ = os.UserConfigDir()
	}

	return
} // ConfDir()

// `fileExists()` checks if a file exists at the given path.
//
// Parameters:
// - `aFile`: The path to the file to be checked.
//
// Returns:
// - bool: Returns `true` if the file exists, `false` otherwise.
//
// NOTE: If `aFile` is a directory-name this function will return false.
func Exists(aFile string) bool {
	fileInfo, err := os.Stat(aFile)
	if nil == err {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	// Other error occurred

	return (!fileInfo.IsDir())
} // Exists()

// `isDirectory()` checks whether the given path is a directory.
//
// Parameters:
// - `aPath` (string): The path to be checked.
//
// Returns:
// - bool: Returns `true` if the given path is a directory, `false` otherwise.
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

/* _EoF_ */
