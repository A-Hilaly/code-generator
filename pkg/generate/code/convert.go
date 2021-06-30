package code

import (
	"fmt"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate/multiversion"
	"github.com/aws-controllers-k8s/code-generator/pkg/model"
)

func ConvertTo(
	src, dst *model.CRD,
	firstResVarName string,
	secondResVarName string,
	indentLevel int,
) string {
	deltas, err := multiversion.ComputeCRDFieldsDeltas(src, dst)
	if err != nil {
		return err.Error()
	}
	fmt.Println("----\ndeltas:")
	for _, delta := range deltas {
		fmt.Println("changetype:", delta.ChangeType)
	}
	fmt.Println("----")
	return "YEY" + src.Names.Camel + dst.Names.Camel
}
