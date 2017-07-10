/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main // import "github.com/mjolnir42/zkmutex"

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/client9/reopen"
	"github.com/dchest/blake2b"
	"github.com/droundy/goopt"
	"github.com/mjolnir42/erebos"
	"github.com/samuel/go-zookeeper/zk"
)

var conf *Config
var logInitialized bool
var lastNode, commandNode, runLock, zkmutexVersion string
var command []string

func init() {
	// Discard logspam from Zookeeper library
	erebos.DisableZKLogger()

	// set standard logger options
	erebos.SetLogrusOptions()

	// set goopt information
	goopt.Version = zkmutexVersion
	goopt.Suite = `zkMutex`
	goopt.Summary = `Command execution under a distributed lock`
	goopt.Author = `Jörg Pernfuß`
	goopt.Description = func() string {
		return "zkMutex can be used to coordinate the execution of a" +
			" command between multiple hosts.\n\nIt enforces that the" +
			" command only runs once between all hosts of a" +
			" syncgroup at any given time."
	}
}

func main() {
	os.Exit(run())
}

func run() int {
	// parse command line flags
	cliConfPath := goopt.String([]string{`-c`, `--config`},
		`/etc/zkmutex/zkmutex.conf`, `Configuration file`)
	goopt.Parse(nil)

	// read runtime configuration
	conf = &Config{}
	if err := conf.FromFile(*cliConfPath); err != nil {
		assertOK(fmt.Errorf("Could not open configuration: %s", err))
	}

	// validate we can fork to the requested user
	validUser()
	validSyncGroup()

	// setup logfile
	if lfh, err := reopen.NewFileWriter(conf.LogFile); err != nil {
		assertOK(fmt.Errorf("Unable to open logfile: %s", err))
	} else {
		logrus.SetOutput(lfh)
		logInitialized = true
	}
	logrus.Infoln(`Starting zkmutex`)

	conn, chroot := connect(conf.Ensemble)
	defer conn.Close()
	logrus.Infoln(`Configured zookeeper chroot:`, chroot)

	// ensure fixed node hierarchy exists
	if !zkHier(conn, filepath.Join(chroot, `zkmutex`), true) {
		return 1
	}

	// ensure required nodes exist
	zkMutexPath := filepath.Join(chroot, `zkmutex`, conf.SyncGroup)
	if !zkCreatePath(conn, zkMutexPath, true) {
		return 1
	}

	// get the command to run and calculate its hash
	for i := range os.Args {
		if os.Args[i] == `--` {
			command = os.Args[i+1:]
			break
		}
	}
	if len(command) == 0 {
		// XXX error message
		return 1
	}

	blake := blake2b.New512()
	for i := range command {
		blake.Write([]byte(command[i]))
	}
	cmdHash := hex.EncodeToString(blake.Sum(nil))

	zkMutexPath = filepath.Join(zkMutexPath, cmdHash)
	if !zkCreatePath(conn, zkMutexPath, true) {
		return 1
	}

	lastNode = filepath.Join(zkMutexPath, `last`)
	if !zkCreatePath(conn, lastNode, true) {
		return 1
	}

	commandNode = filepath.Join(zkMutexPath, `command`)
	if !zkCreatePath(conn, commandNode, true) {
		return 1
	}

	runLock = filepath.Join(zkMutexPath, `runlock`)
	if !zkCreatePath(conn, runLock, true) {
		return 1
	}

	leaderChan, errChan := zkLeaderLock(conn)

	block := make(chan error)
	select {
	case <-errChan:
		return 1
	case <-leaderChan:
		go leader(conn, block)
	}
	if errorOK(<-block) {
		return 1
	}
	logrus.Infof("Shutting down")
	return 0
}

func leader(conn *zk.Conn, block chan error) {
	logrus.Infoln("Leader election has been won")

	var err error
	var stat *zk.Stat

	cmd := exec.Command(command[0], command[1:]...)
	logrus.Infoln("Running command")

	if conf.User != `` {
		user, uerr := user.Lookup(conf.User)
		if sendError(uerr, block) {
			return
		}
		uid, uerr := strconv.Atoi(user.Uid)
		if sendError(uerr, block) {
			return
		}
		gid, uerr := strconv.Atoi(user.Gid)
		if sendError(uerr, block) {
			return
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{
				Uid:    uint32(uid),
				Gid:    uint32(gid),
				Groups: []uint32{},
			},
		}
	}
	err = cmd.Run()
	if sendError(err, block) {
		return
	}

	// write run information into Zookeeper
	_, stat, err = conn.Get(lastNode)
	if sendError(err, block) {
		return
	}
	version := stat.Version
	nowTime := time.Now().UTC().Format(time.RFC3339Nano)
	hostname, err := os.Hostname()
	if sendError(err, block) {
		return
	}
	runinfo := nowTime + ` ` + hostname
	_, err = conn.Set(lastNode, []byte(runinfo), version)
	if sendError(err, block) {
		return
	}

	// save an unhashed version of the command
	_, stat, err = conn.Get(commandNode)
	if sendError(err, block) {
		return
	}
	version = stat.Version
	flatCmd := strings.Join(command, ` `)
	_, err = conn.Set(commandNode, []byte(flatCmd), version)
	if sendError(err, block) {
		return
	}

	close(block)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
