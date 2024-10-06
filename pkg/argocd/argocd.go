package argocd

import "github.com/mgp-systems/internal-fabrik8-api/internal/argocd"

//nolint:gochecknoglobals
var (
	ArgocdSecretClient         = argocd.ArgocdSecretClient
	GetArgocdTokenV2           = argocd.GetArgocdTokenV2
	GetArgoCDApplicationObject = argocd.GetArgoCDApplicationObject
	RefreshApplication         = argocd.RefreshApplication
	RefreshRegistryApplication = argocd.RefreshRegistryApplication
)
