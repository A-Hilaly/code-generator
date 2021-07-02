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
	"encoding/json"
	"fmt"
	"sort"

	awssdkmodel "github.com/aws/aws-sdk-go/private/model/api"

	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
)

// ChangeType represent the type of field modification.
type ChangeType string

const (
	// ChangeTypeUnknown is used when ChangeType cannot be computed.
	ChangeTypeUnknown ChangeType = "unknown"
	// ChangeTypeIntact is used a field name and structure didn't change.
	ChangeTypeIntact ChangeType = "intact"
	// ChangeTypeAdded is used when a new field is introduced in a CRD.
	ChangeTypeAdded ChangeType = "added"
	// ChangeTypeRemoved is used a when a field is removed from a CRD.
	ChangeTypeRemoved ChangeType = "removed"
	// ChangeTypeRenamed is used when a field is renamed.
	ChangeTypeRenamed ChangeType = "renamed"
	// ChangeTypeShapeChanged is used when a field shape has changed.
	ChangeTypeShapeChanged ChangeType = "shape-changed"
	// ChangeTypeShapeChangedToSecret is used when a field change to
	// a k8s secret type.
	ChangeTypeShapeChangedToSecret ChangeType = "shape-changed-to-secret"
)

// FieldDelta represent the delta between the same field in two different
// CRD versions.
type FieldDelta struct {
	Spoke      *ackmodel.Field
	Hub        *ackmodel.Field
	ChangeType ChangeType
}

// ComputeCRDFieldsDeltas compares two ackmodel.CRD instances and returns the
// spec and status fields deltas. spokeCRD is the CRD of the spoke (source) version
// and hubCRD is the CRD of the hub (destination) version.
func ComputeCRDFieldsDeltas(spokeCRD, hubCRD *ackmodel.CRD) ([]FieldDelta, []FieldDelta, error) {
	renames, _, err := hubCRD.GetResourceFieldRenames()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get resource field renames: %s", err)
	}

	specDeltas, err := ComputeFieldsDiff(spokeCRD.SpecFields, hubCRD.SpecFields, renames)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot compute spec fields deltas: %s", err)
	}

	statusDeltas, err := ComputeFieldsDiff(spokeCRD.StatusFields, hubCRD.StatusFields, renames)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot compute status fields deltas: %s", err)
	}

	return specDeltas, statusDeltas, nil
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

	// collect field names and sort them for determenistic output.
	spokeFieldsNames := []string{}
	for name := range spokeCRDFields {
		spokeFieldsNames = append(spokeFieldsNames, name)
	}
	hubFieldsNames := []string{}
	for name := range hubCRDFields {
		hubFieldsNames = append(hubFieldsNames, name)
	}
	sort.Strings(spokeFieldsNames)
	sort.Strings(hubFieldsNames)

	// let's make sure we don't visit fields more than once - especially
	// when fields are renamed.
	visitedFields := map[string]*struct{}{}

	// first let's loop over the first fieldNames array and see if can find
	// the same field name in fieldNames2.
	for _, spokeFieldName := range spokeFieldsNames {
		spokeField, _ := spokeCRDFields[spokeFieldName]
		hubField, ok := hubCRDFields[spokeFieldName]
		// If a field is found in both arrays only three changes are possible:
		// Intact, TypeChangeChange and ChangeTypeShapeChangedToSecret.
		// NOTE(a-hilaly): carefull about X -> Y then Z -> X renames. It should
		// not be allowed.
		if ok {
			visitedFields[spokeFieldName] = nil
			// check if field became secret
			if (spokeField.FieldConfig == nil ||
				(spokeField.FieldConfig != nil && !spokeField.FieldConfig.IsSecret)) &&
				(hubField.FieldConfig != nil && hubField.FieldConfig.IsSecret) {
				deltas = append(deltas, FieldDelta{
					Spoke:      spokeField,
					Hub:        hubField,
					ChangeType: ChangeTypeShapeChangedToSecret,
				})
				continue
			}

			// TODO(a-hilaly) better function for equalizing shapes
			equalShapes, err := isEqualShape(spokeField.ShapeRef, hubField.ShapeRef)
			if err != nil {
				return nil, err
			}
			if equalShapes {
				// the fields have equal names and types the change is intact
				deltas = append(deltas, FieldDelta{
					Spoke:      spokeField,
					Hub:        hubField,
					ChangeType: ChangeTypeIntact,
				})
				continue
			}
		}

		// if a field is not found in the second array (fieldNames2), there are two
		// possible changes: Removed or Renamed.

		// First let's check if field was renamed
		newName, ok := renames[spokeFieldName]
		if ok {

			hubField, ok2 := hubCRDFields[newName]
			if !ok2 {
				// if a field was renamed and is not in the fieldNames2 array
				// something very wrong happend during CRD loading.
				// TODO(a-hilaly): Maybe panic?
				s := ""
				for i := range hubCRDFields {
					s += i + "/"
				}
				return nil, fmt.Errorf("cannot find renamed field %s "+newName, s)
			}

			// mark field as visited, both old and new names.
			visitedFields[newName] = nil
			visitedFields[spokeFieldName] = nil

			if newName == hubField.Names.Camel {
				// field was renamed
				deltas = append(deltas, FieldDelta{
					Spoke:      spokeField,
					Hub:        hubField,
					ChangeType: ChangeTypeRenamed,
				})
				continue
			}

			// TODO(a-hilaly): Maybe panic?
			return nil, fmt.Errorf("renamed field unmatching: %v != %v", newName, hubField.Names.Camel)
		}

		// Out field was not renamed nor type-modified not intact, so it's
		// a removed field.
		deltas = append(deltas, FieldDelta{
			Spoke:      spokeField,
			Hub:        nil,
			ChangeType: ChangeTypeRemoved,
		})
	}

	// At this point we collected every type of change except added fields.
	// To find added fields we just look for fields that are in fieldNames2
	// and were not visited before.
	for _, hubFieldName := range hubFieldsNames {
		// skip visited names
		_, visited := visitedFields[hubFieldName]
		if visited {
			continue
		}

		hubField, _ := hubCRDFields[hubFieldName]
		deltas = append(deltas, FieldDelta{
			Spoke:      nil,
			Hub:        hubField,
			ChangeType: ChangeTypeAdded,
		})
	}

	return deltas, nil
}

// TODO(a-hilaly): this is very fragile - a simple docstring change will make the
// result wrong
// we'll need to slowly verify each member/key/value
func isEqualShape(shapeRef1, shapeRef2 *awssdkmodel.ShapeRef) (bool, error) {
	j1, err := json.Marshal(shapeRef1.Shape)
	if err != nil {
		return false, err
	}

	j2, err := json.Marshal(shapeRef2.Shape)
	if err != nil {
		return false, err
	}

	return string(j1) == string(j2), nil
}
