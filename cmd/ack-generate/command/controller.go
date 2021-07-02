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
	"bufio"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	k8sversion "k8s.io/apimachinery/pkg/version"
)

var (
	cmdControllerPath string
	pkgResourcePath   string
)

// getLatestAPIVersion looks in a target output directory to determine what the
// latest Kubernetes API version for CRDs exposed by the generated service
// controller.
func getLatestAPIVersion() (string, error) {
	apisPath := filepath.Join(optOutputPath, "apis")
	versions := []string{}
	subdirs, err := ioutil.ReadDir(apisPath)
	if err != nil {
		return "", err
	}

	for _, subdir := range subdirs {
		versions = append(versions, subdir.Name())
	}
	sort.Slice(versions, func(i, j int) bool {
		return k8sversion.CompareKubeAwareVersionStrings(versions[i], versions[j]) < 0
	})
	return versions[len(versions)-1], nil
}

// FallBackFindServiceID reads through aws-sdk-go/models/apis/*/*/api-2.json
// Returns ServiceID (as newSuppliedAlias) if supplied service Alias matches with serviceID in api-2.json
// If not a match, return the supllied alias.
func FallBackFindServiceID(sdkDir, svcAlias string) (string, error) {
	basePath := filepath.Join(sdkDir, "models", "apis")
	var files []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return svcAlias, err
	}
	for _, file := range files {
		if strings.Contains(file, "api-2.json") {
			f, err := os.Open(file)
			if err != nil {
				return svcAlias, err
			}
			defer f.Close()
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				if strings.Contains(scanner.Text(), "serviceId") {
					getServiceID := strings.Split(scanner.Text(), ":")
					re := regexp.MustCompile(`[," \t]`)
					svcID := strings.ToLower(re.ReplaceAllString(getServiceID[1], ``))
					if svcAlias == svcID {
						getNewSvcAlias := strings.Split(file, string(os.PathSeparator))
						return getNewSvcAlias[len(getNewSvcAlias)-3], nil
					}
				}
			}
		}
	}
	return svcAlias, nil
}
