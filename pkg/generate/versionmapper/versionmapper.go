package versionmapper

import (
	"fmt"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate"
)

type apiInfo struct {
	sdkAPI     string
	awsSDKGo   string
	apiVersion string
}

type versionMapper struct {
	serviceAlias string

	hubAPI string

	spokesAPIs map[string]*generate.Inferrer
}

func (vm *versionMapper) WithHub(apiVersion string) *versionMapper {
	vm.hubAPI = apiVersion
	return vm
}

func New(hubVersion string, spokesVersion []string, apiDirectory string) {

}

func (vm *versionMapper) CompareHubWith(apiVersion string) ([]FieldDelta, error) {
	if apiVersion == vm.hubAPI {
		return nil, fmt.Errorf("cannot compare the hub api version with it self")
	}

	return nil, nil
}
