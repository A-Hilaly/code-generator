package multiversion

// TODO(a-hilaly) move this file outside of pkg/generate/multiversion. Ideally we
// Should be able to APIStatus and APIInfo to prevent regenerating removed or
// deprecated APIs.
// TODO(a-hilaly) store API status in `ack-generate-metadata.yaml`

type APIStatus string

const (
	APIStatusUnknown    APIStatus = "unknown"
	APIStatusAvailable            = "available"
	APIStatusRemoved              = "removed"
	APIStatusDeprecated           = "deprecated"
)

// APIInfo contains information related a specific apiVersion.
type APIInfo struct {
	// The API status. Can be one of Available, Removed and Deprecated.
	Status APIStatus
	// the aws-sdk-go version used to generated the apiVersion.
	AWSSDKVersion string
	// Full path of the generator config file.
	GeneratorConfigPath string
}
