package crd

import (
	ackgenconfig "github.com/aws-controllers-k8s/code-generator/pkg/generate/config"
	ackmodel "github.com/aws-controllers-k8s/code-generator/pkg/model"
)

type ACKAPI struct {
	isHub       bool
	apiVersion  string
	cfg         *ackgenconfig.Config
	SDKAPI      *ackmodel.SDKAPI
	crds        []*ackmodel.CRD
	typeImports map[string]string
	typeRenames map[string]string
}

type VersionedAPIs struct {
	hubVersion string
	spokesAPIs []string
	APIS       map[string]*ACKAPI
}
