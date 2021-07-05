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
	"errors"
	"fmt"
	"sort"
	"strings"

	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	"github.com/aws-controllers-k8s/code-generator/pkg/names"
)

var (
	// ErrNilShapePointer indicates an unexpected nil Shape pointer
	ErrNilShapePointer = errors.New("found nil Shape pointer")
)

// GetCRDs returns a slice of `CRD` structs that describe the
// top-level resources discovered by the code generator for an AWS service API
func GetCRDs(SDKAPI *SDKAPI, cfg *ackgenconfig.Config) ([]*CRD, error) {
	crds := []*CRD{}
	opMap := SDKAPI.GetOperationMap(cfg)

	createOps := (*opMap)[OpTypeCreate]
	readOneOps := (*opMap)[OpTypeGet]
	readManyOps := (*opMap)[OpTypeList]
	updateOps := (*opMap)[OpTypeUpdate]
	deleteOps := (*opMap)[OpTypeDelete]
	getAttributesOps := (*opMap)[OpTypeGetAttributes]
	setAttributesOps := (*opMap)[OpTypeSetAttributes]

	for crdName, createOp := range createOps {
		if cfg.IsIgnoredResource(crdName) {
			continue
		}
		crdNames := names.New(crdName)
		ops := Ops{
			Create:        createOps[crdName],
			ReadOne:       readOneOps[crdName],
			ReadMany:      readManyOps[crdName],
			Update:        updateOps[crdName],
			Delete:        deleteOps[crdName],
			GetAttributes: getAttributesOps[crdName],
			SetAttributes: setAttributesOps[crdName],
		}
		removeIgnoredOperations(&ops, cfg)
		crd := NewCRD(SDKAPI, cfg, crdNames, ops)

		// OK, begin to gather the CRDFields that will go into the Spec struct.
		// These fields are those members of the Create operation's Input
		// Shape.
		inputShape := createOp.InputRef.Shape
		if inputShape == nil {
			return nil, ErrNilShapePointer
		}
		for memberName, memberShapeRef := range inputShape.MemberRefs {
			if memberShapeRef.Shape == nil {
				return nil, ErrNilShapePointer
			}
			renamedName, _ := crd.InputFieldRename(
				createOp.Name, memberName,
			)
			memberNames := names.New(renamedName)
			memberNames.ModelOriginal = memberName
			if memberName == "Attributes" && cfg.UnpacksAttributesMap(crdName) {
				crd.UnpackAttributes()
				continue
			}
			crd.AddSpecField(memberNames, memberShapeRef)
		}

		// Now any additional Spec fields that are required from other API
		// operations.
		for targetFieldName, fieldConfig := range cfg.ResourceFields(crdName) {
			if fieldConfig.IsReadOnly {
				// It's a Status field...
				continue
			}
			if fieldConfig.From == nil {
				// Isn't an additional Spec field...
				continue
			}
			from := fieldConfig.From
			memberShapeRef, found := SDKAPI.GetInputShapeRef(
				from.Operation, from.Path,
			)
			if found {
				memberNames := names.New(targetFieldName)
				crd.AddSpecField(memberNames, memberShapeRef)
			} else {
				// This is a compile-time failure, just bomb out...
				msg := fmt.Sprintf(
					"unknown additional Spec field with Op: %s and Path: %s",
					from.Operation, from.Path,
				)
				panic(msg)
			}
		}

		// Now process the fields that will go into the Status struct. We want
		// fields that are in the Create operation's Output Shape but that are
		// not in the Input Shape.
		outputShape := createOp.OutputRef.Shape
		if outputShape.UsedAsOutput && len(outputShape.MemberRefs) == 1 {
			// We might be in a "wrapper" shape. Unwrap it to find the real object
			// representation for the CRD's createOp. If there is a single member
			// shape and that member shape is a structure, unwrap it.
			for _, memberRef := range outputShape.MemberRefs {
				if memberRef.Shape.Type == "structure" {
					outputShape = memberRef.Shape
				}
			}
		}
		for memberName, memberShapeRef := range outputShape.MemberRefs {
			if memberShapeRef.Shape == nil {
				return nil, ErrNilShapePointer
			}
			// Check that the field in the output shape isn't the same as
			// fields in the input shape (where the input shape has potentially
			// been renamed)
			renamedName, _ := crd.InputFieldRename(
				createOp.Name, memberName,
			)
			memberNames := names.New(renamedName)
			if _, found := crd.SpecFields[renamedName]; found {
				// We don't put fields that are already in the Spec struct into
				// the Status struct
				continue
			}
			if memberName == "Attributes" && cfg.UnpacksAttributesMap(crdName) {
				continue
			}
			if crd.IsPrimaryARNField(memberName) {
				// We automatically place the primary resource ARN value into
				// the Status.ACKResourceMetadata.ARN field
				continue
			}
			crd.AddStatusField(memberNames, memberShapeRef)
		}

		// Now add the additional Status fields that are required from other
		// API operations.
		for targetFieldName, fieldConfig := range cfg.ResourceFields(crdName) {
			if !fieldConfig.IsReadOnly {
				// It's a Spec field...
				continue
			}
			if fieldConfig.From == nil {
				// Isn't an additional Status field...
				continue
			}
			from := fieldConfig.From
			memberShapeRef, found := SDKAPI.GetOutputShapeRef(
				from.Operation, from.Path,
			)
			if found {
				memberNames := names.New(targetFieldName)
				crd.AddStatusField(memberNames, memberShapeRef)
			} else {
				// This is a compile-time failure, just bomb out...
				msg := fmt.Sprintf(
					"unknown additional Status field with Op: %s and Path: %s",
					from.Operation, from.Path,
				)
				panic(msg)
			}
		}

		crds = append(crds, crd)
	}
	sort.Slice(crds, func(i, j int) bool {
		return crds[i].Names.Camel < crds[j].Names.Camel
	})
	// This is the place that we build out the CRD.Fields map with
	// `pkg/model.Field` objects that represent the non-top-level Spec and
	// Status fields.
	processNestedFields(crds)
	return crds, nil
}

// RemoveIgnoredOperations updates Ops argument by setting those
// operations to nil that are configured to be ignored in generator config for
// the AWS service
func removeIgnoredOperations(ops *Ops, cfg *ackgenconfig.Config) {
	if cfg.IsIgnoredOperation(ops.Create) {
		ops.Create = nil
	}
	if cfg.IsIgnoredOperation(ops.ReadOne) {
		ops.ReadOne = nil
	}
	if cfg.IsIgnoredOperation(ops.ReadMany) {
		ops.ReadMany = nil
	}
	if cfg.IsIgnoredOperation(ops.Update) {
		ops.Update = nil
	}
	if cfg.IsIgnoredOperation(ops.Delete) {
		ops.Delete = nil
	}
	if cfg.IsIgnoredOperation(ops.GetAttributes) {
		ops.GetAttributes = nil
	}
	if cfg.IsIgnoredOperation(ops.SetAttributes) {
		ops.SetAttributes = nil
	}
}

// processNestedFields is responsible for walking all of the CRDs' Spec and
// Status fields' Shape objects and adding `pkg/model.Field` objects for all
// nested fields along with that `Field`'s `Config` object that allows us to
// determine if the TypeDef associated with that nested field should have its
// data type overridden (e.g. for SecretKeyReferences)
func processNestedFields(crds []*CRD) {
	for _, crd := range crds {
		for _, field := range crd.SpecFields {
			processNestedField(crd, field)
		}
		for _, field := range crd.StatusFields {
			processNestedField(crd, field)
		}
	}
}

// processNestedField processes any nested fields (non-scalar fields associated
// with the Spec and Status objects)
func processNestedField(
	crd *CRD,
	field *Field,
) {
	if field.ShapeRef == nil && (field.FieldConfig == nil || !field.FieldConfig.IsAttribute) {
		fmt.Printf(
			"WARNING: Field %s:%s has nil ShapeRef and is not defined as an Attribute-based Field!\n",
			crd.Names.Original,
			field.Names.Original,
		)
		return
	}
	if field.ShapeRef != nil {
		fieldShape := field.ShapeRef.Shape
		fieldType := fieldShape.Type
		switch fieldType {
		case "structure":
			processNestedStructField(crd, field.Path+".", field)
		case "list":
			processNestedListField(crd, field.Path+"..", field)
		case "map":
			processNestedMapField(crd, field.Path+"..", field)
		}
	}
}

// processNestedStructField recurses through the members of a nested field that
// is a struct type and adds any Field objects to the supplied CRD.
func processNestedStructField(
	crd *CRD,
	baseFieldPath string,
	baseField *Field,
) {
	fieldConfigs := crd.Config().ResourceFields(crd.Names.Original)
	baseFieldShape := baseField.ShapeRef.Shape
	for memberName, memberRef := range baseFieldShape.MemberRefs {
		memberNames := names.New(memberName)
		memberShape := memberRef.Shape
		memberShapeType := memberShape.Type
		fieldPath := baseFieldPath + memberNames.Camel
		fieldConfig := fieldConfigs[fieldPath]
		field := NewField(crd, fieldPath, memberNames, memberRef, fieldConfig)
		switch memberShapeType {
		case "structure":
			processNestedStructField(crd, fieldPath+".", field)
		case "list":
			processNestedListField(crd, fieldPath+"..", field)
		case "map":
			processNestedMapField(crd, fieldPath+"..", field)
		}
		crd.Fields[fieldPath] = field
	}
}

// processNestedListField recurses through the members of a nested field that
// is a list type that has a struct element type and adds any Field objects to
// the supplied CRD.
func processNestedListField(
	crd *CRD,
	baseFieldPath string,
	baseField *Field,
) {
	baseFieldShape := baseField.ShapeRef.Shape
	elementFieldShape := baseFieldShape.MemberRef.Shape
	if elementFieldShape.Type != "structure" {
		return
	}
	fieldConfigs := crd.Config().ResourceFields(crd.Names.Original)
	for memberName, memberRef := range elementFieldShape.MemberRefs {
		memberNames := names.New(memberName)
		memberShape := memberRef.Shape
		memberShapeType := memberShape.Type
		fieldPath := baseFieldPath + memberNames.Camel
		fieldConfig := fieldConfigs[fieldPath]
		field := NewField(crd, fieldPath, memberNames, memberRef, fieldConfig)
		switch memberShapeType {
		case "structure":
			processNestedStructField(crd, fieldPath+".", field)
		case "list":
			processNestedListField(crd, fieldPath+"..", field)
		case "map":
			processNestedMapField(crd, fieldPath+"..", field)
		}
		crd.Fields[fieldPath] = field
	}
}

// processNestedMapField recurses through the members of a nested field that
// is a map type that has a struct value type and adds any Field objects to
// the supplied CRD.
func processNestedMapField(
	crd *CRD,
	baseFieldPath string,
	baseField *Field,
) {
	baseFieldShape := baseField.ShapeRef.Shape
	valueFieldShape := baseFieldShape.ValueRef.Shape
	if valueFieldShape.Type != "structure" {
		return
	}
	fieldConfigs := crd.Config().ResourceFields(crd.Names.Original)
	for memberName, memberRef := range valueFieldShape.MemberRefs {
		memberNames := names.New(memberName)
		memberShape := memberRef.Shape
		memberShapeType := memberShape.Type
		fieldPath := baseFieldPath + memberNames.Camel
		fieldConfig := fieldConfigs[fieldPath]
		field := NewField(crd, fieldPath, memberNames, memberRef, fieldConfig)
		switch memberShapeType {
		case "structure":
			processNestedStructField(crd, fieldPath+".", field)
		case "list":
			processNestedListField(crd, fieldPath+"..", field)
		case "map":
			processNestedMapField(crd, fieldPath+"..", field)
		}
		crd.Fields[fieldPath] = field
	}
}

// ApplyShapeIgnoreRules removes the ignored shapes and fields from the API object
// so that they are not considered in any of the calculations of code generator.
func ApplyShapeIgnoreRules(SDKAPI *SDKAPI, cfg *ackgenconfig.Config) {
	if cfg == nil || SDKAPI == nil {
		return
	}
	for sdkShapeID, shape := range SDKAPI.API.Shapes {
		for _, fieldpath := range cfg.Ignore.FieldPaths {
			sn := strings.Split(fieldpath, ".")[0]
			fn := strings.Split(fieldpath, ".")[1]
			if shape.ShapeName != sn {
				continue
			}
			delete(shape.MemberRefs, fn)
		}
		for _, sn := range cfg.Ignore.ShapeNames {
			if shape.ShapeName == sn {
				delete(SDKAPI.API.Shapes, sdkShapeID)
				continue
			}
			// NOTE(muvaf): We need to remove the usage of the shape as well.
			for sdkMemberID, memberRef := range shape.MemberRefs {
				if memberRef.ShapeName == sn {
					delete(shape.MemberRefs, sdkMemberID)
				}
			}
		}
	}
}

// IsShapeUsedInCRDs returns true if the supplied shape name is a member of amy
// CRD's payloads or those payloads sub-member shapes
func IsShapeUsedInCRDs(crds []*CRD, shapeName string) bool {
	for _, crd := range crds {
		if crd.HasShapeAsMember(shapeName) {
			return true
		}
	}
	return false
}
