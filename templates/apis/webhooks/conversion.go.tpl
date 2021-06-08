{{- template "boilerplate" }}

package {{ .APIVersion }}

import (
   ctrlrtconversion "sigs.k8s.io/controller-runtime/pkg/conversion"
)


{{- if .IsHub }}
// Hub marks this type as conversion hub.
func (*{{ .SourceCRD.Kind }}) Hub() {}
{{ else }}

// ConvertTo converts this {{ .SourceCRD.Kind }} to the Hub version ({{ .HubVersion }}).
func (src *{{ .SourceCRD.Kind }}) ConvertTo(dstRaw ctrlrtconversion.Hub) error {
{{ GoCodeConvertTo .SourceCRD .DestCRD "src" "dstRaw" 1}}
}

{{- end }}

func (r *{{ .SourceCRD.Kind }}) SetupWebhookWithManager(mgr ctrl.Manager) error {
    return ctrl.NewWebhookManagedBy(mgr).
        For(r).
        Complete()
}