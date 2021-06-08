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

package command

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aws-controllers-k8s/code-generator/pkg/generate"
	ackgenerate "github.com/aws-controllers-k8s/code-generator/pkg/generate/ack"
	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
)

var ()

var webhooksCmd = &cobra.Command{
	Use:   "webhooks <service>",
	Short: "Generates Go files containing ",
	RunE:  generateWebhooks,
}

func init() {
	rootCmd.AddCommand(webhooksCmd)
}

// generateWebhooks generates the Go files for a service controller
func generateWebhooks(cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("please specify the service alias for the AWS service API to generate")
	}
	svcAlias := strings.ToLower(args[0])
	if optOutputPath == "" {
		optOutputPath = filepath.Join(optServicesDir, svcAlias)
	}

	if err := ensureSDKRepo(optCacheDir, ""); err != nil {
		return err
	}
	sdkHelper := ackmodel.NewSDKHelper(sdkDir)
	sdkAPI, err := sdkHelper.API(svcAlias)
	if err != nil {
		newSvcAlias, err := FallBackFindServiceID(sdkDir, svcAlias)
		if err != nil {
			return err
		}
		sdkAPI, err = sdkHelper.API(newSvcAlias) // retry with serviceID
		if err != nil {
			return fmt.Errorf("service %s not found", svcAlias)
		}
	}
	latestAPIVersion, err = getLatestAPIVersion()
	if err != nil {
		return err
	}

	// read versions

	// read aws-sdk-go versions

	// read generator.yamls

	// determine hub (default is latest - by can be overidable)

	cfg, err := ackgenconfig.New(optGeneratorConfigPath, ackgenerate.DefaultConfig)
	if err != nil {
		return err
	}

	APIs := map[string]*generate.ACKAPI{
		"v1alpha1": generate.NewACKAPI(true, "v1alpha1", &cfg, sdkAPI),
	}

	g := generate.NewGenerator(
		"-", "v1alpha1", nil, APIs,
	)

	if err != nil {
		return err
	}
	ts, err := ackgenerate.Webhooks(g, optTemplateDirs)
	if err != nil {
		return err
	}

	if err = ts.Execute(); err != nil {
		return err
	}

	for path, contents := range ts.Executed() {
		if optDryRun {
			fmt.Printf("============================= %s ======================================\n", path)
			fmt.Println(strings.TrimSpace(contents.String()))
			continue
		}
		outPath := filepath.Join(optOutputPath, path)
		outDir := filepath.Dir(outPath)
		if _, err := ensureDir(outDir); err != nil {
			return err
		}
		if err = ioutil.WriteFile(outPath, contents.Bytes(), 0666); err != nil {
			return err
		}
	}
	return nil
}
