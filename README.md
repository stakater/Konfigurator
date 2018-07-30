# Konfigurator

A kubernetes operator that can dynamically generate app configuration when kubernetes resources change

## Design

The operator itself will have a CRD named `ConfigTemplate` that can define some of the following properties in the spec:

- templates
- volumeMounts
- renderTarget

The `templates` defined can use the go templating syntax to create the configuration templates that the operator will render. Apart from the usual constructs, the templates will have access to the following resources with the constructs below:

- Pods (.Pods)
- Services (.Services)
- Ingresses (.Ingresses)

An example `ConfigTemplate` with fluentd config looks like the following:

```yaml
apiVersion: konfigurator.stakater.com/v1
kind: ConfigTemplate
metadata:
    labels:
        apps: yourapp
        group: com.stakater.platform
        provider: stakater
        version: 1.0.0
    name: yourapp
spec:
    template:
        spec:
            renderTarget: ConfigMap
            volumeMounts:
                - mountPath: /fluentd/etc
                  container: app-container
            templates:
                fluentd.conf: |
    {{- $podsWithAnnotations := whereExist .Pods "ObjectMeta.Annotations.fluentdConfiguration" -}}
    # Create concat filters for supporting multiline
    {{- range $pod := $podsWithAnnotations -}}
        {{- $config := first (parseJson $pod.ObjectMeta.Annotations.fluentdConfiguration) }}
        {{- range $containerConfig := $config.containers }}
    <filter kubernetes.var.log.containers.{{ (index $pod.ObjectMeta.OwnerReferences 0).Name }}**_{{ $pod.ObjectMeta.Namespace }}_{{ $containerConfig.containerName }}**.log>
        @type concat
        key log
        multiline_start_regexp {{ $containerConfig.expressionFirstLine }}
        flush_interval 5s
        timeout_label @LOGS
    </filter>
        {{- end }}
    {{- end }}
```

Konfigurator will render the templates provided in the resource and create a new configmap with the rendered configs and mount them to the app containers. It will also update the config if any kubernetes resource i.e., pods, services or ingresses change.

## TODO

- [ ] Add deployment configuration to the repository
- [ ] Create an initial skeleton of the operator using the operator-sdk
- [ ] Add the ability to generate configmaps from CRD
- [ ] Add the ability to generate secrets from CRD
- [ ] Implement watch for the following:

  - [ ] Pods
  - [ ] Services
  - [ ] Ingresses
