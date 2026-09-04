//go:build !codegen

package provider

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/errors"
	"github.com/fil-forge/ucantone/ucan/command"
)

var Add = binding.Bind[*AddArguments, *AddOK](command.MustParse("/provider/add"))

const (
	InvalidAccountErrorName     = "InvalidAccount"
	AccountPlanMissingErrorName = "AccountPlanMissing"
)

var ErrAccountPlanMissing = errors.New(AccountPlanMissingErrorName, "account does not have an active payment plan")
