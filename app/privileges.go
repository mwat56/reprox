/*
Copyright © 2024  M.Watermann, 10247 Berlin, Germany

		All rights reserved
	EMail : <support@mwat.de>
*/
package main

//lint:file-ignore ST1017 - I prefer Yoda conditions

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/mwat56/apachelogger"
	se "github.com/mwat56/sourceerror"
)

// `ChRoot()` changes the root directory of the process to "/tmp". This is
// done using the `syscall.Chroot()` function, which takes the new root
// directory as an argument. If an error occurs during the chroot operation,
// it is logged.
//
// The purpose of changing the root directory to "/tmp" is to isolate the
// process from the rest of the file system and limit its access to the
// "/tmp" directory only. This is a common technique used in
// security-sensitive applications to prevent unauthorized access to
// sensitive files and directories.
//
// Parameters:
//   - None
//
// Returns:
// - error: An error if it encounters any issues while changing the root
// directory.
func ChRoot() error {

	// This function is responsible for changing the root directory
	// of the process to "/tmp". This is done using the `syscall.Chroot()`
	// function, which takes the new root directory as an argument. If
	// an error occurs during the chroot operation, it is logged.

	// The purpose of changing the root directory to "/tmp" is to isolate
	// the process from the rest of the file system and limit its access
	// to the "/tmp" directory only. This is a common technique used in
	// security-sensitive applications to prevent unauthorized access to
	// sensitive files and directories.
	if err := syscall.Chroot("/tmp"); nil != err {
		apachelogger.Err("",
			fmt.Sprintf("Failed chroot(/tmp): %v", err))
		return se.Wrap(err, 3)
	}

	return nil
} // Chroot()

// `DropCapabilities()` drops all capabilities of the process.
//
// It uses the `unix.Capset()` function to set the capabilities of the process.
// That function uses the `RawSyscall()` function to call the `SYS_CAPSET`
// system call. That system call is used to set the capabilities of a process.
//
// Returns:
// - `error`: The error if any occurs.
func DropCapabilities() error {
	data := unix.CapUserData{
		Effective:   uint32(0),
		Permitted:   uint32(0),
		Inheritable: uint32(0)}
	header := unix.CapUserHeader{}

	// The `unix.Capset()` function is used to set the capabilities of a
	// process.
	// It takes two arguments: `hdr` and `data`. The `hdr` argument is a
	// pointer to a `CapUserHeader` struct, which contains information
	// about the process, such as its process ID and the size of the
	// capability set.
	//
	// The `data` argument is a pointer to a `CapUserData` struct, which
	// contains the actual capability set that needs to be set for the
	// process.

	// The function uses the `RawSyscall` function to call the `SYS_CAPSET`
	// system call. This system call is used to set the capabilities of a
	// process. The `RawSyscall` function takes three arguments: the number
	// of the system call to be called, the first argument to be passed to
	// the system call, and the second argument to be passed to the system
	// call. In this case, the first argument is the pointer to the
	//`CapUserHeader` struct, and the second argument is the pointer to
	// the `CapUserData` struct.
	//
	// If the `SYS_CAPSET` system call returns an error (i.e., if `e1` is
	// not equal to 0), then the function sets the `err` variable to the
	// error returned.
	if err := unix.Capset(&header, &data); nil != err {
		apachelogger.Err("",
			fmt.Sprintf("Failed to set Capabilities: %v", err))
		return se.Wrap(err, 3)
	}

	return nil
} // dropCapabilities()

// `DropPrivileges()` drops all privileges of the process.
//
// It uses the `Unshare()` function to isolate the process from the parent
// process. It then changes the root directory of the process to "/tmp" using
// the `ChRoot()` function. After that, it drops all capabilities of the
// process using the `DropCapabilities()` function. Finally, it mounts a tmpfs
// filesystem at "/tmp" with the `MS_RDONLY` flag using the `Mount()` function.
//
// Returns:
// - `rErr`: An error if it encounters any issues while dropping the privileges.
func DropPrivileges() error {
	var (
		err error
		// result sourceerror.ErrSource
	)
	// pw, err := syscall.Getpwnam("nobody")
	// if nil != err {
	// 	fmt.Println("getpwnam(nobody):", err)
	// }
	// DropUID(uint32(pw.Uid), uint32(pw.Gid))

	err = DropUID(-1, -1)
	if nil != err {
		return err
	}
	err = DropCapabilities()
	if nil != err {
		return err
	}
	err = Unshare()
	if nil != err {
		return err
	}
	err = Mount()
	if nil != err {
		return err
	}
	err = ChRoot()
	if nil != err {
		return err
	}
	err = DropCapabilities()
	if nil != err {
		return err
	}

	return nil
} // DropPrivileges()

// `DropUID()` drops the group and user privileges of the process.
//
// It first checks if the process is running as root. If it is, it proceeds to
// drop the group and user privileges.
//
// Parameters:
// - aUID: The user ID to drop to. If it's less than 0, it defaults to 65534.
// - aGID: The group ID to drop to. If it's less than 0, it defaults to 65534.
//
// Returns:
// - `rErr`: The error if it encounters any issues while dropping the
// privileges.
func DropUID(aUID, aGID int) error {
	// Check if running as root
	if 0 < os.Geteuid() {
		// Already running as non-root
		return nil
	}
	var err error

	// Check the UID and GID to drop to
	if 0 > aUID {
		aUID = 65534 // unknown user
	}
	if 0 > aGID {
		aGID = 65534 // unknown group
	}

	// The `syscall.Setgroups()` function is responsible for setting the
	// list of groups for the current process. The function returns the
	// error that it encounters while setting the groups.
	var gids []int // empty group list
	if err = syscall.Setgroups(gids); (nil != err) && (syscall.EPERM != err) {
		apachelogger.Err("",
			fmt.Sprintf("Failed to clear Groups: %v", err))
		return se.Wrap(err, 3)
	}

	// The `syscall.Setgid()` function sets the group ID of the current
	// process. The function returns the error that it encounters while
	// setting the group ID.

	// First, drop group privileges
	if err = syscall.Setgid(aGID); (nil != err) && (syscall.EPERM != err) {
		apachelogger.Err("",
			fmt.Sprintf("Failed to set GID: %v", err))
		return se.Wrap(err, 3)
	}

	// The `syscall.Setuid()` function sets the real, effective, and saved
	// user IDs of the calling process to the value specified by uid. The
	// function returns the error that it encounters while
	// setting the user ID.

	// Then drop user privileges
	if err = syscall.Setuid(aUID); (nil != err) && (syscall.EPERM != err) {
		apachelogger.Err("",
			fmt.Sprintf("Failed to set UID: %v", err))
		return se.Wrap(err, 3)
	}

	apachelogger.Log("",
		fmt.Sprintf("Privileges dropped. Current UID: %d, GID: %d, last err: %v\n",
			os.Getuid(), os.Getgid(), err))

	return nil
} // DropUID()

// `Mount()` mounts a tmpfs filesystem at /tmp with the `MS_RDONLY` flag.
// The initial size of the tmpfs filesystem id set to 4 kilobytes,
// and the permissions of the tmpfs filesystem are set to read and write
// for the owner, and read-only.
//
// Returns:
// - `error`: If the `Mount()` system call returns an error, then it is
// logged and returned.
func Mount() error {
	var err error

	// The `syscall.Mount()` function is used to mount a new filesystem at
	// a specified directory. In this case, it mounts a tmpfs filesystem
	// at `/tmp` with the `MS_RDONLY` flag, which means that the tmpfs
	// filesystem is read-only. The "size=4k,mode=100" argument is a
	// comma-separated list of options for the tmpfs filesystem. The
	// `size=4k` option sets the initial size of the tmpfs filesystem to
	// 4 kilobytes, and the `mode=100` option sets the permissions of the
	// tmpfs filesystem to read and write for the owner, and read-only
	// for others.
	if err = syscall.Mount("tmpfs", "/tmp", "tmpfs",
		syscall.MS_RDONLY, "size=4k,mode=100"); nil != err {
		apachelogger.Err("",
			fmt.Sprintf("Failed mount(/tmp): %v", err))
		return se.Wrap(err, 4)
	}

	// The `syscall.Chdir()` function is used to change the current working
	// directory of the process to the specified directory. In this case,
	// it changes the current working directory to /tmp.
	if err = syscall.Chdir("/tmp"); nil != err {
		apachelogger.Err("",
			fmt.Sprintf("Failed chdir(/tmp): %v", err))
	}

	return nil
} // Mount()

// `Unshare()` unshares the certain resources and isolates the process
// from the parent process.
//
// Returns:
// - `error`: If the `Unshare()` system call returns an error, then it is
// logged and returned.
func Unshare() error {
	var err error
	flag := syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET | syscall.CLONE_NEWNS | syscall.CLONE_NEWIPC | syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_SYSVSEM

	// The `Unshare()` function is a system call that allows a process to
	// create a new namespace and isolate itself from the parent process.
	// The flag variable is a bitmask that specifies which resources to
	// unshare. In this case, the process wants to unshare the following
	// resources:
	//
	// - CLONE_NEWUSER: Create a new user namespace.
	// - CLONE_NEWNET: Create a new network namespace.
	// - CLONE_NEWNS: Create a new mount namespace.
	// - CLONE_NEWIPC: Create a new IPC namespace.
	// - CLONE_NEWPID: Create a new PID namespace.
	// - CLONE_NEWUTS: Create a new UTS namespace.
	// - CLONE_SYSVSEM: Create a new System V IPC namespace.
	//
	// If the `Unshare` system call returns an error (i.e., if err is not
	// equal to nil), then the function logs the error and returns it.

	if err = syscall.Unshare(flag); nil != err {
		apachelogger.Err("",
			fmt.Sprintf("Failed to Unshare: %v", err))
		return se.Wrap(err, 3)
	}

	return nil
} // Unshare()

func runTests() int {
	var (
		err  error
		sock int
	)
	fmt.Println("Tests:")
	fmt.Printf("… getuid(): %d\n", syscall.Getuid())
	_, err = os.OpenFile("/blah", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	fmt.Printf("… Creating file should fail: %s\n", err)
	_, err = os.Open("/blah")
	fmt.Printf("… Open any file: %s\n", err)

	fmt.Print("… Sending UDP packet should fail: ")
	sock, err = syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if nil != err {
		fmt.Println("socket():", err)
		return 1
	}

	sa := &syscall.SockaddrInet4{Port: 12345, Addr: [4]byte{127, 0, 0, 1}}
	data := []byte("hello")
	err = syscall.Sendto(sock, data, 0, sa)
	if nil != err {
		fmt.Println(err)
	}

	fmt.Println("OK")
	return 0
} // runTests()

/*
func main2() {
	if err := dropPrivs(); nil != err {
		os.Exit(1)
	}
	rc := runTests()
	time.Sleep(1800 * time.Second) // To allow inspecting /prod/<pid> stuff.
	os.Exit(rc)
}
*/

/* _EoF_ */
