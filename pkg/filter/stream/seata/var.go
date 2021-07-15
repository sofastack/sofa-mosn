package seata

import (
	"mosn.io/mosn/pkg/variable"
)

func init () {
	variable.Register(variable.NewStringVariable(XID, nil, nil, variable.DefaultStringSetter, 0))
	variable.Register(variable.NewStringVariable(BranchID, nil, nil, variable.DefaultStringSetter, 0))
}

