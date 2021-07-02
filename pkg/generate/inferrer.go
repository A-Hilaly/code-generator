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

package generate

import (
	"fmt"
	"sort"
	"strings"

	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	"github.com/aws-controllers-k8s/code-generator/pkg/generate/templateset"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
	"github.com/aws-controllers-k8s/code-generator/pkg/names"
	"github.com/aws-controllers-k8s/code-generator/pkg/util"
)

// Inferrer contains the ACK model for the generator to process and apply
// templates against
type Inferrer struct {
	SDKAPI       *ackmodel.SDKAPI
	serviceAlias string
	apiVersion   string
	crds         []*ackmodel.CRD
	typeDefs     []*ackmodel.TypeDef
	typeImports  map[string]string
	typeRenames  map[string]string
	// Instructions to the code generator how to handle the API and its
	// resources
	cfg *ackgenconfig.Config
}

// MetaVars returns a MetaVars struct populated with metadata about the AWS
// service API
func (g *Inferrer) MetaVars() templateset.MetaVars {
	return templateset.MetaVars{
		ServiceAlias:            g.serviceAlias,
		ServiceID:               g.SDKAPI.ServiceID(),
		ServiceIDClean:          g.SDKAPI.ServiceIDClean(),
		APIGroup:                g.SDKAPI.APIGroup(),
		APIVersion:              g.apiVersion,
		SDKAPIInterfaceTypeName: g.SDKAPI.SDKAPIInterfaceTypeName(),
		CRDNames:                g.crdNames(),
	}
}

// crdNames returns all crd names lowercased and in plural
func (g *Inferrer) crdNames() []string {
	var crdConfigs []string

	crds, _ := g.GetCRDs()
	for _, crd := range crds {
		crdConfigs = append(crdConfigs, strings.ToLower(crd.Plural))
	}

	return crdConfigs
}

// GetCRDs returns a slice of `ackmodel.CRD` structs that describe the
// top-level resources discovered by the code generator for an AWS service API
func (g *Inferrer) GetCRDs() ([]*ackmodel.CRD, error) {
	if g.crds != nil {
		return g.crds, nil
	}

	crds, err := GetCRDs(g.SDKAPI, g.cfg)
	if err != nil {
		return nil, err
	}

	g.crds = crds
	return crds, nil
}

// GetTypeDefs returns a slice of `ackmodel.TypeDef` pointers
func (g *Inferrer) GetTypeDefs() ([]*ackmodel.TypeDef, error) {
	if g.typeDefs != nil {
		return g.typeDefs, nil
	}

	tdefs := []*ackmodel.TypeDef{}
	// Map, keyed by original Shape GoTypeElem(), with the values being a
	// renamed type name (due to conflicting names)
	trenames := map[string]string{}

	payloads := g.SDKAPI.GetPayloads()

	for shapeName, shape := range g.SDKAPI.API.Shapes {
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
		if g.SDKAPI.HasConflictingTypeName(shapeName, g.cfg) {
			tdefNames.Camel += ackmodel.ConflictingNameSuffix
			trenames[shapeName] = tdefNames.Camel
		}

		attrs := map[string]*ackmodel.Attr{}
		for memberName, memberRef := range shape.MemberRefs {
			memberNames := names.New(memberName)
			memberShape := memberRef.Shape
			if IsShapeUsedInCRDs(g.crds, memberShape.ShapeName) {
				continue
			}
			// There are shapes that are called things like DBProxyStatus that are
			// fields in a DBProxy CRD... we need to ensure the type names don't
			// conflict. Also, the name of the Go type in the generated code is
			// Camel-cased and normalized, so we use that as the Go type
			gt := memberShape.GoType()
			if memberShape.Type == "structure" {
				typeNames := names.New(memberShape.ShapeName)
				if g.SDKAPI.HasConflictingTypeName(memberShape.ShapeName, g.cfg) {
					typeNames.Camel += ackmodel.ConflictingNameSuffix
				}
				gt = "*" + typeNames.Camel
			} else if memberShape.Type == "list" {
				// If it's a list type, where the element is a structure, we need to
				// set the GoType to the cleaned-up Camel-cased name
				if memberShape.MemberRef.Shape.Type == "structure" {
					elemType := memberShape.MemberRef.Shape.GoTypeElem()
					typeNames := names.New(elemType)
					if g.SDKAPI.HasConflictingTypeName(elemType, g.cfg) {
						typeNames.Camel += ackmodel.ConflictingNameSuffix
					}
					gt = "[]*" + typeNames.Camel
				}
			} else if memberShape.Type == "map" {
				// If it's a map type, where the value element is a structure,
				// we need to set the GoType to the cleaned-up Camel-cased name
				if memberShape.ValueRef.Shape.Type == "structure" {
					valType := memberShape.ValueRef.Shape.GoTypeElem()
					typeNames := names.New(valType)
					if g.SDKAPI.HasConflictingTypeName(valType, g.cfg) {
						typeNames.Camel += ackmodel.ConflictingNameSuffix
					}
					gt = "[]map[string]*" + typeNames.Camel
				}
			} else if memberShape.Type == "timestamp" {
				// time.Time needs to be converted to apimachinery/metav1.Time
				// otherwise there is no DeepCopy support
				gt = "*metav1.Time"
			}
			attrs[memberName] = ackmodel.NewAttr(memberNames, gt, memberShape)
		}
		if len(attrs) == 0 {
			// Just ignore these...
			continue
		}
		tdefs = append(tdefs, &ackmodel.TypeDef{
			Shape: shape,
			Names: tdefNames,
			Attrs: attrs,
		})
	}
	sort.Slice(tdefs, func(i, j int) bool {
		return tdefs[i].Names.Camel < tdefs[j].Names.Camel
	})
	g.processNestedFieldTypeDefs(tdefs)
	g.typeDefs = tdefs
	g.typeRenames = trenames
	return tdefs, nil
}

// processNestedFieldTypeDefs updates the supplied TypeDef structs' if a nested
// field has been configured with a type overriding FieldConfig -- such as
// FieldConfig.IsSecret.
func (g *Inferrer) processNestedFieldTypeDefs(
	tdefs []*ackmodel.TypeDef,
) {
	crds, _ := g.GetCRDs()
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
				replaceSecretAttrGoType(crd, field, tdefs)
			}
		}
	}
}

// replaceSecretAttrGoType replaces a nested field ackmodel.Attr's GoType with
// `*ackv1alpha1.SecretKeyReference`.
func replaceSecretAttrGoType(
	crd *ackmodel.CRD,
	field *ackmodel.Field,
	tdefs []*ackmodel.TypeDef,
) {
	fieldPath := field.Path
	parentFieldPath := ackmodel.ParentFieldPath(field.Path)
	parentField, ok := crd.Fields[parentFieldPath]
	if !ok {
		msg := fmt.Sprintf(
			"Cannot find parent field at parent path %s for %s",
			parentFieldPath,
			fieldPath,
		)
		panic(msg)
	}
	if parentField.ShapeRef == nil {
		msg := fmt.Sprintf(
			"parent field at parent path %s has a nil ShapeRef!",
			parentFieldPath,
		)
		panic(msg)
	}
	parentFieldShape := parentField.ShapeRef.Shape
	parentFieldShapeName := parentField.ShapeRef.ShapeName
	parentFieldShapeType := parentFieldShape.Type
	// For list and map types, we need to grab the element/value
	// type, since that's the type def we need to modify.
	if parentFieldShapeType == "list" {
		if parentFieldShape.MemberRef.Shape.Type != "structure" {
			msg := fmt.Sprintf(
				"parent field at parent path %s is a list type with a non-structure element member shape %s!",
				parentFieldPath,
				parentFieldShape.MemberRef.Shape.Type,
			)
			panic(msg)
		}
		parentFieldShapeName = parentField.ShapeRef.Shape.MemberRef.ShapeName
	} else if parentFieldShapeType == "map" {
		if parentFieldShape.ValueRef.Shape.Type != "structure" {
			msg := fmt.Sprintf(
				"parent field at parent path %s is a map type with a non-structure value member shape %s!",
				parentFieldPath,
				parentFieldShape.ValueRef.Shape.Type,
			)
			panic(msg)
		}
		parentFieldShapeName = parentField.ShapeRef.Shape.ValueRef.ShapeName
	}
	var parentTypeDef *ackmodel.TypeDef
	for _, tdef := range tdefs {
		if tdef.Names.Original == parentFieldShapeName {
			parentTypeDef = tdef
		}
	}
	if parentTypeDef == nil {
		msg := fmt.Sprintf(
			"unable to find associated TypeDef for parent field "+
				"at parent path %s!",
			parentFieldPath,
		)
		panic(msg)
	}
	// Now we modify the parent type def's Attr that corresponds to
	// the secret field...
	attr, found := parentTypeDef.Attrs[field.Names.Camel]
	if !found {
		msg := fmt.Sprintf(
			"unable to find attr %s in parent TypeDef %s "+
				"at parent path %s!",
			field.Names.Camel,
			parentTypeDef.Names.Original,
			parentFieldPath,
		)
		panic(msg)
	}
	attr.GoType = "*ackv1alpha1.SecretKeyReference"
}

// GetEnumDefs returns a slice of pointers to `ackmodel.EnumDef` structs which
// represent string fields whose value is constrained to one or more specific
// string values.
func (g *Inferrer) GetEnumDefs() ([]*ackmodel.EnumDef, error) {
	edefs := []*ackmodel.EnumDef{}

	for shapeName, shape := range g.SDKAPI.API.Shapes {
		if !shape.IsEnum() {
			continue
		}
		enumNames := names.New(shapeName)
		// Handle name conflicts with top-level CRD.Spec or CRD.Status
		// types
		if g.SDKAPI.HasConflictingTypeName(shapeName, g.cfg) {
			enumNames.Camel += ackmodel.ConflictingNameSuffix
		}
		edef, err := ackmodel.NewEnumDef(enumNames, shape.Enum)
		if err != nil {
			return nil, err
		}
		edefs = append(edefs, edef)
	}
	sort.Slice(edefs, func(i, j int) bool {
		return edefs[i].Names.Camel < edefs[j].Names.Camel
	})
	return edefs, nil
}

// New returns a new Generator struct for a supplied API model.
// Optionally, pass a file path to a generator config file that can be used to
// instruct the code generator how to handle the API properly
func New(
	SDKAPI *ackmodel.SDKAPI,
	apiVersion string,
	configPath string,
	defaultConfig ackgenconfig.Config,
) (*Inferrer, error) {
	cfg, err := ackgenconfig.New(configPath, defaultConfig)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %s", err)
	}
	g := &Inferrer{
		SDKAPI: SDKAPI,
		// TODO(jaypipes): Handle cases where service alias and service ID
		// don't match (Step Functions)
		serviceAlias: SDKAPI.ServiceID(),
		apiVersion:   apiVersion,
		cfg:          &cfg,
	}
	ApplyShapeIgnoreRules(SDKAPI, &cfg)
	return g, nil
}

// GetConfig returns the configuration option used to define the current
// generator.
func (g *Inferrer) GetConfig() *ackgenconfig.Config {
	return g.cfg
}
