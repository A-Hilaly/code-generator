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
	"regexp"
	"sort"
	"strconv"

	k8sversion "k8s.io/apimachinery/pkg/version"
)

type apiSet struct {
	ga     string
	alphas []string
	betas  []string
}

// getAPISets returns
func getAPISets(versions []string) (map[int]apiSet, error) {
	sort.Slice(versions, func(i, j int) bool {
		return k8sversion.CompareKubeAwareVersionStrings(versions[i], versions[j]) < 0
	})

	apiSets := map[int]apiSet{}
	for _, version := range versions {
		majorVersion, vType, _, _ := parseKubeVersion(version)
		ag := apiSets[majorVersion]
		switch vType {
		case versionTypeAlpha:
			ag.alphas = append(ag.alphas, version)
		case versionTypeBeta:
			ag.betas = append(ag.betas, version)
		case versionTypeGA:
			ag.ga = version
		default:
			return nil, fmt.Errorf("cannot parse version %s", version)
		}
		apiSets[majorVersion] = ag
	}
	return apiSets, nil
}

type versionType int

const (
	versionTypeAlpha versionType = iota
	versionTypeBeta
	versionTypeGA
)

var kubeVersionRegex = regexp.MustCompile("^v([\\d]+)(?:(alpha|beta)([\\d]+))?$")

func parseKubeVersion(v string) (majorVersion int, vType versionType, minorVersion int, ok bool) {
	var err error
	submatches := kubeVersionRegex.FindStringSubmatch(v)
	if len(submatches) != 4 {
		return 0, 0, 0, false
	}
	switch submatches[2] {
	case "alpha":
		vType = versionTypeAlpha
	case "beta":
		vType = versionTypeBeta
	case "":
		vType = versionTypeGA
	default:
		return 0, 0, 0, false
	}
	if majorVersion, err = strconv.Atoi(submatches[1]); err != nil {
		return 0, 0, 0, false
	}
	if vType != versionTypeGA {
		if minorVersion, err = strconv.Atoi(submatches[3]); err != nil {
			return 0, 0, 0, false
		}
	}
	return majorVersion, vType, minorVersion, true
}
