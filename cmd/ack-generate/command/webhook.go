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

	"github.com/aws-controllers-k8s/code-generator/pkg/generate/ack"
	ackgenerate "github.com/aws-controllers-k8s/code-generator/pkg/generate/ack"
	"github.com/aws-controllers-k8s/code-generator/pkg/generate/multiversion"
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

	files, err := ioutil.ReadDir("../ecr-controller/apis")
	if err != nil {
		return err
	}

	apisInfos := map[string]multiversion.APIInfo{}
	for _, f := range files {
		metadata, err := ack.LoadGenerationMetadata("../ecr-controller/apis", f.Name())
		if err != nil {
			return err
		}
		apisInfos[f.Name()] = multiversion.APIInfo{
			IsDeprecated:        false,
			AWSSDKVersion:       metadata.AWSSDKGoVersion,
			GeneratorConfigPath: "../ecr-controller/apis/" + f.Name() + "/generator.yaml",
		}
	}

	mvi, err := multiversion.New(
		sdkDir,
		svcAlias,
		latestAPIVersion,
		apisInfos,
		ack.DefaultConfig,
	)
	if err != nil {
		return err
	}

	ts, err := ackgenerate.Webhooks(mvi, optTemplateDirs)
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
