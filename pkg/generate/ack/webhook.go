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

package ack

import (
	"fmt"
	ttpl "text/template"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate/code"
	"github.com/aws-controllers-k8s/code-generator/pkg/generate/multiversion"
	"github.com/aws-controllers-k8s/code-generator/pkg/generate/templateset"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
)

var (
	webhooksIncludePaths = []string{
		"boilerplate.go.tpl",
		"apis/webhooks/conversion.go.tpl",
	}
	webhookCopyPaths = []string{}
	webhooksFuncMap  = ttpl.FuncMap{
		"GoCodeConvertTo": func(src *ackmodel.CRD, dst *ackmodel.CRD, sourceVarName string, targetVarName string, indentLevel int) string {
			return code.ConvertTo(src, dst, sourceVarName, targetVarName, indentLevel)
		},
	}
)

// Webhooks returns a pointer to a TemplateSet containing all the templates
// for generating ACK service conversion and defaulting webhooks
func Webhooks(
	mvi *multiversion.MultiVersionInferrer,
	templateBasePaths []string,
) (*templateset.TemplateSet, error) {
	fmt.Println("called webhook")

	ts := templateset.New(
		templateBasePaths,
		webhooksIncludePaths,
		webhookCopyPaths,
		webhooksFuncMap,
	)

	hubVersion := mvi.GetHubVersion()

	for _, spokeVersion := range mvi.GetSpokeVersions() {
		inferrer, err := mvi.GetInferrer(spokeVersion)
		if err != nil {
			return nil, err
		}

		metaVars := inferrer.MetaVars()
		crds, err := inferrer.GetCRDs()
		if err != nil {
			return nil, err
		}

		for _, crd := range crds {
			convertVars := conversionVars{
				MetaVars:   metaVars,
				SourceCRD:  crd,
				DestCRD:    crd,
				IsHub:      false,
				HubVersion: hubVersion,
			}

			target := fmt.Sprintf("apis/%s/convert.go", hubVersion)
			if err = ts.Add(target, "apis/webhooks/conversion.go.tpl", convertVars); err != nil {
				return nil, err
			}
			fmt.Println("YEY")
		}
		fmt.Println("YEYY")
	}

	return ts, nil
}

type conversionVars struct {
	templateset.MetaVars
	SourceCRD  *ackmodel.CRD
	DestCRD    *ackmodel.CRD
	HubVersion string
	IsHub      bool
}
