package multiversion_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate/multiversion"
	"github.com/aws-controllers-k8s/code-generator/pkg/testutil"
)

func TestComputeFieldsDiff_ECR(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	type expectDelta struct {
		fieldName  string
		changeType string
	}
	type args struct {
		name         string
		hubVersion   string
		spokeVersion string
		renames      string
	}
	tests := []struct {
		name         string
		hubVersion   string
		spokeVersion string
		args         args
		want         expectDelta
		wantErr      bool
	}{
		{
			name:         "v1alpha1-v1alpha1: intact changes.",
			hubVersion:   "v1alpha1",
			spokeVersion: "v1alpha1",
		},
		{
			name: "v1alpha1-v1alpha3: renamed fields changes.",
		},
		{
			name: "v1alpha2-v1alpha3: renamed fields changes.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			hubInferrer := testutil.NewInferrerForServiceWithGeneratorConfig(t, "dynamodb", "")
			testInferrer := testutil.NewInferrerForServiceWithGeneratorConfig(t, "dynamodb", "")

			_, err := multiversion.ComputeFieldsDiff(
				tt.args.spokeCRDFields, tt.args.hubCRDFields, tt.args.renames,
			)
			if (err != nil) != tt.wantErr {
				assert.Fail(fmt.Sprintf("Manager.LoadRepository() error = %v, wantErr %v", err, tt.wantErr))
			}
		})
	}
}
