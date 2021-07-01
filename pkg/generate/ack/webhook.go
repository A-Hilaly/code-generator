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
		"GoCodeConvertTo": func(src *ackmodel.CRD, dst *ackmodel.CRD, hubImportPath string, sourceVarName string, targetVarName string, indentLevel int) string {
			return code.ConvertTo(src, dst, hubImportPath, sourceVarName, targetVarName, indentLevel)
		},
	}
)

// ConversionWebhooks returns a pointer to a TemplateSet containing all the templates
// for generating ACK service conversion and defaulting webhooks
func ConversionWebhooks(
	mvi *multiversion.MultiVersionInferrer,
	templateBasePaths []string,
) (*templateset.TemplateSet, error) {
	ts := templateset.New(
		templateBasePaths,
		webhooksIncludePaths,
		webhookCopyPaths,
		webhooksFuncMap,
	)

	hubVersion := mvi.GetHubVersion()
	hubInferrer, err := mvi.GetInferrer(hubVersion)
	if err != nil {
		return nil, err
	}

	hubMetaVars := hubInferrer.MetaVars()
	hubCRDs, err := hubInferrer.GetCRDs()
	if err != nil {
		return nil, err
	}

	for _, crd := range hubCRDs {
		convertVars := conversionVars{
			MetaVars:  hubMetaVars,
			SourceCRD: crd,
			IsHub:     true,
		}

		target := fmt.Sprintf("apis/%s/convert.go", hubVersion)
		if err = ts.Add(target, "apis/webhooks/conversion.go.tpl", convertVars); err != nil {
			return nil, err
		}
		fmt.Println("added tmp", target)
	}

	// Generate spoke version conversion functions
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

		for i, crd := range crds {
			/* 			if spokeVersion == "v1" {
				deltas, err := multiversion.ComputeCRDFieldsDeltas(crd, hubCRDs[i])
				if err != nil {
					return nil, err
				}
				fmt.Println("----\ndeltas:", len(deltas))
				for _, delta := range deltas {
					fmt.Println("changetype:", delta.ChangeType)
				}
				fmt.Println("----")
			} */

			convertVars := conversionVars{
				MetaVars:   metaVars,
				SourceCRD:  crd,
				DestCRD:    hubCRDs[i],
				IsHub:      false,
				HubVersion: hubVersion,
			}

			target := fmt.Sprintf("apis/%s/convert.go", spokeVersion)
			if err = ts.Add(target, "apis/webhooks/conversion.go.tpl", convertVars); err != nil {
				return nil, err
			}
			fmt.Println("added tmp", target)
		}
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
