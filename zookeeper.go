/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main // import "github.com/mjolnir42/zkmutex"

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/samuel/go-zookeeper/zk"
)

func connect(cstr string) (*zk.Conn, string) {
	var servers, chroot string
	sr := strings.SplitN(cstr, `/`, 2)

	switch len(sr) {
	case 0:
		assertOK(fmt.Errorf(`Empty zk ensemble!`))
	case 1:
		servers = sr[0]
		chroot = `/`
	case 2:
		servers = sr[0]
		chroot = `/` + sr[1]
	}
	zks := strings.Split(servers, `,`)
	conn, _, err := zk.Connect(zks, 6*time.Second)
	assertOK(err)

	return conn, chroot
}

func zkHier(conn *zk.Conn, hier string, existsOK bool) bool {
	hierParts := strings.Split(hier, `/`)

	for i := range hierParts {
		part := filepath.Join(append(
			[]string{`/`}, hierParts[0:i+1]...)...)

		// root always exists
		if part == `/` {
			continue
		}
		// create node
		if !zkCreatePath(conn, part, existsOK) {
			return false
		}
	}
	return true
}

func zkCreatePath(conn *zk.Conn, path string, existsOK bool) bool {
	createdPath, err := conn.Create(path, []byte{}, int32(0), zk.WorldACL(zk.PermAll))
	if err != zk.ErrNodeExists || !existsOK {
		if errorOK(err) {
			return false
		}
	}
	if createdPath != `` {
		logrus.Infof("Created zk node %s", createdPath)
	}
	return true
}

func zkLeaderLock(conn *zk.Conn) (chan struct{}, chan struct{}) {
	leaderChannel := make(chan struct{})
	errorChannel := make(chan struct{})
	go func() {
		hostname, err := os.Hostname()
		if errorOK(err) {
			close(errorChannel)
		}

		lockPath := filepath.Join(runLock, `zkmutex-`)
		ballot, err := conn.Create(lockPath, []byte(hostname), int32(
			zk.FlagEphemeral|zk.FlagSequence), zk.WorldACL(zk.PermAll))
		if errorOK(err) {
			close(errorChannel)
		}

		// strip path from leader election ballot
		_, ballot = filepath.Split(ballot)
		logrus.Infof("Running leader election with ballot %s", ballot)

		// get runLock children
		children, _, event, err := conn.ChildrenW(runLock)
		if errorOK(err) {
			close(errorChannel)
		}
		sort.Strings(children)
		if children[0] == ballot {
			close(leaderChannel)
			return
		}
		logrus.Infof("Ballot %s won the leader election", children[0])

	eventrecv:
		for {
			ev := <-event
			switch ev.Type {
			case zk.EventNodeChildrenChanged:
				children, _, event, err = conn.ChildrenW(runLock)
				if errorOK(err) {
					close(errorChannel)
				}
				sort.Strings(children)
				if children[0] == ballot {
					close(leaderChannel)
					break eventrecv
				}
				logrus.Infof("Ballot %s won the leader election", children[0])
			}
		}
	}()
	return leaderChannel, errorChannel
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
