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
	"fmt"

	"gopkg.in/src-d/go-git.v4"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate"
	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
	"github.com/aws-controllers-k8s/code-generator/pkg/util"
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

// Inferrer .
type Inferrer struct {
	gitRepo *git.Repository

	hubVersion    string
	spokeVersions []string

	apiInfos         map[string]APIInfo
	inferrersMapping map[string]*generate.Inferrer
}

// NewInferrer .
func NewInferrer(
	sdkCacheDir string,
	serviceAlias string,
	hubVersion string,
	apisInfo map[string]APIInfo,
	defaultConfig ackgenconfig.Config,
) (*Inferrer, error) {
	spokeVersions := make([]string, 0, len(apisInfo)-1)
	gitRepo, err := util.LoadRepository(sdkCacheDir)
	if err != nil {
		return nil, fmt.Errorf("cannot load sdk repository: %v", err)
	}

	// create inferrer for each non-deprecated api version
	inferrersMapping := make(map[string]*generate.Inferrer, len(apisInfo))
	for apiVersion, apiInfo := range apisInfo {
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

	i := &Inferrer{
		gitRepo:          gitRepo,
		hubVersion:       hubVersion,
		spokeVersions:    spokeVersions,
		inferrersMapping: inferrersMapping,
		apiInfos:         apisInfo,
	}

	return i, nil
}

func (i *Inferrer) GetInferrer(apiVersion string) (*generate.Inferrer, error) {
	if err := i.VerifyAPIVersions(apiVersion); err != nil {
		return nil, fmt.Errorf("cannot verify apiVersions: %v", err)
	}
	return i.inferrersMapping[apiVersion], nil
}

func (i *Inferrer) GetSpokeVersions() []string {
	return i.spokeVersions
}

func (i *Inferrer) GetHubVersion() string {
	return i.hubVersion
}

func (i *Inferrer) CompareHubWith(apiVersion string) ([]FieldDelta, error) {
	return i.CompareAPIVersions(apiVersion, i.hubVersion)
}

func (i *Inferrer) CompareAPIVersions(apiVersion1, apiVersion2 string) ([]FieldDelta, error) {
	if apiVersion1 == apiVersion2 {
		return nil, fmt.Errorf("cannot compare an apiVersion with it self")
	}

	err := i.VerifyAPIVersions(apiVersion1, apiVersion2)
	if err != nil {
		return nil, fmt.Errorf("cannot verify apiVersions: %v", err)
	}

	return nil, nil
}

func (i *Inferrer) VerifyAPIVersions(apiVersions ...string) error {
	for _, apiVersion := range apiVersions {
		apiInfo, ok := i.apiInfos[apiVersion]
		if !ok {
			return fmt.Errorf("cannot find apiVersion %s", apiVersion)
		}
		if apiInfo.IsDeprecated {
			return fmt.Errorf("cannot use a deprecated apiVersion %s", apiVersion)
		}
	}
	return nil
}
