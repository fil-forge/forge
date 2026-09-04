//go:build !codegen

package shard

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

var List = binding.Bind[*ListArguments, *ListOK](command.MustParse("/upload/shard/list"))
