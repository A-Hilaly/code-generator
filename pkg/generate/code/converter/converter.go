// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Code generated by ack-generate. DO NOT EDIT.

package converter

import (
	"fmt"
	"strings"

	awssdkmodel "github.com/aws/aws-sdk-go/private/model/api"

	"github.com/aws-controllers-k8s/code-generator/pkg/model"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
	"github.com/aws-controllers-k8s/code-generator/pkg/model/multiversion"
	"github.com/aws-controllers-k8s/code-generator/pkg/names"
)

// New initialise and returns a new converter.
func New(
	hubImportAlias string,
	sdkPackageName string,
	isConvertingToHub bool,
	typeRenames map[string]string,
	builtinStatuses []string,
	indentLevel int,
) converter {
	return converter{
		hubImportAlias:    hubImportAlias,
		sdkPackageName:    sdkPackageName,
		isConvertingToHub: isConvertingToHub,
		typeRenames:       typeRenames,
		indentLevel:       indentLevel,
		builtinStatuses:   builtinStatuses,
	}
}

// converter is a private structure responsible of generating go code from
// multiversion.FieldDelta's
type converter struct {
	// The hub import alias. Generally the hub version.
	hubImportAlias string
	// The sdk import alias. Generally the service alias.
	sdkPackageName string
	// Whether we're generating `ConvertTo` or `ConvertFrom`
	isConvertingToHub bool
	// indentation level
	indentLevel int
	// go type renames
	typeRenames map[string]string
	// the source and destination variable names
	srcVar, dstVar string
	// builtin statuses
	builtinStatuses []string
}

// WithVariables overrides the srcVar and dstVar fields and returns a copy
// of converter
func (c converter) WithVariables(srcVar, dstVar string) converter {
	c.srcVar = srcVar
	c.dstVar = dstVar
	return c
}

// withIndentLevel overrides the indentLevel field and returns a copy of
// converter
func (c converter) withIndentLevel(indentLevel int) converter {
	c.indentLevel = indentLevel
	return c
}

// GenerateFieldsDeltasCode translates FieldDeltas into Go code that
// converts a CRD to another.
func (c converter) GenerateFieldsDeltasCode(
	deltas []multiversion.FieldDelta,
) string {
	out := ""
	for _, delta := range deltas {
		from := delta.Source
		to := delta.Destination

		switch delta.ChangeType {
		case multiversion.FieldChangeTypeNone:
			out += c.copyField(from, to)
		case multiversion.FieldChangeTypeRenamed:
			out += c.copyField(from, to)
		case multiversion.FieldChangeTypeAdded,
			multiversion.FieldChangeTypeRemoved,
			multiversion.FieldChangeTypeShapeChanged,
			multiversion.FieldChangeTypeShapeChangedFromStringToSecret,
			multiversion.FieldChangeTypeShapeChangedFromSecretToString:
			fmt.Println("Not implemented ChangeType in generate.code.generateFieldsDeltasCode")
		case multiversion.FieldChangeTypeUnknown:
			panic("Received unknown ChangeType in generate.code.generateFieldsDeltasCode")
		default:
			panic("Unsupported ChangeType in generate.code.generateFieldsDeltasCode")
		}
	}
	return out
}

func (c converter) getGoType(original string) string {
	goType := original
	// first try to find a rename
	rename, ok := c.typeRenames[original]
	if ok {
		goType = rename
	}
	// then transform to camel if name dosen't have an '_SDK' suffix
	if !strings.HasSuffix(goType, "_SDK") {
		goType = names.New(goType).Camel
	}
	return goType
}

// copyStruct outputs Go code that converts a struct to another.
//
// Output code will look something like this:
//
//   elementCopy := &v1alpha2.WebhookFilter{}
//   if element != nil {
//   	webhookFilterCopy := &v1alpha2.WebhookFilter{}
//   	webhookFilterCopy.ExcludeMatchedPattern = element.ExcludeMatchedPattern
//   	webhookFilterCopy.Pattern = element.Pattern
//   	webhookFilterCopy.Type = element.Type
//   	elementCopy = webhookFilterCopy
//   }
func (c converter) copyStruct(shape *awssdkmodel.Shape) string {
	out := ""
	indent := strings.Repeat("\t", c.indentLevel)
	//TODO(a-hilaly): use ackcompare.HasNilDifference
	// if src.Spec.Tags != nil {
	out += fmt.Sprintf(
		"%sif %s != nil {\n",
		indent,
		c.srcVar,
	)

	structShapeName := names.New(shape.ShapeName)
	varStructCopy := structShapeName.CamelLower + "Copy"
	// webhookFilterCopy := &v1alpha2.WebhookFilter{}
	out += c.
		withIndentLevel(c.indentLevel+1).
		newShapeTypeInstance(
			shape,
			varStructCopy,
			c.srcVar,
			true,
		)

	// copy struct fields
	for _, memberName := range shape.MemberNames() {
		memberShapeRef := shape.MemberRefs[memberName]
		memberShape := memberShapeRef.Shape
		memberNames := names.New(memberName)

		switch memberShape.Type {
		case "structure":
			out += c.
				withIndentLevel(c.indentLevel+1).
				WithVariables(
					c.srcVar+"."+memberNames.Camel,
					varStructCopy+"."+memberNames.Camel,
				).
				copyStruct(
					memberShape,
				)
		case "list":
			out += c.
				withIndentLevel(c.indentLevel+1).
				WithVariables(
					c.srcVar+"."+memberNames.Camel,
					varStructCopy+"."+memberNames.Camel,
				).
				copyList(
					memberShape,
				)
		case "map":
			out += c.
				withIndentLevel(c.indentLevel+1).
				WithVariables(
					c.srcVar+"."+memberNames.Camel,
					varStructCopy+"."+memberNames.Camel,
				).
				copyMap(
					memberShape,
				)
		default:
			out += c.
				withIndentLevel(c.indentLevel+1).
				WithVariables(
					c.srcVar+"."+memberNames.Camel,
					varStructCopy+"."+memberNames.Camel,
				).
				copyScalar(
					memberShape,
				)
		}
	}
	// elementCopy = webhookFilterCopy
	out += storeVariableIn(
		varStructCopy,
		c.dstVar,
		false,
		c.indentLevel+1,
	)
	out += fmt.Sprintf(
		"%s}\n", indent,
	)
	return out + "\n"
}

// copyScalar outputs Go code that converts a CRD field
// that is a scalar.
//
// Output code will look something like this:
//
//   dst.Status.CreatedAt = src.Status.CreatedAt
func (c converter) copyScalar(shape *awssdkmodel.Shape) string {
	out := ""
	indent := strings.Repeat("\t", c.indentLevel)

	switch shape.Type {
	case "boolean", "string", "character", "byte", "short",
		"integer", "long", "float", "double", "timestamp":
		out += fmt.Sprintf(
			// dst.Spec.Conditions = src.Spec.Conditions
			"%s%s = %s\n",
			indent, c.dstVar, c.srcVar,
		)
	default:
		panic("Unsupported shape type: " + shape.Type)
	}
	return out
}

// copyList outputs Go code that copies one array to another.
//
// Output code will look something like this:
//
//  if src.Spec.APIStages != nil {
//  	listOfAPIStageCopy := make([]*APIStage, 0, len(src.Spec.APIStages))
//  	for i, element := range src.Spec.APIStages {
//  		_ = i // non-used value guard.
//  		elementCopy := &APIStage{}
//  		if element != nil {
//  			apiStageCopy := &APIStage{}
//  			apiStageCopy.APIID = element.APIID
//  			apiStageCopy.Stage = element.Stage
//  			if element.Throttle != nil {
//  				mapOfAPIStageThrottleSettingsCopy := make(map[string]*ThrottleSettings, len(element.Throttle))
//  				for k, v := range element.Throttle {
//  					elementCopy := &ThrottleSettings{}
//  					if v != nil {
//  						throttleSettingsCopy := &ThrottleSettings{}
//  						throttleSettingsCopy.BurstLimit = v.BurstLimit
//  						throttleSettingsCopy.RateLimit = v.RateLimit
//  						elementCopy = throttleSettingsCopy
//  					}
//
//  					mapOfAPIStageThrottleSettingsCopy[k] = elementCopy
//  				}
//  				apiStageCopy.Throttle = mapOfAPIStageThrottleSettingsCopy
//  			}
//
//  			elementCopy = apiStageCopy
//  		}
//
//  		listOfAPIStageCopy = append(listOfAPIStageCopy, elementCopy)
//  	}
//  	dst.Spec.APIStages = listOfAPIStageCopy
//  }
func (c converter) copyList(shape *awssdkmodel.Shape) string {
	indent := strings.Repeat("\t", c.indentLevel)
	// if a slice is only made of builtin types we just copy src to dst
	if isMadeOfBuiltinTypes(shape) {
		return fmt.Sprintf(
			"%s%s = %s\n",
			indent,
			c.srcVar,
			c.dstVar,
		)
	}

	out := ""
	// if err != nil {
	out += fmt.Sprintf(
		"%sif %s != nil {\n",
		indent,
		c.srcVar,
	)

	structShapeName := names.New(shape.ShapeName)
	varStructCopy := structShapeName.CamelLower + "Copy"

	// filterGroupsCopy := make([][]*v1alpha2.WebhookFilter, 0, len(src.Spec.FilterGroups))
	out += c.withIndentLevel(c.indentLevel+1).newShapeTypeInstance(
		shape,
		varStructCopy,
		c.srcVar,
		false,
	)

	varIndex := "i"
	varElement := "element"
	// for i, element := range src.Spec.FilterGroups {
	out += fmt.Sprintf(
		"%s\tfor %s, %s := range %s {\n",
		indent,
		varIndex,
		varElement,
		c.srcVar,
	)

	// _ = i // non-used value guard.
	out += fmt.Sprintf(
		"%s\t\t_ = %s // non-used value guard.\n",
		indent,
		varIndex,
	)

	memberShapeRef := shape.MemberRef
	memberShape := memberShapeRef.Shape
	// elementCopy := make([]*v1alpha2.WebhookFilter, 0, len(element))
	varElementCopy := "element" + "Copy"
	out += c.withIndentLevel(c.indentLevel+2).newShapeTypeInstance(
		memberShape,
		varElementCopy,
		varElement,
		true,
	)

	switch memberShape.Type {
	case "structure":
		out += c.
			withIndentLevel(c.indentLevel+2).
			WithVariables(varElement, varElementCopy).
			copyStruct(
				memberShape,
			)
	case "list":
		if isMadeOfBuiltinTypes(memberShape) {
			out += fmt.Sprintf(
				"%s%s = %s\n",
				indent,
				c.srcVar,
				c.dstVar,
			)
		} else {
			out += c.
				withIndentLevel(c.indentLevel+2).
				WithVariables(varElement, varElementCopy).
				copyList(
					memberShape,
				)
		}
	case "map":
		if isMadeOfBuiltinTypes(memberShape) {
			out += fmt.Sprintf(
				"%s%s = %s\n",
				indent,
				c.srcVar,
				c.dstVar,
			)
		} else {
			out += c.withIndentLevel(c.indentLevel+2).
				WithVariables(varElement, varElementCopy).
				copyMap(
					memberShape,
				)
		}
	default:
		panic(fmt.Sprintf("Unsupported shape type in generate.code.copyMap"))
	}

	// filterGroupCopy = append(filterGroupCopy, elementCopy)
	out += fmt.Sprintf(
		"%s\t\t%s = append(%s, %s)\n",
		indent,
		varStructCopy,
		varStructCopy,
		varElementCopy,
	)

	// closing loop
	out += fmt.Sprintf(
		"%s\t}\n", indent,
	)

	// dst.Spec.FilterGroups = filterGroupsCopy
	out += storeVariableIn(varStructCopy, c.dstVar, false, c.indentLevel+1)
	out += fmt.Sprintf(
		"%s}\n", indent,
	)
	return out + "\n"
}

// copyMap outputs Go code that copies one map to another.
//
// Output code will look something like this:
//
//   if src.Spec.RouteSettings != nil {
//   	routeSettingsMapCopy := make(map[string]*v1alpha2.RouteSettings, len(src.Spec.RouteSettings))
//   	for k, v := range src.Spec.RouteSettings {
//   		elementCopy := &v1alpha2.RouteSettings{}
//   		if v != nil {
//   			routeSettingsCopy := &v1alpha2.RouteSettings{}
//   			routeSettingsCopy.DataTraceEnabled = v.DataTraceEnabled
//   			routeSettingsCopy.DetailedMetricsEnabled = v.DetailedMetricsEnabled
//   			routeSettingsCopy.LoggingLevel = v.LoggingLevel
//   			routeSettingsCopy.ThrottlingBurstLimit = v.ThrottlingBurstLimit
//   			routeSettingsCopy.ThrottlingRateLimit = v.ThrottlingRateLimit
//   			elementCopy = routeSettingsCopy
//   		}
//
//   		routeSettingsMapCopy[k] = elementCopy
//   	}
//   	dst.Spec.RouteSettings = routeSettingsMapCopy
//   }
func (c converter) copyMap(shape *awssdkmodel.Shape) string {
	indent := strings.Repeat("\t", c.indentLevel)
	// if a map is only made of builtin types we just copy src to dst
	if isMadeOfBuiltinTypes(shape) {
		return storeVariableIn(
			c.srcVar,
			c.dstVar,
			false,
			c.indentLevel,
		)
	}

	// if err != nil
	out := fmt.Sprintf(
		"%sif %s != nil {\n",
		indent,
		c.srcVar,
	)

	// routeSettingsCopy := &v1alpha2.RouteSettings{}
	structShapeName := names.New(shape.ShapeName)
	varStructCopy := structShapeName.CamelLower + "Copy"
	out += c.
		withIndentLevel(c.indentLevel+1).
		newShapeTypeInstance(
			shape,
			varStructCopy,
			c.srcVar,
			false,
		)

	keyVarName := "k"
	valueVarName := "v"
	// for k, v := range src.Spec.RouteSettings {
	out += fmt.Sprintf(
		"%s\tfor %s, %s := range %s {\n",
		indent,
		keyVarName,
		valueVarName,
		c.srcVar,
	)

	memberShape := shape.ValueRef.Shape
	varElementCopy := "element" + "Copy"
	// elementCopy := &v1alpha2.RouteSettings{}
	out += c.
		withIndentLevel(c.indentLevel+2).
		newShapeTypeInstance(
			memberShape,
			varElementCopy,
			valueVarName,
			true,
		)

	switch memberShape.Type {
	case "structure":
		out += c.
			withIndentLevel(c.indentLevel+2).
			WithVariables(valueVarName, varElementCopy).
			copyStruct(
				memberShape,
			)
	case "list":
		if isMadeOfBuiltinTypes(memberShape) {
			out += fmt.Sprintf(
				"%s%s = %s\n",
				indent,
				c.srcVar,
				c.dstVar,
			)
		} else {
			out += c.withIndentLevel(c.indentLevel+2).
				WithVariables(valueVarName, varElementCopy).
				copyMap(
					memberShape,
				)
		}
	case "map":
		if isMadeOfBuiltinTypes(memberShape) {
			out += fmt.Sprintf(
				"%s%s = %s\n",
				indent,
				c.srcVar,
				c.dstVar,
			)
		} else {
			out += c.withIndentLevel(c.indentLevel+2).
				WithVariables(valueVarName, varElementCopy).
				copyMap(
					memberShape,
				)
		}
	default:
		msg := fmt.Sprintf("Unsupported shape type in generate.code.copyMap %s", memberShape.Type)
		panic(msg)
	}

	// routeSettingsMapCopy[k] = elementCopy
	out += fmt.Sprintf(
		"%s\t\t%s[%s] = %s\n",
		indent,
		varStructCopy,
		keyVarName,
		varElementCopy,
	)
	out += fmt.Sprintf(
		"%s\t}\n", indent,
	)

	// dst.Spec.RouteSettings = routeSettingsMapCopy
	out += storeVariableIn(varStructCopy, c.dstVar, false, c.indentLevel+1)

	// attach the copy struct to the dst variable
	out += fmt.Sprintf(
		"%s}\n", indent,
	)

	return out + "\n"
}

// copyField returns go code that copies a src field to destination field. Typically
// field have similar Go structures but imported from different packages.
//
// Output code that looks like this:
//
//   dst.Spec.ProjectName = src.Spec.ProjectName
func (c converter) copyField(from, to *model.Field) string {
	// if a field is not renamed, from and to have the same name
	// so the name doesn't impact much the code generation
	varFromPath := c.srcVar + "." + from.Names.Camel
	varToPath := c.dstVar + "." + to.Names.Camel

	// however in case of renames we should correctly invert from/to field
	// paths. Only when we are convert to hub (not from hub).
	if !c.isConvertingToHub {
		varFromPath = c.srcVar + "." + to.Names.Camel
		varToPath = c.dstVar + "." + from.Names.Camel
	}

	switch from.ShapeRef.Shape.Type {
	case "structure":
		return c.
			WithVariables(varFromPath, varToPath).
			copyStruct(from.ShapeRef.Shape)
	case "list":
		return c.
			WithVariables(varFromPath, varToPath).
			copyList(from.ShapeRef.Shape)
	case "map":
		return c.
			WithVariables(varFromPath, varToPath).
			copyMap(from.ShapeRef.Shape)
	default:
		return c.
			WithVariables(varFromPath, varToPath).
			copyScalar(from.ShapeRef.Shape)
	}
}

// newShapeTypeInstance returns Go code that instanciate a new shape type.
//
// Output code will look something like this:
//
//   imageScanningConfigurationCopy := &v2.ImageScanningConfiguration{}
func (c converter) newShapeTypeInstance(
	shape *awssdkmodel.Shape,
	allocationVarName string,
	fromVar string,
	isPointer bool,
) string {
	out := ""
	indent := strings.Repeat("\t", c.indentLevel)

	switch shape.Type {
	case "structure":
		goType := shape.GoTypeElem()
		goType = c.getGoType(goType)
		if c.isConvertingToHub {
			goType = c.hubImportAlias + "." + goType
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
		goType := shape.GoTypeWithPkgName()
		if c.isConvertingToHub {
			goType = ackmodel.ReplacePkgName(
				goType,
				c.sdkPackageName,
				c.hubImportAlias,
				true,
			)
		} else {
			goType = ackmodel.ReplacePkgName(
				goType,
				c.sdkPackageName,
				"",
				true,
			)
		}
		out += fmt.Sprintf(
			"%s%s := make(%s, 0, len(%s))\n",
			indent,
			allocationVarName,
			goType,
			fromVar,
		)
	case "map":
		goType := shape.GoTypeWithPkgName()
		if c.isConvertingToHub {
			goType = ackmodel.ReplacePkgName(
				goType,
				c.sdkPackageName,
				c.hubImportAlias,
				true,
			)
		} else {
			goType = ackmodel.ReplacePkgName(
				goType,
				c.sdkPackageName,
				"",
				true,
			)
		}
		out += fmt.Sprintf(
			"%s%s := make(%s, len(%s))\n",
			indent,
			allocationVarName,
			goType,
			fromVar,
		)
	default:
		msg := fmt.Sprintf("Unsupported shape type in generate.code.newShapeTypeInstance %s", shape.Type)
		panic(msg)
	}

	return out
}

// storeVariableIn retruns go code that stores a value in a given variable.
func storeVariableIn(
	// the value name to store
	from string,
	// the target name to store in
	target string,
	// whether to allocate the target variable
	allocate bool,
	// indentation level.
	indentLevel int,
) string {
	out := ""
	indent := strings.Repeat("\t", indentLevel)
	assignValue := "="
	if allocate {
		assignValue = ":" + assignValue
	}
	out += fmt.Sprintf(
		"%s%s %s %s\n",
		indent,
		target,
		assignValue,
		from,
	)
	return out
}

// isMadeOfBuiltinTypes returns true if a given shape is fully made of Go
// builtin types. Knowning such information allows us to directly copy a
// variable into another without having to minutely walk and copy the elements.
func isMadeOfBuiltinTypes(shape *awssdkmodel.Shape) bool {
	switch shape.Type {
	case "boolean", "string", "character", "byte", "short",
		"integer", "long", "float", "double", "timestamp":
		return true
	case "list":
		return isMadeOfBuiltinTypes(shape.MemberRef.Shape)
	case "map":
		return isMadeOfBuiltinTypes(shape.ValueRef.Shape)
	default:
		return false
	}
}