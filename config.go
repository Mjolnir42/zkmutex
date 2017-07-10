/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	ucl "github.com/nahanni/go-ucl"
)

// Config holds the runtime configuration which is expected to be
// read from a UCL formatted file
type Config struct {
	Ensemble  string `json:"ensemble"`
	SyncGroup string `json:"sync.group"`
	LogFile   string `json:"log.file"`
	User      string `json:"run.as.user"`
}

// FromFile sets Config c based on the file contents
func (c *Config) FromFile(fname string) error {
	var (
		file, uclJSON []byte
		err           error
		fileBytes     *bytes.Buffer
		parser        *ucl.Parser
		uclData       map[string]interface{}
	)
	if fname, err = filepath.Abs(fname); err != nil {
		return err
	}
	if fname, err = filepath.EvalSymlinks(fname); err != nil {
		return err
	}
	if file, err = ioutil.ReadFile(fname); err != nil {
		return err
	}

	fileBytes = bytes.NewBuffer(file)
	parser = ucl.NewParser(fileBytes)
	if uclData, err = parser.Ucl(); err != nil {
		return err
	}

	if uclJSON, err = json.Marshal(uclData); err != nil {
		return err
	}
	return json.Unmarshal(uclJSON, &c)
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
