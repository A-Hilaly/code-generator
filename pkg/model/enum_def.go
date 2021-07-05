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
	"bytes"
	"fmt"
	"sort"

	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	"github.com/aws-controllers-k8s/code-generator/pkg/names"
)

type EnumValue struct {
	Original string
	Clean    string
}

// EnumDef is the definition of an enumeration type for a field present in
// either a CRD or a TypeDef
type EnumDef struct {
	Names  names.Names
	Values []EnumValue
}

// NewEnumDef returns a pointer to an `ackmodel.EnumDef` struct representing a
// constrained string value field
func NewEnumDef(names names.Names, values []string) (*EnumDef, error) {
	enumVals := make([]EnumValue, len(values))
	for x, item := range values {
		enumVals[x] = newEnumVal(item)
	}
	return &EnumDef{names, enumVals}, nil
}

func newEnumVal(orig string) EnumValue {
	// Convert values like "m5.xlarge" into "m5_xlarge"
	cleaner := func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}
	clean := bytes.Map(cleaner, []byte(orig))

	return EnumValue{
		Original: orig,
		Clean:    string(clean),
	}
}

// GetEnumDefs takes an *SDKAPI and *ackgenconfig.Config and returns a slice of
// pointers to `EnumDef` structs which represent string fields whose value is
// constrained to one or more specific string values.
func GetEnumDefs(SDKAPI *SDKAPI, cfg *ackgenconfig.Config) ([]*EnumDef, error) {
	edefs := []*EnumDef{}

	for shapeName, shape := range SDKAPI.API.Shapes {
		if !shape.IsEnum() {
			continue
		}
		enumNames := names.New(shapeName)
		// Handle name conflicts with top-level CRD.Spec or CRD.Status
		// types
		if SDKAPI.HasConflictingTypeName(shapeName, cfg) {
			enumNames.Camel += ConflictingNameSuffix
		}
		edef, err := NewEnumDef(enumNames, shape.Enum)
		if err != nil {
			return nil, fmt.Errorf("cannot create enum def %s: %v", enumNames.Camel, err)
		}
		edefs = append(edefs, edef)
	}
	sort.Slice(edefs, func(i, j int) bool {
		return edefs[i].Names.Camel < edefs[j].Names.Camel
	})
	return edefs, nil
}
