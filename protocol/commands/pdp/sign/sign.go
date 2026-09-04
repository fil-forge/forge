//go:build !codegen

package sign

import (
	"github.com/fil-forge/ucantone/binding"
	"github.com/fil-forge/ucantone/ucan/command"
)

// Every /pdp/sign/* operation returns the same shape; these per-operation
// labels exist so call sites can keep the operation name in the type.
type (
	DataSetCreateOK        = AuthSignature
	DataSetDeleteOK        = AuthSignature
	PiecesAddOK            = AuthSignature
	PiecesRemoveScheduleOK = AuthSignature
)

var (
	DataSetCreate        = binding.Bind[*DataSetCreateArguments, *DataSetCreateOK](command.MustParse("/pdp/sign/dataset/create"))
	DataSetDelete        = binding.Bind[*DataSetDeleteArguments, *DataSetDeleteOK](command.MustParse("/pdp/sign/dataset/delete"))
	PiecesAdd            = binding.Bind[*PiecesAddArguments, *PiecesAddOK](command.MustParse("/pdp/sign/pieces/add"))
	PiecesRemoveSchedule = binding.Bind[*PiecesRemoveScheduleArguments, *PiecesRemoveScheduleOK](command.MustParse("/pdp/sign/pieces/remove/schedule"))
)
