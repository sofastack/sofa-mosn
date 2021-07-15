package seata

import (
	"mosn.io/mosn/pkg/variable"
)

func init () {
	variable.RegisterVariable(variable.NewIndexedVariable(XID, nil, nil, variable.BasicSetter, 0))
	variable.RegisterVariable(variable.NewIndexedVariable(BranchID, nil, nil, variable.BasicSetter, 0))
}

