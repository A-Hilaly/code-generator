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
	"errors"
	"fmt"
	"sort"

	"gopkg.in/src-d/go-git.v4"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate"
	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
	"github.com/aws-controllers-k8s/code-generator/pkg/util"
)

var (
	ErrIllegalDeprecation   = errors.New("illegal deprecation")
	ErrAPIVersionNotFound   = errors.New("apiVersion not found")
	ErrAPIVersionDeprecated = errors.New("apiVersion is deprecated")
	ErrEmptyAPIMapper       = errors.New("empty apis mapper")
)

// APIInfo contains information related a specific apiVersion.
type APIInfo struct {
	// Whether this API is deprecated or not. Deprecating a version
	// prevents the code generator from generating webhooks for it.
	IsDeprecated bool
	// the aws-sdk-go version used to generated the apiVersion.
	AWSSDKVersion string
	// Full path of the generator config file.
	GeneratorConfigPath string
}

// Inferrer is a multi-version aware inferrer. It is containing the mapping
// of each non-deprecated version with their correspending inferrer and APIInfos.
type Inferrer struct {
	gitRepo *git.Repository

	hubVersion         string
	spokeVersions      []string
	deprecatedVersions []string

	apiInfos         map[string]APIInfo
	inferrersMapping map[string]*generate.Inferrer
}

// NewInferrer returns a new Inferrer struct.
func NewInferrer(
	sdkCacheDir string,
	serviceAlias string,
	hubVersion string,
	apisInfo map[string]APIInfo,
	defaultConfig ackgenconfig.Config,
) (*Inferrer, error) {
	if len(apisInfo) == 0 {
		return nil, ErrEmptyAPIMapper
	}

	spokeVersions := make([]string, 0, len(apisInfo)-1)
	gitRepo, err := util.LoadRepository(sdkCacheDir)
	if err != nil {
		return nil, fmt.Errorf("cannot read sdk git repository: %v", err)
	}

	// create inferrer for each non-deprecated api version
	inferrersMapping := make(map[string]*generate.Inferrer, len(apisInfo))
	deprecatedVersions := []string{}
	for apiVersion, apiInfo := range apisInfo {
		if apiInfo.IsDeprecated {
			deprecatedVersions = append(deprecatedVersions, apiVersion)
		}
		if apiVersion != hubVersion {
			spokeVersions = append(spokeVersions, apiVersion)
		}

		SDKAPI, err := ackmodel.LoadSDKAPI(gitRepo, sdkCacheDir, serviceAlias, apiInfo.AWSSDKVersion)
		if err != nil {
			return nil, fmt.Errorf("cannot load repository SDKAPI: %v", err)
		}

		i, err := generate.New(
			SDKAPI,
			apiVersion,
			apiInfo.GeneratorConfigPath,
			defaultConfig,
		)
		if err != nil {
			return nil, fmt.Errorf("cannot create inferrer for apiVersion %s: %v", apiVersion, err)
		}
		inferrersMapping[apiVersion] = i
	}

	sort.Strings(spokeVersions)
	sort.Strings(deprecatedVersions)

	if err := auditDeprecations(hubVersion, spokeVersions, deprecatedVersions); err != nil {
		return nil, err
	}

	return &Inferrer{
		gitRepo:            gitRepo,
		hubVersion:         hubVersion,
		spokeVersions:      spokeVersions,
		deprecatedVersions: deprecatedVersions,
		apiInfos:           apisInfo,
		inferrersMapping:   inferrersMapping,
	}, nil
}

// GetInferrer returns the inferrer of a given api version.
func (i *Inferrer) GetInferrer(apiVersion string) (*generate.Inferrer, error) {
	if err := i.VerifyAPIVersions(apiVersion); err != nil {
		return nil, fmt.Errorf("cannot verify apiVersions %s: %v", apiVersion, err)
	}
	return i.inferrersMapping[apiVersion], nil
}

// GetSpokeVersions returns the spokes versions list.
func (i *Inferrer) GetSpokeVersions() []string {
	return i.spokeVersions
}

// GetHubVersion returns the hub version.
func (i *Inferrer) GetHubVersion() string {
	return i.hubVersion
}

// CompareHubWith compares a given api version with the hub version and returns
// slices of FieldDeltas representing the diff between CRDs status and spec fields.
func (i *Inferrer) CompareHubWith(apiVersion string) (map[string]*CRDDelta, error) {
	return i.CompareAPIVersions(apiVersion, i.hubVersion)
}

// CompareAPIVersions compares two api versions and returns a slice of FieldDeltas
// representing the diff between CRDs status and spec fields.
func (i *Inferrer) CompareAPIVersions(srcAPIVersion, dstAPIVersion string) (
	map[string]*CRDDelta,
	error,
) {
	if srcAPIVersion == dstAPIVersion {
		return nil, fmt.Errorf("cannot compare an apiVersion with it self")
	}

	// get source CRDs
	srcInferrer, err := i.GetInferrer(srcAPIVersion)
	if err != nil {
		return nil, err
	}
	srcCRDs, err := generate.GetCRDs(srcInferrer.SDKAPI, srcInferrer.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("error getting crds for %s: %v", srcAPIVersion, err)
	}

	// get destination crds
	dstInferrer, err := i.GetInferrer(dstAPIVersion)
	if err != nil {
		return nil, err
	}
	dstCRDs, err := generate.GetCRDs(dstInferrer.SDKAPI, dstInferrer.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("error getting crds for %s: %v", dstAPIVersion, err)
	}

	// compute FieldDeltas for each CRD
	apiDeltas := make(map[string]*CRDDelta)
	if len(srcCRDs) != len(dstCRDs) {
		// TODO(a-hilaly) handle added/removed CRDs
		return nil, fmt.Errorf("source and destination apiVersions don't have the same number of CRDs")
	}
	for i, crd := range dstCRDs {
		crdDelta, err := ComputeCRDFieldsDeltas(srcCRDs[i], dstCRDs[i])
		if err != nil {
			return nil, fmt.Errorf("cannot compute crd field deltas: %v", err)
		}
		apiDeltas[crd.Names.Camel] = crdDelta
	}
	return apiDeltas, nil
}

// VerifyAPIVersions verifies that an API version exists and is not deprecated.
func (i *Inferrer) VerifyAPIVersions(apiVersions ...string) error {
	for _, apiVersion := range apiVersions {
		apiInfo, ok := i.apiInfos[apiVersion]
		if !ok {
			return fmt.Errorf("%v: %s", ErrAPIVersionNotFound, apiVersion)
		}
		if apiInfo.IsDeprecated {
			return fmt.Errorf("%v: %s", ErrAPIVersionDeprecated, apiVersion)
		}
	}
	return nil
}

// auditDeprecations verifies that the list of deprecations doesn't break any of the
// kubernetes deprecation policies.
func auditDeprecations(
	hubVersion string,
	spokeVersions []string,
	deprecatedVersions []string,
) error {
	// First we can not deprecated the hub version.
	if util.InStrings(hubVersion, deprecatedVersions) {
		return fmt.Errorf("%v: %s", ErrIllegalDeprecation, hubVersion)
	}

	// TODO(a-hilaly): make sure that deprecation is incremental. For example you can't
	// deprecate v1alpha2 without deprecating v1alpha1
	return nil
}
