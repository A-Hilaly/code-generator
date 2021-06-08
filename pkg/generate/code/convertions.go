package code

import (
	"github.com/aws-controllers-k8s/code-generator/pkg/model"
)

func ConvertTo(
	src, dst *model.CRD,
	firstResVarName string,
	secondResVarName string,
	indentLevel int,
) string {
	return "YEY" + src.Names.Camel + dst.Names.Camel
}
