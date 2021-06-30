package multiversion

import (
	"fmt"

	"gopkg.in/src-d/go-git.v4"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate"
	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
	"github.com/aws-controllers-k8s/code-generator/pkg/util"
)

type APIInfo struct {
	IsDeprecated        bool
	AWSSDKVersion       string
	GeneratorConfigPath string
}

type MultiVersionInferrer struct {
	gitRepo *git.Repository

	hubVersion    string
	spokeVersions []string

	apiInfos         map[string]APIInfo
	inferrersMapping map[string]*generate.Inferrer
}

func (mvi *MultiVersionInferrer) WithHub(apiVersion string) *MultiVersionInferrer {
	mvi.hubVersion = apiVersion
	return mvi
}

func New(
	sdkCacheDir string,
	serviceAlias string,
	hubVersion string,
	apisInfo map[string]APIInfo,
	defaultConfig ackgenconfig.Config,
) (*MultiVersionInferrer, error) {

	// compute the list of spokes versions
	spokeVersions := make([]string, 0, len(apisInfo)-1)

	rg, err := util.LoadRepository(sdkCacheDir)
	if err != nil {
		return nil, fmt.Errorf("cannot load sdk cache: %v", err)
	}

	inferrersMapping := make(map[string]*generate.Inferrer, len(apisInfo))
	for apiVersion, apiInfo := range apisInfo {
		if apiVersion != hubVersion {
			spokeVersions = append(spokeVersions, apiVersion)
		}

		SDKAPI, err := ackmodel.LoadSDKAPI(rg, sdkCacheDir, serviceAlias, apiInfo.AWSSDKVersion)
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

	mvi := &MultiVersionInferrer{
		gitRepo:          rg,
		hubVersion:       hubVersion,
		spokeVersions:    spokeVersions,
		inferrersMapping: inferrersMapping,
	}
	return mvi, nil
}

func (mvi *MultiVersionInferrer) GetInferrer(apiVersion string) (*generate.Inferrer, error) {
	if err := mvi.VerifyAPIVersions(apiVersion); err != nil {
		return nil, fmt.Errorf("cannot verify apiVersions: %v", err)
	}
	return mvi.inferrersMapping[apiVersion], nil
}

func (mvi *MultiVersionInferrer) GetSpokeVersions() []string {
	return mvi.spokeVersions
}

func (mvi *MultiVersionInferrer) GetHubVersion() string {
	return mvi.hubVersion
}

func (mvi *MultiVersionInferrer) CompareHubWith(apiVersion string) ([]FieldDelta, error) {
	return mvi.CompareAPIVersions(apiVersion, mvi.hubVersion)
}

func (mvi *MultiVersionInferrer) CompareAPIVersions(apiVersion1, apiVersion2 string) ([]FieldDelta, error) {
	if apiVersion1 == apiVersion2 {
		return nil, fmt.Errorf("cannot compare an apiVersion with it self")
	}

	err := mvi.VerifyAPIVersions(apiVersion1, apiVersion2)
	if err != nil {
		return nil, fmt.Errorf("cannot verify apiVersions: %v", err)
	}

	return nil, nil
}

func (mvi *MultiVersionInferrer) VerifyAPIVersions(apiVersions ...string) error {
	for _, apiVersion := range apiVersions {
		apiInfo, ok := mvi.apiInfos[apiVersion]
		if !ok {
			return fmt.Errorf("cannot find apiVersion %s", apiVersion)
		}
		if apiInfo.IsDeprecated {
			return fmt.Errorf("cannot use a deprecated apiVersion %s", apiVersion)
		}
	}
	return nil
}
