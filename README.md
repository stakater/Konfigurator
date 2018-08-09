# Konfigurator

## Problem

Dynamically generating app configuration when kubernetes resources change.

## Solution

A kubernetes operator that can dynamically generate app configuration when kubernetes resources change

## TODO

- [x] Add deployment configuration to the repository
- [x] Create an initial skeleton of the operator using the operator-sdk
- [x] Add the ability to generate configmaps from CRD
- [x] Add the ability to generate secrets from CRD
- [x] Implement watch for the following:

  - [x] Pods
  - [x] Services
  - [x] Ingresses

## Design

The operator has a CRD named `KonfiguratorTemplate` that can define some of the following properties in the spec:

- templates
- volumeMounts
- renderTarget

The `templates` defined can use the go templating syntax to create the configuration templates that the operator will render. Apart from the usual constructs, the templates will have access to the following resources with the constructs below:

- Pods (.Pods)
- Services (.Services)
- Ingresses (.Ingresses)

An example `KonfiguratorTemplate` with fluentd config looks like the following:

```yaml
apiVersion: konfigurator.stakater.com/v1alpha1
kind: KonfiguratorTemplate
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
            app:
                name: testapp
                kind: Deployment
                volumeMounts:
                - mountPath: /var/cfg
                container: test
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

## KonfiguratorTemplate properties

You can set the following properties in KonfiguratorTemplate to customize your generated resource

- renderTarget (Where will the rendered config be generated? ConfigMap or Secret)
- app (The app that you want to tie this resource to)
    - name (Name of the app)
    - kind (Kind of the app. Can be Deployment, StatefulSet or DaemonSet)
    - volumeMounts (Array of volume mounts that need to be mounted to the container)
        - mountPath (The mount path where the rendered resource needs to be mounted)
    - container (Container name inside the target app that needs the volume mount)
- template (Array of key value pairs, just like a configmap, but you can use go templates inside of them)

## How to use Konfigurator

Clone the repository and apply the CRD by running the following command:

```bash
cd Konfigurator
kubectl apply -f deploy/crd.yaml
```

Once the CRD is installed, you can deploy the operator on your kubernetes via any of the following methods.

## Deploying to Kubernetes

You can deploy Konfigurator by using the following methods:

### Vanilla Manifests

You can apply vanilla manifests by running the following command

```bash
kubectl apply -f https://raw.githubusercontent.com/stakater/Konfigurator/master/deployments/kubernetes/konfigurator.yaml
```

By default Konfigurator gets deployed in the default namespace and manages its custom resources in that namespace.

### Helm Charts

Alternatively if you have configured helm on your cluster, you can add konfigurator to helm from our public chart repository and deploy it via helm using below mentioned commands

```bash
helm repo add stakater https://stakater.github.io/stakater-charts

helm repo update

helm install stakater/konfigurator
```

## Help

**Got a question?**
File a GitHub [issue](https://github.com/stakater/Konfigurator/issues), or send us an [email](mailto:stakater@gmail.com).

### Talk to us on Slack

Join and talk to us on Slack for discussing Konfigurator

[![Join Slack](https://stakater.github.io/README/stakater-join-slack-btn.png)](https://stakater-slack.herokuapp.com/)
[![Chat](https://stakater.github.io/README/stakater-chat-btn.png)](https://stakater.slack.com/)

## Contributing

### Bug Reports & Feature Requests

Please use the [issue tracker](https://github.com/stakater/Konfigurator/issues) to report any bugs or file feature requests.

### Developing

PRs are welcome. In general, we follow the "fork-and-pull" Git workflow.

 1. **Fork** the repo on GitHub
 2. **Clone** the project to your own machine
 3. **Commit** changes to your own branch
 4. **Push** your work back up to your fork
 5. Submit a **Pull request** so that we can review your changes

NOTE: Be sure to merge the latest from "upstream" before making a pull request!

## Changelog

View our closed [Pull Requests](https://github.com/stakater/Konfigurator/pulls?q=is%3Apr+is%3Aclosed).

## License

Apache2 © [Stakater](http://stakater.com)

## About

`Konfigurator` is maintained by [Stakater][website]. Like it? Please let us know at <hello@stakater.com>

See [our other projects][community]
or contact us in case of professional services and queries on <hello@stakater.com>

  [website]: http://stakater.com/
  [community]: https://github.com/stakater/