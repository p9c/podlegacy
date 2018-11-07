// Copyright (c) 2014 The btcsuite developers
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

// TestWalletSvrCmds tests all of the wallet server commands marshal and
// unmarshal into valid results include handling of optional fields being
// omitted in the marshalled command, while optional fields with defaults have
// the default assigned on unmarshalled commands.
func TestWalletSvrCmds(t *testing.T) {
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
			name: "addmultisigaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("addmultisigaddress", 2, []string{"031234", "035678"})
			},
			staticCmd: func() interface{} {
				keys := []string{"031234", "035678"}
				return JSON.NewAddMultisigAddressCmd(2, keys, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"addmultisigaddress","params":[2,["031234","035678"]],"id":1}`,
			unmarshalled: &JSON.AddMultisigAddressCmd{
				NRequired: 2,
				Keys:      []string{"031234", "035678"},
				Account:   nil,
			},
		},
		{
			name: "addmultisigaddress optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("addmultisigaddress", 2, []string{"031234", "035678"}, "test")
			},
			staticCmd: func() interface{} {
				keys := []string{"031234", "035678"}
				return JSON.NewAddMultisigAddressCmd(2, keys, JSON.String("test"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"addmultisigaddress","params":[2,["031234","035678"],"test"],"id":1}`,
			unmarshalled: &JSON.AddMultisigAddressCmd{
				NRequired: 2,
				Keys:      []string{"031234", "035678"},
				Account:   JSON.String("test"),
			},
		},
		{
			name: "addwitnessaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("addwitnessaddress", "1address")
			},
			staticCmd: func() interface{} {
				return JSON.NewAddWitnessAddressCmd("1address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"addwitnessaddress","params":["1address"],"id":1}`,
			unmarshalled: &JSON.AddWitnessAddressCmd{
				Address: "1address",
			},
		},
		{
			name: "createmultisig",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("createmultisig", 2, []string{"031234", "035678"})
			},
			staticCmd: func() interface{} {
				keys := []string{"031234", "035678"}
				return JSON.NewCreateMultisigCmd(2, keys)
			},
			marshalled: `{"jsonrpc":"1.0","method":"createmultisig","params":[2,["031234","035678"]],"id":1}`,
			unmarshalled: &JSON.CreateMultisigCmd{
				NRequired: 2,
				Keys:      []string{"031234", "035678"},
			},
		},
		{
			name: "dumpprivkey",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("dumpprivkey", "1Address")
			},
			staticCmd: func() interface{} {
				return JSON.NewDumpPrivKeyCmd("1Address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"dumpprivkey","params":["1Address"],"id":1}`,
			unmarshalled: &JSON.DumpPrivKeyCmd{
				Address: "1Address",
			},
		},
		{
			name: "encryptwallet",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("encryptwallet", "pass")
			},
			staticCmd: func() interface{} {
				return JSON.NewEncryptWalletCmd("pass")
			},
			marshalled: `{"jsonrpc":"1.0","method":"encryptwallet","params":["pass"],"id":1}`,
			unmarshalled: &JSON.EncryptWalletCmd{
				Passphrase: "pass",
			},
		},
		{
			name: "estimatefee",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("estimatefee", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewEstimateFeeCmd(6)
			},
			marshalled: `{"jsonrpc":"1.0","method":"estimatefee","params":[6],"id":1}`,
			unmarshalled: &JSON.EstimateFeeCmd{
				NumBlocks: 6,
			},
		},
		{
			name: "estimatepriority",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("estimatepriority", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewEstimatePriorityCmd(6)
			},
			marshalled: `{"jsonrpc":"1.0","method":"estimatepriority","params":[6],"id":1}`,
			unmarshalled: &JSON.EstimatePriorityCmd{
				NumBlocks: 6,
			},
		},
		{
			name: "getaccount",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getaccount", "1Address")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetAccountCmd("1Address")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaccount","params":["1Address"],"id":1}`,
			unmarshalled: &JSON.GetAccountCmd{
				Address: "1Address",
			},
		},
		{
			name: "getaccountaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getaccountaddress", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetAccountAddressCmd("acct")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaccountaddress","params":["acct"],"id":1}`,
			unmarshalled: &JSON.GetAccountAddressCmd{
				Account: "acct",
			},
		},
		{
			name: "getaddressesbyaccount",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getaddressesbyaccount", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetAddressesByAccountCmd("acct")
			},
			marshalled: `{"jsonrpc":"1.0","method":"getaddressesbyaccount","params":["acct"],"id":1}`,
			unmarshalled: &JSON.GetAddressesByAccountCmd{
				Account: "acct",
			},
		},
		{
			name: "getbalance",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getbalance")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetBalanceCmd(nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getbalance","params":[],"id":1}`,
			unmarshalled: &JSON.GetBalanceCmd{
				Account: nil,
				MinConf: JSON.Int(1),
			},
		},
		{
			name: "getbalance optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getbalance", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetBalanceCmd(JSON.String("acct"), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getbalance","params":["acct"],"id":1}`,
			unmarshalled: &JSON.GetBalanceCmd{
				Account: JSON.String("acct"),
				MinConf: JSON.Int(1),
			},
		},
		{
			name: "getbalance optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getbalance", "acct", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewGetBalanceCmd(JSON.String("acct"), JSON.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getbalance","params":["acct",6],"id":1}`,
			unmarshalled: &JSON.GetBalanceCmd{
				Account: JSON.String("acct"),
				MinConf: JSON.Int(6),
			},
		},
		{
			name: "getnewaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getnewaddress")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetNewAddressCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnewaddress","params":[],"id":1}`,
			unmarshalled: &JSON.GetNewAddressCmd{
				Account: nil,
			},
		},
		{
			name: "getnewaddress optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getnewaddress", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetNewAddressCmd(JSON.String("acct"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getnewaddress","params":["acct"],"id":1}`,
			unmarshalled: &JSON.GetNewAddressCmd{
				Account: JSON.String("acct"),
			},
		},
		{
			name: "getrawchangeaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getrawchangeaddress")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetRawChangeAddressCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawchangeaddress","params":[],"id":1}`,
			unmarshalled: &JSON.GetRawChangeAddressCmd{
				Account: nil,
			},
		},
		{
			name: "getrawchangeaddress optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getrawchangeaddress", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetRawChangeAddressCmd(JSON.String("acct"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getrawchangeaddress","params":["acct"],"id":1}`,
			unmarshalled: &JSON.GetRawChangeAddressCmd{
				Account: JSON.String("acct"),
			},
		},
		{
			name: "getreceivedbyaccount",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getreceivedbyaccount", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetReceivedByAccountCmd("acct", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaccount","params":["acct"],"id":1}`,
			unmarshalled: &JSON.GetReceivedByAccountCmd{
				Account: "acct",
				MinConf: JSON.Int(1),
			},
		},
		{
			name: "getreceivedbyaccount optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getreceivedbyaccount", "acct", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewGetReceivedByAccountCmd("acct", JSON.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaccount","params":["acct",6],"id":1}`,
			unmarshalled: &JSON.GetReceivedByAccountCmd{
				Account: "acct",
				MinConf: JSON.Int(6),
			},
		},
		{
			name: "getreceivedbyaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getreceivedbyaddress", "1Address")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetReceivedByAddressCmd("1Address", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaddress","params":["1Address"],"id":1}`,
			unmarshalled: &JSON.GetReceivedByAddressCmd{
				Address: "1Address",
				MinConf: JSON.Int(1),
			},
		},
		{
			name: "getreceivedbyaddress optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getreceivedbyaddress", "1Address", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewGetReceivedByAddressCmd("1Address", JSON.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"getreceivedbyaddress","params":["1Address",6],"id":1}`,
			unmarshalled: &JSON.GetReceivedByAddressCmd{
				Address: "1Address",
				MinConf: JSON.Int(6),
			},
		},
		{
			name: "gettransaction",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("gettransaction", "123")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetTransactionCmd("123", nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettransaction","params":["123"],"id":1}`,
			unmarshalled: &JSON.GetTransactionCmd{
				Txid:             "123",
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "gettransaction optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("gettransaction", "123", true)
			},
			staticCmd: func() interface{} {
				return JSON.NewGetTransactionCmd("123", JSON.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"gettransaction","params":["123",true],"id":1}`,
			unmarshalled: &JSON.GetTransactionCmd{
				Txid:             "123",
				IncludeWatchOnly: JSON.Bool(true),
			},
		},
		{
			name: "getwalletinfo",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("getwalletinfo")
			},
			staticCmd: func() interface{} {
				return JSON.NewGetWalletInfoCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"getwalletinfo","params":[],"id":1}`,
			unmarshalled: &JSON.GetWalletInfoCmd{},
		},
		{
			name: "importprivkey",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("importprivkey", "abc")
			},
			staticCmd: func() interface{} {
				return JSON.NewImportPrivKeyCmd("abc", nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"importprivkey","params":["abc"],"id":1}`,
			unmarshalled: &JSON.ImportPrivKeyCmd{
				PrivKey: "abc",
				Label:   nil,
				Rescan:  JSON.Bool(true),
			},
		},
		{
			name: "importprivkey optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("importprivkey", "abc", "label")
			},
			staticCmd: func() interface{} {
				return JSON.NewImportPrivKeyCmd("abc", JSON.String("label"), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"importprivkey","params":["abc","label"],"id":1}`,
			unmarshalled: &JSON.ImportPrivKeyCmd{
				PrivKey: "abc",
				Label:   JSON.String("label"),
				Rescan:  JSON.Bool(true),
			},
		},
		{
			name: "importprivkey optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("importprivkey", "abc", "label", false)
			},
			staticCmd: func() interface{} {
				return JSON.NewImportPrivKeyCmd("abc", JSON.String("label"), JSON.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"importprivkey","params":["abc","label",false],"id":1}`,
			unmarshalled: &JSON.ImportPrivKeyCmd{
				PrivKey: "abc",
				Label:   JSON.String("label"),
				Rescan:  JSON.Bool(false),
			},
		},
		{
			name: "keypoolrefill",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("keypoolrefill")
			},
			staticCmd: func() interface{} {
				return JSON.NewKeyPoolRefillCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"keypoolrefill","params":[],"id":1}`,
			unmarshalled: &JSON.KeyPoolRefillCmd{
				NewSize: JSON.Uint(100),
			},
		},
		{
			name: "keypoolrefill optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("keypoolrefill", 200)
			},
			staticCmd: func() interface{} {
				return JSON.NewKeyPoolRefillCmd(JSON.Uint(200))
			},
			marshalled: `{"jsonrpc":"1.0","method":"keypoolrefill","params":[200],"id":1}`,
			unmarshalled: &JSON.KeyPoolRefillCmd{
				NewSize: JSON.Uint(200),
			},
		},
		{
			name: "listaccounts",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listaccounts")
			},
			staticCmd: func() interface{} {
				return JSON.NewListAccountsCmd(nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listaccounts","params":[],"id":1}`,
			unmarshalled: &JSON.ListAccountsCmd{
				MinConf: JSON.Int(1),
			},
		},
		{
			name: "listaccounts optional",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listaccounts", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewListAccountsCmd(JSON.Int(6))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listaccounts","params":[6],"id":1}`,
			unmarshalled: &JSON.ListAccountsCmd{
				MinConf: JSON.Int(6),
			},
		},
		{
			name: "listaddressgroupings",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listaddressgroupings")
			},
			staticCmd: func() interface{} {
				return JSON.NewListAddressGroupingsCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"listaddressgroupings","params":[],"id":1}`,
			unmarshalled: &JSON.ListAddressGroupingsCmd{},
		},
		{
			name: "listlockunspent",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listlockunspent")
			},
			staticCmd: func() interface{} {
				return JSON.NewListLockUnspentCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"listlockunspent","params":[],"id":1}`,
			unmarshalled: &JSON.ListLockUnspentCmd{},
		},
		{
			name: "listreceivedbyaccount",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaccount")
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAccountCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAccountCmd{
				MinConf:          JSON.Int(1),
				IncludeEmpty:     JSON.Bool(false),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaccount optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaccount", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAccountCmd(JSON.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[6],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAccountCmd{
				MinConf:          JSON.Int(6),
				IncludeEmpty:     JSON.Bool(false),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaccount optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaccount", 6, true)
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAccountCmd(JSON.Int(6), JSON.Bool(true), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[6,true],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAccountCmd{
				MinConf:          JSON.Int(6),
				IncludeEmpty:     JSON.Bool(true),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaccount optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaccount", 6, true, false)
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAccountCmd(JSON.Int(6), JSON.Bool(true), JSON.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaccount","params":[6,true,false],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAccountCmd{
				MinConf:          JSON.Int(6),
				IncludeEmpty:     JSON.Bool(true),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaddress")
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAddressCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAddressCmd{
				MinConf:          JSON.Int(1),
				IncludeEmpty:     JSON.Bool(false),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaddress", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAddressCmd(JSON.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[6],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAddressCmd{
				MinConf:          JSON.Int(6),
				IncludeEmpty:     JSON.Bool(false),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaddress", 6, true)
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAddressCmd(JSON.Int(6), JSON.Bool(true), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[6,true],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAddressCmd{
				MinConf:          JSON.Int(6),
				IncludeEmpty:     JSON.Bool(true),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listreceivedbyaddress optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listreceivedbyaddress", 6, true, false)
			},
			staticCmd: func() interface{} {
				return JSON.NewListReceivedByAddressCmd(JSON.Int(6), JSON.Bool(true), JSON.Bool(false))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listreceivedbyaddress","params":[6,true,false],"id":1}`,
			unmarshalled: &JSON.ListReceivedByAddressCmd{
				MinConf:          JSON.Int(6),
				IncludeEmpty:     JSON.Bool(true),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listsinceblock",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listsinceblock")
			},
			staticCmd: func() interface{} {
				return JSON.NewListSinceBlockCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":[],"id":1}`,
			unmarshalled: &JSON.ListSinceBlockCmd{
				BlockHash:           nil,
				TargetConfirmations: JSON.Int(1),
				IncludeWatchOnly:    JSON.Bool(false),
			},
		},
		{
			name: "listsinceblock optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listsinceblock", "123")
			},
			staticCmd: func() interface{} {
				return JSON.NewListSinceBlockCmd(JSON.String("123"), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":["123"],"id":1}`,
			unmarshalled: &JSON.ListSinceBlockCmd{
				BlockHash:           JSON.String("123"),
				TargetConfirmations: JSON.Int(1),
				IncludeWatchOnly:    JSON.Bool(false),
			},
		},
		{
			name: "listsinceblock optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listsinceblock", "123", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewListSinceBlockCmd(JSON.String("123"), JSON.Int(6), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":["123",6],"id":1}`,
			unmarshalled: &JSON.ListSinceBlockCmd{
				BlockHash:           JSON.String("123"),
				TargetConfirmations: JSON.Int(6),
				IncludeWatchOnly:    JSON.Bool(false),
			},
		},
		{
			name: "listsinceblock optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listsinceblock", "123", 6, true)
			},
			staticCmd: func() interface{} {
				return JSON.NewListSinceBlockCmd(JSON.String("123"), JSON.Int(6), JSON.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listsinceblock","params":["123",6,true],"id":1}`,
			unmarshalled: &JSON.ListSinceBlockCmd{
				BlockHash:           JSON.String("123"),
				TargetConfirmations: JSON.Int(6),
				IncludeWatchOnly:    JSON.Bool(true),
			},
		},
		{
			name: "listtransactions",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listtransactions")
			},
			staticCmd: func() interface{} {
				return JSON.NewListTransactionsCmd(nil, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":[],"id":1}`,
			unmarshalled: &JSON.ListTransactionsCmd{
				Account:          nil,
				Count:            JSON.Int(10),
				From:             JSON.Int(0),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listtransactions optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listtransactions", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewListTransactionsCmd(JSON.String("acct"), nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct"],"id":1}`,
			unmarshalled: &JSON.ListTransactionsCmd{
				Account:          JSON.String("acct"),
				Count:            JSON.Int(10),
				From:             JSON.Int(0),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listtransactions optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listtransactions", "acct", 20)
			},
			staticCmd: func() interface{} {
				return JSON.NewListTransactionsCmd(JSON.String("acct"), JSON.Int(20), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct",20],"id":1}`,
			unmarshalled: &JSON.ListTransactionsCmd{
				Account:          JSON.String("acct"),
				Count:            JSON.Int(20),
				From:             JSON.Int(0),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listtransactions optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listtransactions", "acct", 20, 1)
			},
			staticCmd: func() interface{} {
				return JSON.NewListTransactionsCmd(JSON.String("acct"), JSON.Int(20),
					JSON.Int(1), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct",20,1],"id":1}`,
			unmarshalled: &JSON.ListTransactionsCmd{
				Account:          JSON.String("acct"),
				Count:            JSON.Int(20),
				From:             JSON.Int(1),
				IncludeWatchOnly: JSON.Bool(false),
			},
		},
		{
			name: "listtransactions optional4",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listtransactions", "acct", 20, 1, true)
			},
			staticCmd: func() interface{} {
				return JSON.NewListTransactionsCmd(JSON.String("acct"), JSON.Int(20),
					JSON.Int(1), JSON.Bool(true))
			},
			marshalled: `{"jsonrpc":"1.0","method":"listtransactions","params":["acct",20,1,true],"id":1}`,
			unmarshalled: &JSON.ListTransactionsCmd{
				Account:          JSON.String("acct"),
				Count:            JSON.Int(20),
				From:             JSON.Int(1),
				IncludeWatchOnly: JSON.Bool(true),
			},
		},
		{
			name: "listunspent",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listunspent")
			},
			staticCmd: func() interface{} {
				return JSON.NewListUnspentCmd(nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[],"id":1}`,
			unmarshalled: &JSON.ListUnspentCmd{
				MinConf:   JSON.Int(1),
				MaxConf:   JSON.Int(9999999),
				Addresses: nil,
			},
		},
		{
			name: "listunspent optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listunspent", 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewListUnspentCmd(JSON.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[6],"id":1}`,
			unmarshalled: &JSON.ListUnspentCmd{
				MinConf:   JSON.Int(6),
				MaxConf:   JSON.Int(9999999),
				Addresses: nil,
			},
		},
		{
			name: "listunspent optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listunspent", 6, 100)
			},
			staticCmd: func() interface{} {
				return JSON.NewListUnspentCmd(JSON.Int(6), JSON.Int(100), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[6,100],"id":1}`,
			unmarshalled: &JSON.ListUnspentCmd{
				MinConf:   JSON.Int(6),
				MaxConf:   JSON.Int(100),
				Addresses: nil,
			},
		},
		{
			name: "listunspent optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("listunspent", 6, 100, []string{"1Address", "1Address2"})
			},
			staticCmd: func() interface{} {
				return JSON.NewListUnspentCmd(JSON.Int(6), JSON.Int(100),
					&[]string{"1Address", "1Address2"})
			},
			marshalled: `{"jsonrpc":"1.0","method":"listunspent","params":[6,100,["1Address","1Address2"]],"id":1}`,
			unmarshalled: &JSON.ListUnspentCmd{
				MinConf:   JSON.Int(6),
				MaxConf:   JSON.Int(100),
				Addresses: &[]string{"1Address", "1Address2"},
			},
		},
		{
			name: "lockunspent",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("lockunspent", true, `[{"txid":"123","vout":1}]`)
			},
			staticCmd: func() interface{} {
				txInputs := []JSON.TransactionInput{
					{Txid: "123", Vout: 1},
				}
				return JSON.NewLockUnspentCmd(true, txInputs)
			},
			marshalled: `{"jsonrpc":"1.0","method":"lockunspent","params":[true,[{"txid":"123","vout":1}]],"id":1}`,
			unmarshalled: &JSON.LockUnspentCmd{
				Unlock: true,
				Transactions: []JSON.TransactionInput{
					{Txid: "123", Vout: 1},
				},
			},
		},
		{
			name: "move",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("move", "from", "to", 0.5)
			},
			staticCmd: func() interface{} {
				return JSON.NewMoveCmd("from", "to", 0.5, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"move","params":["from","to",0.5],"id":1}`,
			unmarshalled: &JSON.MoveCmd{
				FromAccount: "from",
				ToAccount:   "to",
				Amount:      0.5,
				MinConf:     JSON.Int(1),
				Comment:     nil,
			},
		},
		{
			name: "move optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("move", "from", "to", 0.5, 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewMoveCmd("from", "to", 0.5, JSON.Int(6), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"move","params":["from","to",0.5,6],"id":1}`,
			unmarshalled: &JSON.MoveCmd{
				FromAccount: "from",
				ToAccount:   "to",
				Amount:      0.5,
				MinConf:     JSON.Int(6),
				Comment:     nil,
			},
		},
		{
			name: "move optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("move", "from", "to", 0.5, 6, "comment")
			},
			staticCmd: func() interface{} {
				return JSON.NewMoveCmd("from", "to", 0.5, JSON.Int(6), JSON.String("comment"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"move","params":["from","to",0.5,6,"comment"],"id":1}`,
			unmarshalled: &JSON.MoveCmd{
				FromAccount: "from",
				ToAccount:   "to",
				Amount:      0.5,
				MinConf:     JSON.Int(6),
				Comment:     JSON.String("comment"),
			},
		},
		{
			name: "sendfrom",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendfrom", "from", "1Address", 0.5)
			},
			staticCmd: func() interface{} {
				return JSON.NewSendFromCmd("from", "1Address", 0.5, nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5],"id":1}`,
			unmarshalled: &JSON.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     JSON.Int(1),
				Comment:     nil,
				CommentTo:   nil,
			},
		},
		{
			name: "sendfrom optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendfrom", "from", "1Address", 0.5, 6)
			},
			staticCmd: func() interface{} {
				return JSON.NewSendFromCmd("from", "1Address", 0.5, JSON.Int(6), nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5,6],"id":1}`,
			unmarshalled: &JSON.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     JSON.Int(6),
				Comment:     nil,
				CommentTo:   nil,
			},
		},
		{
			name: "sendfrom optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendfrom", "from", "1Address", 0.5, 6, "comment")
			},
			staticCmd: func() interface{} {
				return JSON.NewSendFromCmd("from", "1Address", 0.5, JSON.Int(6),
					JSON.String("comment"), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5,6,"comment"],"id":1}`,
			unmarshalled: &JSON.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     JSON.Int(6),
				Comment:     JSON.String("comment"),
				CommentTo:   nil,
			},
		},
		{
			name: "sendfrom optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendfrom", "from", "1Address", 0.5, 6, "comment", "commentto")
			},
			staticCmd: func() interface{} {
				return JSON.NewSendFromCmd("from", "1Address", 0.5, JSON.Int(6),
					JSON.String("comment"), JSON.String("commentto"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendfrom","params":["from","1Address",0.5,6,"comment","commentto"],"id":1}`,
			unmarshalled: &JSON.SendFromCmd{
				FromAccount: "from",
				ToAddress:   "1Address",
				Amount:      0.5,
				MinConf:     JSON.Int(6),
				Comment:     JSON.String("comment"),
				CommentTo:   JSON.String("commentto"),
			},
		},
		{
			name: "sendmany",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendmany", "from", `{"1Address":0.5}`)
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"1Address": 0.5}
				return JSON.NewSendManyCmd("from", amounts, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendmany","params":["from",{"1Address":0.5}],"id":1}`,
			unmarshalled: &JSON.SendManyCmd{
				FromAccount: "from",
				Amounts:     map[string]float64{"1Address": 0.5},
				MinConf:     JSON.Int(1),
				Comment:     nil,
			},
		},
		{
			name: "sendmany optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendmany", "from", `{"1Address":0.5}`, 6)
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"1Address": 0.5}
				return JSON.NewSendManyCmd("from", amounts, JSON.Int(6), nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendmany","params":["from",{"1Address":0.5},6],"id":1}`,
			unmarshalled: &JSON.SendManyCmd{
				FromAccount: "from",
				Amounts:     map[string]float64{"1Address": 0.5},
				MinConf:     JSON.Int(6),
				Comment:     nil,
			},
		},
		{
			name: "sendmany optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendmany", "from", `{"1Address":0.5}`, 6, "comment")
			},
			staticCmd: func() interface{} {
				amounts := map[string]float64{"1Address": 0.5}
				return JSON.NewSendManyCmd("from", amounts, JSON.Int(6), JSON.String("comment"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendmany","params":["from",{"1Address":0.5},6,"comment"],"id":1}`,
			unmarshalled: &JSON.SendManyCmd{
				FromAccount: "from",
				Amounts:     map[string]float64{"1Address": 0.5},
				MinConf:     JSON.Int(6),
				Comment:     JSON.String("comment"),
			},
		},
		{
			name: "sendtoaddress",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendtoaddress", "1Address", 0.5)
			},
			staticCmd: func() interface{} {
				return JSON.NewSendToAddressCmd("1Address", 0.5, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendtoaddress","params":["1Address",0.5],"id":1}`,
			unmarshalled: &JSON.SendToAddressCmd{
				Address:   "1Address",
				Amount:    0.5,
				Comment:   nil,
				CommentTo: nil,
			},
		},
		{
			name: "sendtoaddress optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("sendtoaddress", "1Address", 0.5, "comment", "commentto")
			},
			staticCmd: func() interface{} {
				return JSON.NewSendToAddressCmd("1Address", 0.5, JSON.String("comment"),
					JSON.String("commentto"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"sendtoaddress","params":["1Address",0.5,"comment","commentto"],"id":1}`,
			unmarshalled: &JSON.SendToAddressCmd{
				Address:   "1Address",
				Amount:    0.5,
				Comment:   JSON.String("comment"),
				CommentTo: JSON.String("commentto"),
			},
		},
		{
			name: "setaccount",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("setaccount", "1Address", "acct")
			},
			staticCmd: func() interface{} {
				return JSON.NewSetAccountCmd("1Address", "acct")
			},
			marshalled: `{"jsonrpc":"1.0","method":"setaccount","params":["1Address","acct"],"id":1}`,
			unmarshalled: &JSON.SetAccountCmd{
				Address: "1Address",
				Account: "acct",
			},
		},
		{
			name: "settxfee",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("settxfee", 0.0001)
			},
			staticCmd: func() interface{} {
				return JSON.NewSetTxFeeCmd(0.0001)
			},
			marshalled: `{"jsonrpc":"1.0","method":"settxfee","params":[0.0001],"id":1}`,
			unmarshalled: &JSON.SetTxFeeCmd{
				Amount: 0.0001,
			},
		},
		{
			name: "signmessage",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("signmessage", "1Address", "message")
			},
			staticCmd: func() interface{} {
				return JSON.NewSignMessageCmd("1Address", "message")
			},
			marshalled: `{"jsonrpc":"1.0","method":"signmessage","params":["1Address","message"],"id":1}`,
			unmarshalled: &JSON.SignMessageCmd{
				Address: "1Address",
				Message: "message",
			},
		},
		{
			name: "signrawtransaction",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("signrawtransaction", "001122")
			},
			staticCmd: func() interface{} {
				return JSON.NewSignRawTransactionCmd("001122", nil, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122"],"id":1}`,
			unmarshalled: &JSON.SignRawTransactionCmd{
				RawTx:    "001122",
				Inputs:   nil,
				PrivKeys: nil,
				Flags:    JSON.String("ALL"),
			},
		},
		{
			name: "signrawtransaction optional1",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("signrawtransaction", "001122", `[{"txid":"123","vout":1,"scriptPubKey":"00","redeemScript":"01"}]`)
			},
			staticCmd: func() interface{} {
				txInputs := []JSON.RawTxInput{
					{
						Txid:         "123",
						Vout:         1,
						ScriptPubKey: "00",
						RedeemScript: "01",
					},
				}

				return JSON.NewSignRawTransactionCmd("001122", &txInputs, nil, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122",[{"txid":"123","vout":1,"scriptPubKey":"00","redeemScript":"01"}]],"id":1}`,
			unmarshalled: &JSON.SignRawTransactionCmd{
				RawTx: "001122",
				Inputs: &[]JSON.RawTxInput{
					{
						Txid:         "123",
						Vout:         1,
						ScriptPubKey: "00",
						RedeemScript: "01",
					},
				},
				PrivKeys: nil,
				Flags:    JSON.String("ALL"),
			},
		},
		{
			name: "signrawtransaction optional2",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("signrawtransaction", "001122", `[]`, `["abc"]`)
			},
			staticCmd: func() interface{} {
				txInputs := []JSON.RawTxInput{}
				privKeys := []string{"abc"}
				return JSON.NewSignRawTransactionCmd("001122", &txInputs, &privKeys, nil)
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122",[],["abc"]],"id":1}`,
			unmarshalled: &JSON.SignRawTransactionCmd{
				RawTx:    "001122",
				Inputs:   &[]JSON.RawTxInput{},
				PrivKeys: &[]string{"abc"},
				Flags:    JSON.String("ALL"),
			},
		},
		{
			name: "signrawtransaction optional3",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("signrawtransaction", "001122", `[]`, `[]`, "ALL")
			},
			staticCmd: func() interface{} {
				txInputs := []JSON.RawTxInput{}
				privKeys := []string{}
				return JSON.NewSignRawTransactionCmd("001122", &txInputs, &privKeys,
					JSON.String("ALL"))
			},
			marshalled: `{"jsonrpc":"1.0","method":"signrawtransaction","params":["001122",[],[],"ALL"],"id":1}`,
			unmarshalled: &JSON.SignRawTransactionCmd{
				RawTx:    "001122",
				Inputs:   &[]JSON.RawTxInput{},
				PrivKeys: &[]string{},
				Flags:    JSON.String("ALL"),
			},
		},
		{
			name: "walletlock",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("walletlock")
			},
			staticCmd: func() interface{} {
				return JSON.NewWalletLockCmd()
			},
			marshalled:   `{"jsonrpc":"1.0","method":"walletlock","params":[],"id":1}`,
			unmarshalled: &JSON.WalletLockCmd{},
		},
		{
			name: "walletpassphrase",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("walletpassphrase", "pass", 60)
			},
			staticCmd: func() interface{} {
				return JSON.NewWalletPassphraseCmd("pass", 60)
			},
			marshalled: `{"jsonrpc":"1.0","method":"walletpassphrase","params":["pass",60],"id":1}`,
			unmarshalled: &JSON.WalletPassphraseCmd{
				Passphrase: "pass",
				Timeout:    60,
			},
		},
		{
			name: "walletpassphrasechange",
			newCmd: func() (interface{}, error) {
				return JSON.NewCmd("walletpassphrasechange", "old", "new")
			},
			staticCmd: func() interface{} {
				return JSON.NewWalletPassphraseChangeCmd("old", "new")
			},
			marshalled: `{"jsonrpc":"1.0","method":"walletpassphrasechange","params":["old","new"],"id":1}`,
			unmarshalled: &JSON.WalletPassphraseChangeCmd{
				OldPassphrase: "old",
				NewPassphrase: "new",
			},
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
