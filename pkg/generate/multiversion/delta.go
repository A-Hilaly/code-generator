package multiversion

import (
	"encoding/json"
	"fmt"

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

type FieldDelta struct {
	From       *ackmodel.Field
	To         *ackmodel.Field
	ChangeType ChangeType
}

// ComputeCRDFieldsDeltas .
// crd1 is the spoke version
// crd2 is the hub version
// sometimes convertions are impossible and will need to do some deprecations.
// we are assuming that resource name doesn't change.
func ComputeCRDFieldsDeltas(crd1, crd2 *ackmodel.CRD) ([]FieldDelta, error) {
	deltas := make([]FieldDelta, 0, len(crd2.SpecFields)+len(crd2.StatusFields))

	// if same aws-sdk-go and same generator.yaml return all intact

	// visitedFields := map[string]struct{}{}

	for _, specField1Name := range crd1.SpecFieldNames() {
		specField1, _ := crd1.SpecFields[specField1Name]
		specField2, ok := crd2.SpecFields[specField1Name]
		// if field name stayed the same
		// NOTE: carefull about A -> B then C -> A renames ?
		if ok {
			if !specField1.FieldConfig.IsSecret && specField2.FieldConfig.IsSecret {
				// field changed to secret
				deltas = append(deltas, FieldDelta{
					From:       specField1,
					To:         specField2,
					ChangeType: ChangeTypeShapeChangedToSecret,
				})
				continue
			}

			equalShapes, err := isEqualShape(specField1.ShapeRef, specField2.ShapeRef)
			if err != nil {
				return nil, err
			}
			if equalShapes {
				// the fields have equal names and types so change is intact
				deltas = append(deltas, FieldDelta{
					From:       specField1,
					To:         specField2,
					ChangeType: ChangeTypeIntact,
				})
				continue
			}
		}

		// if not OK then field must have been deleted or renamed
		// let's check if it was renamed first
		// loop over Operation configs and look for renames
		oldToNewRenames2, _, err := crd2.GetResourceFieldRenames()
		if err != nil {
			return nil, err
		}

		newName, ok := oldToNewRenames2[specField1Name]
		if ok {
			if newName == specField2.Names.Camel {
				deltas = append(deltas, FieldDelta{
					From:       specField1,
					To:         specField2,
					ChangeType: ChangeTypeRenamed,
				})
				continue
			}
			panic(fmt.Sprintf("renamed field unmatching: %v != %v", newName, specField2.Names.Camel))
		}

		// field probably deleted
		deltas = append(deltas, FieldDelta{
			From:       specField1,
			To:         nil,
			ChangeType: ChangeTypeRemoved,
		})
	}

	return nil, nil
}

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
