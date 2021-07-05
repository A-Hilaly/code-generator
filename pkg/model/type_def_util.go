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

package model

import (
	"fmt"
	"sort"
	"strings"

	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	"github.com/aws-controllers-k8s/code-generator/pkg/names"
	"github.com/aws-controllers-k8s/code-generator/pkg/util"
)

// GetTypeDefs returns a slice of `ackmodel.TypeDef` pointers
func GetTypeDefs(
	SDKAPI *SDKAPI,
	crds []*CRD,
	cfg *ackgenconfig.Config,
) ([]*TypeDef, map[string]string, error) {
	tdefs := []*TypeDef{}
	// Map, keyed by original Shape GoTypeElem(), with the values being a
	// renamed type name (due to conflicting names)
	trenames := map[string]string{}

	payloads := SDKAPI.GetPayloads()

	for shapeName, shape := range SDKAPI.API.Shapes {
		if util.InStrings(shapeName, payloads) {
			// Payloads are not type defs
			continue
		}
		if shape.Type != "structure" {
			continue
		}
		if shape.Exception {
			// Neither are exceptions
			continue
		}
		tdefNames := names.New(shapeName)
		if SDKAPI.HasConflictingTypeName(shapeName, cfg) {
			tdefNames.Camel += ConflictingNameSuffix
			trenames[shapeName] = tdefNames.Camel
		}

		attrs := map[string]*Attr{}
		for memberName, memberRef := range shape.MemberRefs {
			memberNames := names.New(memberName)
			memberShape := memberRef.Shape
			if !IsShapeUsedInCRDs(crds, memberShape.ShapeName) {
				continue
			}
			// There are shapes that are called things like DBProxyStatus that are
			// fields in a DBProxy CRD... we need to ensure the type names don't
			// conflict. Also, the name of the Go type in the generated code is
			// Camel-cased and normalized, so we use that as the Go type
			gt := memberShape.GoType()
			if memberShape.Type == "structure" {
				typeNames := names.New(memberShape.ShapeName)
				if SDKAPI.HasConflictingTypeName(memberShape.ShapeName, cfg) {
					typeNames.Camel += ConflictingNameSuffix
				}
				gt = "*" + typeNames.Camel
			} else if memberShape.Type == "list" {
				// If it's a list type, where the element is a structure, we need to
				// set the GoType to the cleaned-up Camel-cased name
				if memberShape.MemberRef.Shape.Type == "structure" {
					elemType := memberShape.MemberRef.Shape.GoTypeElem()
					typeNames := names.New(elemType)
					if SDKAPI.HasConflictingTypeName(elemType, cfg) {
						typeNames.Camel += ConflictingNameSuffix
					}
					gt = "[]*" + typeNames.Camel
				}
			} else if memberShape.Type == "map" {
				// If it's a map type, where the value element is a structure,
				// we need to set the GoType to the cleaned-up Camel-cased name
				if memberShape.ValueRef.Shape.Type == "structure" {
					valType := memberShape.ValueRef.Shape.GoTypeElem()
					typeNames := names.New(valType)
					if SDKAPI.HasConflictingTypeName(valType, cfg) {
						typeNames.Camel += ConflictingNameSuffix
					}
					gt = "[]map[string]*" + typeNames.Camel
				}
			} else if memberShape.Type == "timestamp" {
				// time.Time needs to be converted to apimachinery/metav1.Time
				// otherwise there is no DeepCopy support
				gt = "*metav1.Time"
			}
			attrs[memberName] = NewAttr(memberNames, gt, memberShape)
		}
		if len(attrs) == 0 {
			// Just ignore these...
			continue
		}
		tdefs = append(tdefs, &TypeDef{
			Shape: shape,
			Names: tdefNames,
			Attrs: attrs,
		})
	}
	sort.Slice(tdefs, func(i, j int) bool {
		return tdefs[i].Names.Camel < tdefs[j].Names.Camel
	})
	processNestedFieldTypeDefs(crds, tdefs)
	return tdefs, trenames, nil
}

// processNestedFieldTypeDefs updates the supplied TypeDef structs' if a nested
// field has been configured with a type overriding FieldConfig -- such as
// FieldConfig.IsSecret.
func processNestedFieldTypeDefs(
	crds []*CRD,
	tdefs []*TypeDef,
) error {
	for _, crd := range crds {
		for fieldPath, field := range crd.Fields {
			if !strings.Contains(fieldPath, ".") {
				// top-level fields have already had their structure
				// transformed during the CRD.AddSpecField and
				// CRD.AddStatusField methods. All we need to do here is look
				// at nested fields, which are identifiable as fields with
				// field paths contains a dot (".")
				continue
			}
			if field.FieldConfig == nil {
				// Likewise, we don't need to transform any TypeDef if the
				// nested field doesn't have a FieldConfig instructing us to
				// treat this field differently.
				continue
			}
			if field.FieldConfig.IsSecret {
				// Find the TypeDef that was created for the *containing*
				// secret field struct. For example, assume the nested field
				// path `Users..Password`, we'd want to find the TypeDef that
				// was created for the `Users` field's element type (which is a
				// struct)
				if err := replaceSecretAttrGoType(crd, field, tdefs); err != nil {
					return fmt.Errorf("cannot replace secret attribute for field %s: %v", fieldPath, err)
				}
			}
		}
	}
	return nil
}

// replaceSecretAttrGoType replaces a nested field Attr's GoType with
// `*ackv1alpha1.SecretKeyReference`.
func replaceSecretAttrGoType(
	crd *CRD,
	field *Field,
	tdefs []*TypeDef,
) error {
	fieldPath := field.Path
	parentFieldPath := ParentFieldPath(field.Path)
	parentField, ok := crd.Fields[parentFieldPath]
	if !ok {
		return fmt.Errorf("cannot find parent field at parent path %s for %s", parentFieldPath, fieldPath)
	}
	if parentField.ShapeRef == nil {
		return fmt.Errorf("parent field at parent path %s has a nil ShapeRef", parentFieldPath)
	}
	parentFieldShape := parentField.ShapeRef.Shape
	parentFieldShapeName := parentField.ShapeRef.ShapeName
	parentFieldShapeType := parentFieldShape.Type
	// For list and map types, we need to grab the element/value
	// type, since that's the type def we need to modify.
	if parentFieldShapeType == "list" {
		if parentFieldShape.MemberRef.Shape.Type != "structure" {
			return fmt.Errorf(
				"parent field at parent path %s is a list type with a non-structure element member shape %s",
				parentFieldPath,
				parentFieldShape.MemberRef.Shape.Type,
			)
		}
		parentFieldShapeName = parentField.ShapeRef.Shape.MemberRef.ShapeName
	} else if parentFieldShapeType == "map" {
		if parentFieldShape.ValueRef.Shape.Type != "structure" {
			return fmt.Errorf(
				"parent field at parent path %s is a map type with a non-structure value member shape %s",
				parentFieldPath,
				parentFieldShape.ValueRef.Shape.Type,
			)
		}
		parentFieldShapeName = parentField.ShapeRef.Shape.ValueRef.ShapeName
	}
	var parentTypeDef *TypeDef
	for _, tdef := range tdefs {
		if tdef.Names.Original == parentFieldShapeName {
			parentTypeDef = tdef
		}
	}
	if parentTypeDef == nil {
		return fmt.Errorf(
			"unable to find associated TypeDef for parent field "+
				"at parent path %s",
			parentFieldPath,
		)
	}
	// Now we modify the parent type def's Attr that corresponds to
	// the secret field...
	attr, found := parentTypeDef.Attrs[field.Names.Camel]
	if !found {
		return fmt.Errorf(
			"unable to find attr %s in parent TypeDef %s at parent path %s",
			field.Names.Camel,
			parentTypeDef.Names.Original,
			parentFieldPath,
		)
	}
	attr.GoType = "*ackv1alpha1.SecretKeyReference"
	return nil
}
