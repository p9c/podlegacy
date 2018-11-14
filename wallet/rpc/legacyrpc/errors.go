// Copyright (c) 2013-2015 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package legacyrpc

import (
	"errors"

	"github.com/parallelcointeam/pod/JSON"
)

// TODO(jrick): There are several error paths which 'replace' various errors
// with a more appropiate error from the btcjson package.  Create a map of
// these replacements so they can be handled once after an RPC handler has
// returned and before the error is marshaled.

// Error types to simplify the reporting of specific categories of
// errors, and their *JSON.RPCError creation.
type (
	// DeserializationError describes a failed deserializaion due to bad
	// user input.  It corresponds to JSON.ErrRPCDeserialization.
	DeserializationError struct {
		error
	}

	// InvalidParameterError describes an invalid parameter passed by
	// the user.  It corresponds to JSON.ErrRPCInvalidParameter.
	InvalidParameterError struct {
		error
	}

	// ParseError describes a failed parse due to bad user input.  It
	// corresponds to JSON.ErrRPCParse.
	ParseError struct {
		error
	}
)

// Errors variables that are defined once here to avoid duplication below.
var (
	ErrNeedPositiveAmount = InvalidParameterError{
		errors.New("amount must be positive"),
	}

	ErrNeedPositiveMinconf = InvalidParameterError{
		errors.New("minconf must be positive"),
	}

	ErrAddressNotInWallet = JSON.RPCError{
		Code:    JSON.ErrRPCWallet,
		Message: "address not found in wallet",
	}

	ErrAccountNameNotFound = JSON.RPCError{
		Code:    JSON.ErrRPCWalletInvalidAccountName,
		Message: "account name not found",
	}

	ErrUnloadedWallet = JSON.RPCError{
		Code:    JSON.ErrRPCWallet,
		Message: "Request requires a wallet but wallet has not loaded yet",
	}

	ErrWalletUnlockNeeded = JSON.RPCError{
		Code:    JSON.ErrRPCWalletUnlockNeeded,
		Message: "Enter the wallet passphrase with walletpassphrase first",
	}

	ErrNotImportedAccount = JSON.RPCError{
		Code:    JSON.ErrRPCWallet,
		Message: "imported addresses must belong to the imported account",
	}

	ErrNoTransactionInfo = JSON.RPCError{
		Code:    JSON.ErrRPCNoTxInfo,
		Message: "No information for transaction",
	}

	ErrReservedAccountName = JSON.RPCError{
		Code:    JSON.ErrRPCInvalidParameter,
		Message: "Account name is reserved by RPC server",
	}
)
