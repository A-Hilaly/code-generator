package code

import (
	"fmt"
	"strings"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate/multiversion"
	"github.com/aws-controllers-k8s/code-generator/pkg/model"
	"github.com/aws-controllers-k8s/code-generator/pkg/names"
	awssdkmodel "github.com/aws/aws-sdk-go/private/model/api"
)

func ConvertTo(
	src, dst *model.CRD,
	hubImportAlias string,
	srcVarName string,
	dstRawVarName string,
	indentLevel int,
) string {
	out := "\n"
	indent := strings.Repeat("\t", indentLevel)

	// dst := dstRaw.(*v1.Repository)
	copyFromVar := "dst"
	copyFromVarRaw := "dstRaw"
	if srcVarName == "dst" {
		// src := srcRaw.(*v1.Repository)
		copyFromVar = "src"
		copyFromVarRaw = "srcRaw"
	}

	// dst := dstRaw.(*v1.Repository) || src := srcRaw.(*v1.Repository)
	out += fmt.Sprintf(
		"%s%s := %s.(*%s.%s)\n",
		indent,
		copyFromVar,
		copyFromVarRaw,
		hubImportAlias,
		dst.Names.Camel,
	)

	deltas, err := multiversion.ComputeCRDFieldsDeltas(src, dst)
	if err != nil {
		return err.Error()
	}

	for _, delta := range deltas {
		fmt.Println(delta.From.Names.Camel)
		from := delta.From
		to := delta.To

		varFrom := "src.Spec"
		varTo := "dst.Spec"

		switch delta.ChangeType {
		case multiversion.ChangeTypeIntact:
			out += convertIntactField(
				from,
				to,
				hubImportAlias,
				varFrom,
				varTo,
				indentLevel,
			)
		case multiversion.ChangeTypeRenamed:
		case multiversion.ChangeTypeAdded:
		case multiversion.ChangeTypeRemoved:
		case multiversion.ChangeTypeShapeChanged, multiversion.ChangeTypeShapeChangedToSecret:
			panic("not implemented")
		case multiversion.ChangeTypeUnknown:
			panic("should never happen")
		default:
		}
	}

	// copy metadata

	// copy status

	out += fmt.Sprintf("%sreturn nil", indent)
	return out
}

func convertIntactField(
	from, to *model.Field,
	hubImportAlias string,
	varFrom string,
	varTo string,
	indentLevel int,
) string {
	// we don't care about the 'to' variable - It's an intact change.

	switch from.ShapeRef.Shape.Type {
	case "structure":
		return copyStruct(
			from,
			hubImportAlias,
			varFrom,
			varTo,
			indentLevel,
		)
	case "list":
		return "//unsupported: list\n"
	default:
		return copyScalar(
			from.ShapeRef.Shape,
			hubImportAlias,
			varFrom,
			varTo,
			indentLevel,
		)
	}
}

func copyScalar(
	shape *awssdkmodel.Shape,
	hubImportAlias string,
	varFrom string,
	varTo string,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)

	fieldName := names.New(shape.ShapeName)

	switch shape.Type {
	case "boolean", "string", "character", "byte", "short", "integer", "long", "float", "double", "timestamp":
		out += fmt.Sprintf(
			"%s%s.%s = %s.%s\n",
			indent, varTo, fieldName.Camel, varFrom, fieldName.Camel,
		)
	default:
		panic("Unsupported shape type: " + shape.Type)
	}

	return out
}

func copyStruct(
	field *model.Field,
	hubImportAlias string,
	varFrom string,
	varTo string,
	//fieldPath string,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)

	//TODO(a-hilaly): use ackcompare.HasNilDifference
	out += fmt.Sprintf(
		"%sif %s.%s != nil {\n",
		indent,
		varFrom,
		field.Names.Camel,
	)
	shape := field.ShapeRef.Shape
	for _, memberName := range shape.MemberNames() {
		memberShapeRef := shape.MemberRefs[memberName]
		memberShape := memberShapeRef.Shape
		switch memberShape.Type {
		case "structure":
			out += "//nostruct\n"
		default:
			out += copyScalar(
				memberShape,
				hubImportAlias,
				varFrom+"."+shape.ShapeName,
				varTo+"."+shape.ShapeName,
				indentLevel+1,
			)
		}
	}
	out += fmt.Sprintf(
		"%s}\n", indent,
	)
	return out
}
