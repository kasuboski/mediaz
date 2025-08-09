# mediaz

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.0.1](https://img.shields.io/badge/AppVersion-0.0.1-informational?style=flat-square)

A Helm chart for Mediaz

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| affinity | object | `{}` |  |
| config.filePath | string | `"/app/config.yaml"` |  |
| config.library.downloadMountDir | string | `"/downloads"` |  |
| config.library.movie | string | `"/movies"` |  |
| config.library.tv | string | `"/tv"` |  |
| config.server.port | int | `8080` |  |
| fullnameOverride | string | `""` |  |
| httproute.annotations | object | `{}` |  |
| httproute.enabled | bool | `false` |  |
| httproute.hostnames[0] | string | `"mediaz.local"` |  |
| httproute.labels | object | `{}` |  |
| httproute.parentRefs[0].name | string | `"gateway"` |  |
| httproute.parentRefs[0].namespace | string | `"envoy-gateway-system"` |  |
| httproute.rules.backendRefs[0].name | string | `"mediaz"` |  |
| httproute.rules.backendRefs[0].port | int | `8080` |  |
| image.pullPolicy | string | `"IfNotPresent"` |  |
| image.repository | string | `"ghcr.io/kasuboski/mediaz"` |  |
| image.tag | string | `""` |  |
| imagePullSecrets | list | `[]` |  |
| ingress.annotations | object | `{}` |  |
| ingress.className | string | `""` |  |
| ingress.enabled | bool | `false` |  |
| ingress.hosts[0].host | string | `"chart-example.local"` |  |
| ingress.hosts[0].paths[0].path | string | `"/"` |  |
| ingress.hosts[0].paths[0].pathType | string | `"ImplementationSpecific"` |  |
| ingress.tls | list | `[]` |  |
| livenessProbe.httpGet.path | string | `"/healthz"` |  |
| livenessProbe.httpGet.port | string | `"http"` |  |
| nameOverride | string | `""` |  |
| nodeSelector | object | `{}` |  |
| persistence.accessMode | string | `"ReadWriteOnce"` |  |
| persistence.annotations | object | `{}` |  |
| persistence.enabled | bool | `false` |  |
| persistence.labels | object | `{}` |  |
| persistence.mountPath | string | `"/config"` |  |
| persistence.size | string | `"256Mi"` |  |
| podAnnotations | object | `{}` |  |
| podLabels | object | `{}` |  |
| podSecurityContext | object | `{}` |  |
| port | int | `8080` |  |
| prowlarr.secretKey | string | `""` |  |
| prowlarr.secretRef | string | `""` |  |
| readinessProbe.httpGet.path | string | `"/healthz"` |  |
| readinessProbe.httpGet.port | string | `"http"` |  |
| replicaCount | int | `1` |  |
| resources | object | `{}` |  |
| securityContext | object | `{}` |  |
| service.port | int | `8080` |  |
| service.type | string | `"ClusterIP"` |  |
| serviceAccount.annotations | object | `{}` |  |
| serviceAccount.automount | bool | `true` |  |
| serviceAccount.create | bool | `true` |  |
| serviceAccount.name | string | `""` |  |
| tmdb.secretKey | string | `""` |  |
| tmdb.secretRef | string | `""` |  |
| tolerations | list | `[]` |  |
| volumeMounts | list | `[]` |  |
| volumes | list | `[]` |  |

