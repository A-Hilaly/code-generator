package main

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	k8sversion "k8s.io/apimachinery/pkg/version"
)

func main() {
	v := []string{
		"v1alpha1", "v1alpha2", "v1alpha3", "v1alpha6",
		"v1beta1", "v1beta2", "v1beta3",
		"v2",
		"v2alpha1", "v2alpha2", "v2beta3",
	}
	ag, _ := getAPIGroups(v)
	fmt.Println(ag)
}

type apiVersionGroup struct {
	ga     string
	alphas []string
	betas  []string
}

// getAPIGroups returns sorted lists of versionTypes
func getAPIGroups(versions []string) (map[int]apiVersionGroup, error) {
	sort.Slice(versions, func(i, j int) bool {
		return k8sversion.CompareKubeAwareVersionStrings(versions[i], versions[j]) < 0
	})

	apiGroups := map[int]apiVersionGroup{}
	for _, version := range versions {
		majorVersion, vType, _, _ := parseKubeVersion(version)
		ag := apiGroups[majorVersion]
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
		apiGroups[majorVersion] = ag
	}
	return apiGroups, nil
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
