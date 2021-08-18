[![Community Extension](https://img.shields.io/badge/Community%20Extension-An%20open%20source%20community%20maintained%20project-FF4700)](https://github.com/camunda-community-hub/community)[![Lifecycle: Incubating](https://img.shields.io/badge/Lifecycle-Incubating-blue)](https://github.com/Camunda-Community-Hub/community/blob/main/extension-lifecycle.md#incubating-)[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

# Zeebe + Operate + TaskList and Ingress Controller HELM Chart
 
 
This chart is hosted at: [http://helm.camunda.io](http://helm.camunda.io)
 
These charts install:
- A Zeebe Cluster
- Operate configured to talk with the Zeebe Cluster
- Ingress Controller which
    - Expose Operate HTTP endpoint under `/`
    - If Kibana is enabled also expose Kibana HTTP endpoint under `/logs`
   

```
zeebe-cluster: 
  kibana:
    enabled: true     
    healthCheckPath: "/logs/app/kibana"
    ingress:
      enabled: true
      annotations:
        kubernetes.io/ingress.class: nginx
      path: /logs
      hosts:
        - ""
    extraEnvs:
      - name: SERVER_BASEPATH
        value: /logs
      - name: SERVER_REWRITEBASEPATH
        value: "true"
```
