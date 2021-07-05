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

package multiversion

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	awssdkmodel "github.com/aws/aws-sdk-go/private/model/api"

	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
)

// FieldChangeType represent the type of field modification.
type FieldChangeType string

const (
	// FieldChangeTypeUnknown is used when ChangeType cannot be computed.
	FieldChangeTypeUnknown FieldChangeType = "unknown"
	// FieldChangeTypeIntact is used a field name and structure didn't change.
	FieldChangeTypeIntact FieldChangeType = "intact"
	// FieldChangeTypeAdded is used when a new field is introduced in a CRD.
	FieldChangeTypeAdded FieldChangeType = "added"
	// FieldChangeTypeRemoved is used a when a field is removed from a CRD.
	FieldChangeTypeRemoved FieldChangeType = "removed"
	// FieldChangeTypeRenamed is used when a field is renamed.
	FieldChangeTypeRenamed FieldChangeType = "renamed"
	// FieldChangeTypeShapeChanged is used when a field shape has changed.
	FieldChangeTypeShapeChanged FieldChangeType = "shape-changed"
	// FieldChangeTypeShapeChangedToSecret is used when a field change to
	// a k8s secret type.
	FieldChangeTypeShapeChangedToSecret FieldChangeType = "shape-changed-to-secret"
)

// FieldDelta represent the delta between the same field in two different
// CRD versions. If a field is removed in the Hub version the Hub value will
// be nil. If a field is new in the Hub version, the Spoke value will be nil.
type FieldDelta struct {
	ChangeType FieldChangeType
	// Field from the hub version CRD
	Hub *ackmodel.Field
	// Field from the spoke version CRD
	Spoke *ackmodel.Field
}

// CRDDelta stores the spec and status deltas for a custom resource.
type CRDDelta struct {
	SpecDeltas   []FieldDelta
	StatusDeltas []FieldDelta
}

// ComputeCRDFieldsDeltas compares two ackmodel.CRD instances and returns the
// spec and status fields deltas. spokeCRD is the CRD of the spoke (source) version
// and hubCRD is the CRD of the hub (destination) version.
func ComputeCRDFieldsDeltas(spokeCRD, hubCRD *ackmodel.CRD) (*CRDDelta, error) {
	renames, _, err := hubCRD.GetAllRenames()
	if err != nil {
		return nil, fmt.Errorf("cannot get resource field renames: %s", err)
	}

	specDeltas, err := ComputeFieldsDiff(spokeCRD.SpecFields, hubCRD.SpecFields, renames)
	if err != nil {
		return nil, fmt.Errorf("cannot compute spec fields deltas: %s", err)
	}

	statusDeltas, err := ComputeFieldsDiff(spokeCRD.StatusFields, hubCRD.StatusFields, renames)
	if err != nil {
		return nil, fmt.Errorf("cannot compute status fields deltas: %s", err)
	}

	return &CRDDelta{
		specDeltas,
		statusDeltas,
	}, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ComputeFieldsDiff computes the difference between two maps of fields. It returns a list
// of FieldDelta's that contains the the ChangeType and at least one field reference.
func ComputeFieldsDiff(
	spokeCRDFields map[string]*ackmodel.Field,
	hubCRDFields map[string]*ackmodel.Field,
	// the hub renames
	renames map[string]string,
) ([]FieldDelta, error) {
	// the smallest size of deltas we can get is max(len(spokeCRDFields), len(hubCRDFields))
	deltas := make([]FieldDelta, 0, max(len(spokeCRDFields), len(hubCRDFields)))

	// collect field names and sort them to ensure a determenistic output order.
	spokeFieldsNames := []string{}
	for name := range spokeCRDFields {
		spokeFieldsNames = append(spokeFieldsNames, name)
	}
	sort.Strings(spokeFieldsNames)

	hubFieldsNames := []string{}
	for name := range hubCRDFields {
		hubFieldsNames = append(hubFieldsNames, name)
	}
	sort.Strings(hubFieldsNames)

	// let's make sure we don't visit fields more than once - especially
	// when fields are renamed.
	visitedFields := map[string]*struct{}{}

	// first let's loop over the spokeFieldsNames array and see if we can find
	// the same field name in hubFieldNames.
	for _, spokeFieldName := range spokeFieldsNames {
		spokeField, _ := spokeCRDFields[spokeFieldName]
		hubField, ok := hubCRDFields[spokeFieldName]
		// If a field is found in both arrays only three changes are possible:
		// Intact, TypeChangeChange and ChangeTypeShapeChangedToSecret.
		// NOTE(a-hilaly): carefull about X -> Y then Z -> X renames. It should
		// not be allowed.
		if ok {
			// mark field as visited.
			visitedFields[spokeFieldName] = nil
			// check if field became secret
			if (spokeField.FieldConfig == nil ||
				(spokeField.FieldConfig != nil && !spokeField.FieldConfig.IsSecret)) &&
				(hubField.FieldConfig != nil && hubField.FieldConfig.IsSecret) {
				deltas = append(deltas, FieldDelta{
					Spoke:      spokeField,
					Hub:        hubField,
					ChangeType: FieldChangeTypeShapeChangedToSecret,
				})
				continue
			}

			// TODO(a-hilaly) better function for equalizing shapes
			equalShapes, err := isEqualShape(spokeField.ShapeRef, hubField.ShapeRef)
			if err != nil {
				return nil, err
			}
			if equalShapes {
				// if the fields have equal names and types the change is intact
				deltas = append(deltas, FieldDelta{
					Spoke:      spokeField,
					Hub:        hubField,
					ChangeType: FieldChangeTypeIntact,
				})
				continue
			}

			// at this point we know that the fields kept the same name but have different
			// shapes
			deltas = append(deltas, FieldDelta{
				Spoke:      spokeField,
				Hub:        hubField,
				ChangeType: FieldChangeTypeIntact,
			})
			continue
		}

		// if a field is not found in the hubFieldsNames, there are three
		// possible changes: Removed, Added or Renamed.

		// First let's check if field was renamed
		newName, ok := renames[spokeFieldName]
		if ok {
			hubField, ok2 := hubCRDFields[newName]
			if !ok2 {
				// if a field was renamed and we can't find it in hubFieldsNames, something
				// very wrong happend during CRD loading.
				return nil, fmt.Errorf("cannot find renamed field %s " + newName)
			}

			// mark field as visited, both old and new names.
			visitedFields[newName] = nil
			visitedFields[spokeFieldName] = nil

			// this will mostlikely be always true, but let's double check.
			if newName == hubField.Names.Camel {
				// field was renamed
				deltas = append(deltas, FieldDelta{
					Spoke:      spokeField,
					Hub:        hubField,
					ChangeType: FieldChangeTypeRenamed,
				})
				continue
			}
			return nil, fmt.Errorf("renamed field unmatching: %v != %v", newName, hubField.Names.Camel)
		}

		// If the field was not renamed nor left intact nor it shape changed, it's
		// a removed field.
		deltas = append(deltas, FieldDelta{
			Spoke:      spokeField,
			Hub:        nil,
			ChangeType: FieldChangeTypeRemoved,
		})
	}

	// At this point we collected every type of change except added fields.
	// To find added fields we just look for fields that are in hubFieldsNames
	// and were not visited before (are not in spokeFieldsNames).
	for _, hubFieldName := range hubFieldsNames {
		_, visited := visitedFields[hubFieldName]
		if visited {
			continue
		}

		hubField, _ := hubCRDFields[hubFieldName]
		deltas = append(deltas, FieldDelta{
			Spoke:      nil,
			Hub:        hubField,
			ChangeType: FieldChangeTypeAdded,
		})
	}

	return deltas, nil
}

// isEqualShape returns whether two awssdkmodel.ShapeRef are equal or not.
// TODO(a-hilaly): this is very fragile - a simple docstring change will make the
// result wrong - we'll need to slowly verify each member/key/value
func isEqualShape(shapeRef1, shapeRef2 *awssdkmodel.ShapeRef) (bool, error) {
	b1, err := json.Marshal(shapeRef1.Shape)
	if err != nil {
		return false, err
	}

	b2, err := json.Marshal(shapeRef2.Shape)
	if err != nil {
		return false, err
	}

	return bytes.Equal(b1, b2), nil
}
