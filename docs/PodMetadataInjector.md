# PodMetadataInjector

The operator has a CRD named `PodMetadataInjector` that can supplement `ObjectMetadata` to the Pod manifests which are used for go templating in `KonfiguratorTemplate` rendering.
## Why?
The motivation of this CRD is that some operators don't allow to specify custom Labels and Annotations for underline resources. It makes using go templating with ObjectMedata filed impossible.
For example, because every application has its own log format, users can define the app-specific fluentd configurations in the Pod Annotation like [this](https://github.com/stakater/Konfigurator/blob/master/examples/parsing-multiline-logs-with-fluentd.md#setting-up-apps-that-log-stuff). But if there is no way to customize Pod annotations, it breaks the templating flow.
To solve this problem, we provide ObjectMetadata injection for annotations so users can define their custom annotations in `PodMetadataInjector` CR and it will be injected to the Pod manifest when it is rendered with templates.

## Properties
It does not have any properties. It's Labels are used to select the Pods and it's Annotations are injected to the selected Pods.

**Example**
```js
apiVersion: konfigurator.stakater.com/v1alpha1
kind: PodMetadataInjector
metadata:
  name: test
  labels:
    apps: Kibana
  annotations:
    fluentdConfiguration: >
          [
            {
              "containers":
              [
                {
                  "expressionFirstLine": "starting regex",
                  "expression": "full regex",
                  "timeFormat": "time format",
                  "containerName": "container-name"
                }
              ],
              "notifications": {
                "slack": {
                  "webhookURL": "https://google.com",
                  "channelName": "dev-notifications"
                }
              }
            }
          ]
```  
Once this is created, the annotations are injected to the Pods having `apps=kibana` label when they are used for go template rendering in `KonfiguratorTemplate` reconcile loop. 
