package ast

import (
	"testing"
)

func TestOffsetOfExpr(t *testing.T) {
	nodes := map[string]Node{
		`0x7fa855aab838 <col:63, col:95> 'unsigned long'`: &OffsetOfExpr{
			Address:  "0x7fa855aab838",
			Position: "col:63, col:95",
			Type:     "unsigned long",
			Children: []Node{},
		},
	}

	runNodeTests(t, nodes)
}
