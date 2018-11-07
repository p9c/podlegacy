// Copyright (c) 2014-2016 The btcsuite developers
// Copyright (c) 2015-2016 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package JSON_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/parallelcointeam/pod/JSON"
)

// TestBtcdExtCmds tests all of the btcd extended commands marshal and unmarshal
// into valid results include handling of optional fields being omitted in the
// marshalled command, while optional fields with defaults have the default
// assigned on unmarshalled commands.
func TestBtcdExtCmds(t *testing.T) {
	t.Parallel()

	testID := int(1)
	tests := []struct {
		name         string
		newCmd       func() (interface{}, error)
		staticCmd    func() interface{}
		marshalled   string
		unmarshalled interface{}
	}{
		{
			name: "debuglevel",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("debuglevel", "trace")
			},
			staticCmd: func() interface{} {
				return JSON.NewDebugLevelCmd("trace")
			},
			marshalled: `{"jsonrpc":"1.0","method":"debuglevel","params":["trace"],"id":1}`,
			unmarshalled: &JSON.DebugLevelCmd{
				LevelSpec: "trace",
			},
		},
		{
			name: "node",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("node", JSON.NRemove, "1.1.1.1")
			},
			staticCmd: func() interface{} {
				return JSON.NewNodeCmd("remove", "1.1.1.1", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"node","params":["remove","1.1.1.1"],"id":1}`,
			unmarshalled: &JSON.NodeCmd{
				SubCmd: JSON.NRemove,
				Target: "1.1.1.1",
			},
		},
		{
			name: "node",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("node", JSON.NDisconnect, "1.1.1.1")
			},
			staticCmd: func() interface{} {
				return JSON.NewNodeCmd("disconnect", "1.1.1.1", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"node","params":["disconnect","1.1.1.1"],"id":1}`,
			unmarshalled: &JSON.NodeCmd{
				SubCmd: JSON.NDisconnect,
				Target: "1.1.1.1",
			},
		},
		{
			name: "node",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("node", JSON.NConnect, "1.1.1.1", "perm")
			},
			staticCmd: func() interface{} {
				return JSON.NewNodeCmd("connect", "1.1.1.1", JSON.String("perm"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"node","params":["connect","1.1.1.1","perm"],"id":1}`,
			unmarshalled: &JSON.NodeCmd{
				SubCmd:        JSON.NConnect,
				Target:        "1.1.1.1",
				ConnectSubCmd: JSON.String("perm"),
			},
		},
		{
			name: "node",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("node", JSON.NConnect, "1.1.1.1", "temp")
			},
			staticCmd: func() interface{} {
				return JSON.NewNodeCmd("connect", "1.1.1.1", JSON.String("temp"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"node","params":["connect","1.1.1.1","temp"],"id":1}`,
			unmarshalled: &JSON.NodeCmd{
				SubCmd:        JSON.NConnect,
				Target:        "1.1.1.1",
				ConnectSubCmd: JSON.String("temp"),
			},
		},
		{
			name: "generate",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("generate", 1)
			},
			staticCmd: func() interface{} {
				return JSON.NewGenerateCmd(1)
			},
			marshalled: `{"jsonrpc":"1.0","method":"generate","params":[1],"id":1}`,
			unmarshalled: &JSON.GenerateCmd{
				NumBlocks: 1,
			},
		},
		{
			name: "getbestblock",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getbestblock")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetBestBlockCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getbestblock","params":[],"id":1}`,
			unmarshalled: &JSON.GetBestBlockCmd{},
		},
		{
			name: "getcurrentnet",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getcurrentnet")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetCurrentNetCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getcurrentnet","params":[],"id":1}`,
			unmarshalled: &JSON.GetCurrentNetCmd{},
		},
		{
			name: "getheaders",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getheaders", []string{}, "")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetHeadersCmd(
					[]string{},
					"",
				)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getheaders","params":[[],""],"id":1}`,
			unmarshalled: &JSON.GetHeadersCmd{
				BlockLocators: []string{},
				HashStop:      "",
			},
		},
		{
			name: "getheaders - with arguments",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getheaders", []string{"000000000000000001f1739002418e2f9a84c47a4fd2a0eb7a787a6b7dc12f16", "0000000000000000026f4b7f56eef057b32167eb5ad9ff62006f1807b7336d10"}, "000000000000000000ba33b33e1fad70b69e234fc24414dd47113bff38f523f7")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetHeadersCmd(
					[]string{
						"000000000000000001f1739002418e2f9a84c47a4fd2a0eb7a787a6b7dc12f16",
						"0000000000000000026f4b7f56eef057b32167eb5ad9ff62006f1807b7336d10",
					},
					"000000000000000000ba33b33e1fad70b69e234fc24414dd47113bff38f523f7",
				)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getheaders","params":[["000000000000000001f1739002418e2f9a84c47a4fd2a0eb7a787a6b7dc12f16","0000000000000000026f4b7f56eef057b32167eb5ad9ff62006f1807b7336d10"],"000000000000000000ba33b33e1fad70b69e234fc24414dd47113bff38f523f7"],"id":1}`,
			unmarshalled: &JSON.GetHeadersCmd{
				BlockLocators: []string{
					"000000000000000001f1739002418e2f9a84c47a4fd2a0eb7a787a6b7dc12f16",
					"0000000000000000026f4b7f56eef057b32167eb5ad9ff62006f1807b7336d10",
				},
				HashStop: "000000000000000000ba33b33e1fad70b69e234fc24414dd47113bff38f523f7",
			},
		},
		{
			name: "version",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("version")
			},
			staticCmd: func() interface{} {
				return JSON.NewVersionCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"version","params":[],"id":1}`,
			unmarshalled: &JSON.VersionCmd{},
		},
	}

	t.Logf("Running %d tests", len(tests))
	for i, test := range tests {
		// Marshal the command as created by the new static command
		// creation function.
		marshalled, err := JSON.MarshalCmd(testID, test.staticCmd())
		if err != nil {
			t.Errorf("MarshalCmd #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}

		if !bytes.Equal(marshalled, []byte(test.marshalled)) {
			t.Errorf("Test #%d (%s) unexpected marshalled data - "+
				"got %s, want %s", i, test.name, marshalled,
				test.marshalled)
			continue
		}

		// Ensure the command is created without error via the generic
		// new command creation function.
		cmd, err := test.newCmd()
		if err != nil {
			t.Errorf("Test #%d (%s) unexpected NewCmd error: %v ",
				i, test.name, err)
		}

		// Marshal the command as created by the generic new command
		// creation function.
		marshalled, err = JSON.MarshalCmd(testID, cmd)
		if err != nil {
			t.Errorf("MarshalCmd #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}

		if !bytes.Equal(marshalled, []byte(test.marshalled)) {
			t.Errorf("Test #%d (%s) unexpected marshalled data - "+
				"got %s, want %s", i, test.name, marshalled,
				test.marshalled)
			continue
		}

		var request JSON.Request
		if err := json.Unmarshal(marshalled, &request); err != nil {
			t.Errorf("Test #%d (%s) unexpected error while "+
				"unmarshalling JSON-RPC request: %v", i,
				test.name, err)
			continue
		}

		cmd, err = JSON.UnmarshalCmd(&request)
		if err != nil {
			t.Errorf("UnmarshalCmd #%d (%s) unexpected error: %v", i,
				test.name, err)
			continue
		}

		if !reflect.DeepEqual(cmd, test.unmarshalled) {
			t.Errorf("Test #%d (%s) unexpected unmarshalled command "+
				"- got %s, want %s", i, test.name,
				fmt.Sprintf("(%T) %+[1]v", cmd),
				fmt.Sprintf("(%T) %+[1]v\n", test.unmarshalled))
			continue
		}
	}
}
