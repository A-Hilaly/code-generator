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

	"github.com/spf13/cobra"

	"github.com/aws-controllers-k8s/code-generator/pkg/version"
)

const debugHeader = `Date: %s
Build: %s
Version: %s
Git Hash: %s
`

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of " + appName,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf(debugHeader, version.BuildDate, version.GoVersion, version.Version, version.BuildHash)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
