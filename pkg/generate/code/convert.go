package code

import (
	"encoding/json"
	"fmt"
	"strconv"
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
	isCopyingFromHub := true // otherwise we are copying to a hub :)

	if srcVarName == "dst" {
		// src := srcRaw.(*v1.Repository)
		copyFromVar = "src"
		copyFromVarRaw = "srcRaw"
		// TODO(a-hilaly) explain why and how this impacts the code generation
		// for ConvertTo and ConvertFrom
		isCopyingFromHub = false
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
		panic("delta error" + err.Error())
	}

	out += fmt.Sprintf(
		"%sobjectMetadataCopy := src.ObjectMeta\n\n",
		indent,
	)

	for _, delta := range deltas {
		from := delta.From
		to := delta.To

		varFrom := "src.Spec"
		varTo := "dst.Spec"

		switch delta.ChangeType {
		case multiversion.ChangeTypeIntact:
			out += "\t// ChangeType: intact\n"
			out += copyField(
				from,
				to,
				hubImportAlias,
				varFrom,
				varTo,
				isCopyingFromHub,
				indentLevel,
			)
		case multiversion.ChangeTypeRenamed:
			out += "\t// ChangeType: renamed\n"
			out += copyField(
				from,
				to,
				hubImportAlias,
				varFrom,
				varTo,
				isCopyingFromHub,
				indentLevel,
			)
		case multiversion.ChangeTypeAdded:
			out += "\t// ChangeType: removed\n"

		case multiversion.ChangeTypeRemoved:
			fmt.Println("field added")
			out += "\t// ChangeType: added\n"
			out += copyRemovedField(
				from,
				hubImportAlias,
				varFrom,
				varTo,
				isCopyingFromHub,
				indentLevel,
			)
		case multiversion.ChangeTypeShapeChanged, multiversion.ChangeTypeShapeChangedToSecret:
			panic("not implemented")
		case multiversion.ChangeTypeUnknown:
			panic("should never happen")
		default:
		}
	}

	// copy metadata

	out += "\n\tdst.ObjectMeta = objectMetadataCopy\n"

	// copy status
	// TODO(a-hilaly) compute status diff and convert

	out += fmt.Sprintf("%sreturn nil", indent)
	return out
}

func swapVariables(a, b string) (string, string) {
	return b, a
}

func copyField(
	from, to *model.Field,
	hubImportAlias string,
	varFrom string,
	varTo string,
	isCopyingFromHub bool,
	indentLevel int,
) string {
	// if a field is not renamed, from and to have the same name
	// so the name doesn't impact much the code generation
	varFromPath := varFrom + "." + from.Names.Original
	varToPath := varTo + "." + to.Names.Original

	// however in case of renames we should correctly invert from/to field
	// paths. Only when we are convert to hub (not from hub).
	if !isCopyingFromHub {
		varFromPath = varFrom + "." + to.Names.Original
		varToPath = varTo + "." + from.Names.Original
	}

	switch from.ShapeRef.Shape.Type {
	case "structure":
		return copyStruct(
			from.ShapeRef.Shape,
			hubImportAlias,
			varFromPath,
			varToPath,
			isCopyingFromHub,
			indentLevel,
		)
	case "list":
		return copyList(
			from.ShapeRef.Shape,
			hubImportAlias,
			varFromPath,
			varToPath,
			isCopyingFromHub,
			indentLevel,
		)
	default:
		return copyScalar(
			from.ShapeRef.Shape,
			hubImportAlias,
			varFromPath,
			varToPath,
			indentLevel,
		)
	}
}

func copyRemovedField(
	from *model.Field,
	hubImportAlias string,
	varFrom string,
	varTo string,
	isCopyingFromHub bool,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)
	errVar := "err"
	annotationKeyVar := "annotationKey"
	annotationValueVar := "annotationValueVar"

	if !isCopyingFromHub {
		out += fmt.Sprintf(
			"%s%s, %s, %s := AnnotateFieldData(\"%s\", %s)\n",
			indent,
			annotationKeyVar,
			annotationValueVar,
			errVar,
			from.Names.Camel,
			varFrom+"."+from.Names.Camel,
		)
		out += fmt.Sprintf("%sif err != nil {\n", indent)
		out += fmt.Sprintf("%s\treturn err\n", indent)
		out += fmt.Sprintf("%s}\n", indent)
		out += fmt.Sprintf(
			"%sobjectMetadataCopy.Annotations[%s] = %s\n",
			indent,
			annotationKeyVar,
			annotationValueVar,
		)
	} else {
		out += fmt.Sprintf(
			"%s%s := DecodeFieldDataAnnotation(%s, %s)\n",
			indent,
			errVar,
			"\"conversions.ack.aws.dev/"+from.Names.Camel+"\"",
			varTo+"."+from.Names.Camel,
		)
		out += fmt.Sprintf("%sif err != nil {\n", indent)
		out += fmt.Sprintf("%s\treturn err\n", indent)
		out += fmt.Sprintf("%s}\n", indent)
	}

	return out
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

	switch shape.Type {
	case "boolean", "string", "character", "byte", "short",
		"integer", "long", "float", "double", "timestamp":
		out += fmt.Sprintf(
			"%s%s = %s\n",
			indent, varTo, varFrom,
		)
	default:
		panic("Unsupported shape type: " + shape.Type)
	}

	return out
}

func copyStruct(
	shape *awssdkmodel.Shape,
	hubImportAlias string,
	varFrom string,
	varTo string,
	isCopyingFromHub bool,
	//fieldPath string,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)

	//TODO(a-hilaly): use ackcompare.HasNilDifference
	out += fmt.Sprintf(
		"%sif %s != nil {\n",
		indent,
		varFrom,
	)

	// initialize a new copy struct
	structShapeName := names.New(shape.ShapeName)
	varStructCopy := structShapeName.CamelLower + "Copy"
	out += newShapeTypeInstance(
		shape,
		hubImportAlias,
		varStructCopy,
		varFrom,
		true,
		isCopyingFromHub,
		indentLevel+1,
	)

	for _, memberName := range shape.MemberNames() {
		memberShapeRef := shape.MemberRefs[memberName]
		memberShape := memberShapeRef.Shape
		memberNames := names.New(memberName)

		switch memberShape.Type {
		case "structure":
			out += "//nested struct\n"
		default:
			out += copyScalar(
				memberShape,
				hubImportAlias,
				varFrom+"."+memberNames.Camel,
				varStructCopy+"."+memberNames.Camel,
				indentLevel+1,
			)
		}
	}

	out += storeVariableIn(varStructCopy, varTo, indentLevel+1)

	// attach the copy struct to the dst variable
	out += fmt.Sprintf(
		"%s}\n", indent,
	)
	return out + "\n"
}

func copyList(
	shape *awssdkmodel.Shape,
	hubImportAlias string,
	varFrom string,
	varTo string,
	isCopyingFromHub bool,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)

	//TODO(a-hilaly): use ackcompare.HasNilDifference
	out += fmt.Sprintf(
		"%sif %s != nil {\n",
		indent,
		varFrom,
	)

	// initialize a new copy struct
	structShapeName := names.New(shape.ShapeName)
	varStructCopy := structShapeName.CamelLower + "Copy"
	out += newShapeTypeInstance(
		shape,
		hubImportAlias,
		varStructCopy,
		varFrom,
		false,
		isCopyingFromHub,
		indentLevel+1,
	)

	varIndex := "i"
	varElement := "element"
	out += fmt.Sprintf(
		"%s\tfor %s, %s := range %s {\n",
		indent,
		varIndex,
		varElement,
		varFrom,
	)

	out += fmt.Sprintf(
		"%s\t\t_ = %s // non-used value guard.\n",
		indent,
		varIndex,
	)

	memberShapeRef := shape.MemberRef
	memberShape := memberShapeRef.Shape

	varElementCopy := "element" + "Copy"
	out += newShapeTypeInstance(
		memberShape,
		hubImportAlias,
		varElementCopy,
		varElement,
		true,
		isCopyingFromHub,
		indentLevel+2,
	)

	switch shape.MemberRef.Shape.Type {
	case "structure":
		out += copyStruct(
			memberShape,
			hubImportAlias,
			varElement,
			varElementCopy,
			isCopyingFromHub,
			indentLevel+2,
		)
	case "":

	default:
	}

	out += fmt.Sprintf(
		"%s\t\t%s = append(%s, %s)\n",
		indent,
		varStructCopy,
		varStructCopy,
		varElementCopy,
	)

	/* 	switch shape.MemberRef.Shape.Type {
	   	case "boolean", "string", "character", "byte", "short",
	   		"integer", "long", "float", "double", "timestamp":
	   		out += "0"
	   	default:
	   		panic("copyList: error")
	   	}
	*/

	// closing for loop
	out += fmt.Sprintf(
		"%s\t}\n", indent,
	)

	out += storeVariableIn(varStructCopy, varTo, indentLevel+1)

	// attach the copy struct to the dst variable
	out += fmt.Sprintf(
		"%s}\n", indent,
	)

	return out + "\n"
}

func newShapeTypeInstance(
	shape *awssdkmodel.Shape,
	hubImportAlias string,
	allocationVarName string,
	fromVar string,
	isPointer bool,
	isCopyingFromHub bool,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)

	switch shape.Type {
	case "structure":
		goType := shape.GoTypeElem()
		if isCopyingFromHub {
			goType = hubImportAlias + "." + goType
		}
		if isPointer {
			goType = "&" + goType
		}
		out += fmt.Sprintf(
			"%s%s := %s{}\n",
			indent,
			allocationVarName,
			goType,
		)
	case "list":
		goType := shape.MemberRef.GoTypeElem()
		if isCopyingFromHub {
			goType = hubImportAlias + "." + goType
		}
		if isPointer {
			goType = "*" + goType
		}
		out += fmt.Sprintf(
			"%s%s := make([]*%s, 0, len(%s))\n",
			indent,
			allocationVarName,
			goType,
			fromVar,
		)
	default:
		panic("not support shape init")
	}

	return out
}

func storeVariableIn(
	from string,
	target string,
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)
	out += fmt.Sprintf(
		"%s%s = %s\n",
		indent,
		target,
		from,
	)
	return out
}

func isMadeOfBuiltinTypes(shape *awssdkmodel.Shape) bool {
	switch shape.Type {
	case "boolean", "string", "character", "byte", "short",
		"integer", "long", "float", "double", "timestamp":
		return true
	case "list":
		return isMadeOfBuiltinTypes(shape.MemberRef.Shape)
	case "map":
		return isMadeOfBuiltinTypes(shape.KeyRef.Shape) &&
			isMadeOfBuiltinTypes(shape.ValueRef.Shape)
	default:
		return false
	}
}

const (
	conversionAnnotationField = "ack.aws.dev/"
)

// returns annotationName, annotationValue and an error
// TODO(a-hilaly) care about annotation key size limit (maybe hash the keys? and store a hashmap as annotation value?)
// TODO(a-hilaly) move this utility to runtime
func annotateFieldData(fieldName string, data interface{}) (string, string, error) {
	annotationKey := conversionAnnotationField + fieldName
	annotationValue := ""
	switch data.(type) {
	case *string:
		annotationValue = "string=" + *data.(*string)
	case *int:
		annotationValue = "int=" + strconv.Itoa(*data.(*int))
	case *int8:
	case *int16:
	case *int32:
	case *int64:
	default:
		bytes, err := json.Marshal(data)
		if err != nil {
			return "", "", err
		}
		annotationValue = "json=" + string(bytes)
	}

	return annotationKey, annotationValue, nil
}

func decodeFieldDataAnnotation(annotationValue string, unmarshallTo interface{}) error {
	/* 	if !strings.HasPrefix(annotationKey, conversionAnnotationField) {
		return fmt.Errorf("not a conversion annotation")
	} */

	parts := strings.Split(annotationValue, "=")
	vType := parts[0]
	value := parts[1]
	switch vType {
	case "string":
		unmarshallTo = value
	case "int":
		unmarshallTo = value
	case "int8":
	case "int16":
	case "int32":
	case "int64":
	case "json":
		valueBytes := []byte(value)
		err := json.Unmarshal(valueBytes, unmarshallTo)
		if err != nil {
			return fmt.Errorf("unmarshalling value of type %s: %v", vType, err)
		}
	default:
		return fmt.Errorf("unsupported type")
	}

	return nil
}
