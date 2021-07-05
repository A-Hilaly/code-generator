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
	"strings"

	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	"github.com/aws-controllers-k8s/code-generator/pkg/generate/templateset"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
)

// NewInferrer returns a new Inferrer struct for a supplied API model.
// Optionally, pass a file path to a generator config file that can be used to
// instruct the code generator how to handle the API properly
func NewInferrer(
	SDKAPI *ackmodel.SDKAPI,
	apiVersion string,
	configPath string,
	defaultConfig ackgenconfig.Config,
) (*Inferrer, error) {
	cfg, err := ackgenconfig.New(configPath, defaultConfig)
	if err != nil {
		return nil, err
	}
	g := &Inferrer{
		SDKAPI:       SDKAPI,
		serviceAlias: SDKAPI.ServiceID(),
		apiVersion:   apiVersion,
		cfg:          &cfg,
	}
	ackmodel.ApplyShapeIgnoreRules(SDKAPI, &cfg)
	return g, nil
}

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
func (i *Inferrer) MetaVars() templateset.MetaVars {
	return templateset.MetaVars{
		ServiceAlias:            i.serviceAlias,
		ServiceID:               i.SDKAPI.ServiceID(),
		ServiceIDClean:          i.SDKAPI.ServiceIDClean(),
		APIGroup:                i.SDKAPI.APIGroup(),
		APIVersion:              i.apiVersion,
		SDKAPIInterfaceTypeName: i.SDKAPI.SDKAPIInterfaceTypeName(),
		CRDNames:                i.crdNames(),
	}
}

// crdNames returns all crd names lowercased and in plural
func (i *Inferrer) crdNames() []string {
	var crdConfigs []string

	crds, _ := i.GetCRDs()
	for _, crd := range crds {
		crdConfigs = append(crdConfigs, strings.ToLower(crd.Plural))
	}

	return crdConfigs
}

// GetCRDs returns a slice of `ackmodel.CRD` structs that describe the
// top-level resources discovered by the code generator for an AWS service API
func (i *Inferrer) GetCRDs() ([]*ackmodel.CRD, error) {
	if i.crds != nil {
		return i.crds, nil
	}

	crds, err := ackmodel.GetCRDs(i.SDKAPI, i.cfg)
	if err != nil {
		return nil, err
	}
	i.crds = crds
	return crds, nil
}

// GetTypeDefs returns a slice of `ackmodel.TypeDef` pointers
func (i *Inferrer) GetTypeDefs() ([]*ackmodel.TypeDef, error) {
	if i.typeDefs != nil {
		return i.typeDefs, nil
	}

	crds, _ := i.GetCRDs()
	tdefs, trenames, err := ackmodel.GetTypeDefs(i.SDKAPI, crds, i.cfg)
	if err != nil {
		return nil, err
	}
	i.typeDefs = tdefs
	i.typeRenames = trenames
	return tdefs, nil
}

// GetEnumDefs returns a slice of pointers to `ackmodel.EnumDef` structs which
// represent string fields whose value is constrained to one or more specific
// string values.
func (i *Inferrer) GetEnumDefs() ([]*ackmodel.EnumDef, error) {
	return ackmodel.GetEnumDefs(i.SDKAPI, i.cfg)
}

// GetConfig returns the configuration option used to define the current
// generator.
func (i *Inferrer) GetConfig() *ackgenconfig.Config {
	return i.cfg
}
