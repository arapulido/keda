//go:build e2e
// +build e2e

package trigger_update_so_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes"

	. "github.com/kedacore/keda/v2/tests/helper"
)

const (
	testName = "eventsource-test"
)

// Load environment variables from .env file
var _ = godotenv.Load("../../.env")

var (
	namespace                  = fmt.Sprintf("%s-ns", testName)
	scaledObjectName           = fmt.Sprintf("%s-so", testName)
	deploymentName             = fmt.Sprintf("%s-d", testName)
	clientName                 = fmt.Sprintf("%s-client", testName)
	cloudeventSourceName       = fmt.Sprintf("%s-ce", testName)
	cloudeventSourceErrName    = fmt.Sprintf("%s-ce-err", testName)
	cloudeventSourceErrName2   = fmt.Sprintf("%s-ce-err2", testName)
	cloudEventHTTPReceiverName = fmt.Sprintf("%s-cloudevent-http-receiver", testName)
	cloudEventHTTPServiceName  = fmt.Sprintf("%s-cloudevent-http-service", testName)
	cloudEventHTTPServiceURL   = fmt.Sprintf("http://%s.%s.svc.cluster.local:8899", cloudEventHTTPServiceName, namespace)
	clusterName                = "test-cluster"
	expectedSubject            = fmt.Sprintf("/%s/%s/scaledobject/%s", clusterName, namespace, scaledObjectName)
	expectedSource             = fmt.Sprintf("/%s/keda/keda", clusterName)
	lastCloudEventTime         = time.Now()
)

type templateData struct {
	TestNamespace              string
	ScaledObject               string
	DeploymentName             string
	ClientName                 string
	CloudEventSourceName       string
	CloudeventSourceErrName    string
	CloudeventSourceErrName2   string
	CloudEventHTTPReceiverName string
	CloudEventHTTPServiceName  string
	CloudEventHTTPServiceURL   string
	ClusterName                string
}

const (
	cloudEventSourceTemplate = `
  apiVersion: eventing.keda.sh/v1alpha1
  kind: CloudEventSource
  metadata:
    name: {{.CloudEventSourceName}}
    namespace: {{.TestNamespace}}
  spec:
    clusterName: {{.ClusterName}}
    destination:
      http:
        uri: {{.CloudEventHTTPServiceURL}}
  `

	cloudEventSourceWithExcludeTemplate = `
    apiVersion: eventing.keda.sh/v1alpha1
    kind: CloudEventSource
    metadata:
      name: {{.CloudEventSourceName}}
      namespace: {{.TestNamespace}}
    spec:
      clusterName: {{.ClusterName}}
      destination:
        http:
          uri: {{.CloudEventHTTPServiceURL}}
      eventSubscription:
        excludedEventTypes:
        - keda.scaledobject.failed.v1
    `

	cloudEventSourceWithIncludeTemplate = `
    apiVersion: eventing.keda.sh/v1alpha1
    kind: CloudEventSource
    metadata:
      name: {{.CloudEventSourceName}}
      namespace: {{.TestNamespace}}
    spec:
      clusterName: {{.ClusterName}}
      destination:
        http:
          uri: {{.CloudEventHTTPServiceURL}}
      eventSubscription:
        includedEventTypes:
        - keda.scaledobject.failed.v1
    `

	cloudEventHTTPServiceTemplate = `
  apiVersion: v1
  kind: Service
  metadata:
    name: {{.CloudEventHTTPServiceName}}
    namespace: {{.TestNamespace}}
  spec:
    type: ClusterIP
    ports:
    - protocol: TCP
      port: 8899
      targetPort: 8899
    selector:
      app: {{.CloudEventHTTPReceiverName}}
  `

	cloudEventHTTPReceiverTemplate = `
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    labels:
      deploy: {{.CloudEventHTTPReceiverName}}
    name: {{.CloudEventHTTPReceiverName}}
    namespace: {{.TestNamespace}}
  spec:
    selector:
      matchLabels:
        app: {{.CloudEventHTTPReceiverName}}
    replicas: 1
    template:
      metadata:
        labels:
          app: {{.CloudEventHTTPReceiverName}}
      spec:
        containers:
        - name: httpreceiver
          image: ghcr.io/kedacore/tests-cloudevents-http:latest
          ports:
          - containerPort: 8899
          resources:
            requests:
              cpu: "200m"
            limits:
              cpu: "500m"
  `

	scaledObjectErrTemplate = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObject}}
  namespace: {{.TestNamespace}}
spec:
  scaleTargetRef:
    name: test
  triggers:
    - type: kubernetes-workload
      metadata:
        podSelector: 'pod=testWorkloadDeploymentName'
        value: '1'
        activationValue: '3'
`

	clientTemplate = `
apiVersion: v1
kind: Pod
metadata:
  name: {{.ClientName}}
  namespace: {{.TestNamespace}}
spec:
  containers:
  - name: {{.ClientName}}
    image: curlimages/curl
    command:
      - sh
      - -c
      - "exec tail -f /dev/null"`

	cloudEventSourceWithErrTypeTemplate = `
    apiVersion: eventing.keda.sh/v1alpha1
    kind: CloudEventSource
    metadata:
      name: {{.CloudeventSourceErrName}}
      namespace: {{.TestNamespace}}
    spec:
      clusterName: {{.ClusterName}}
      destination:
        http:
          uri: {{.CloudEventHTTPServiceURL}}
      eventSubscription:
        includedEventTypes:
        - keda.scaledobject.failed.v2
    `

	cloudEventSourceWithErrTypeTemplate2 = `
    apiVersion: eventing.keda.sh/v1alpha1
    kind: CloudEventSource
    metadata:
      name: {{.CloudeventSourceErrName2}}
      namespace: {{.TestNamespace}}
    spec:
      clusterName: {{.ClusterName}}
      destination:
        http:
          uri: {{.CloudEventHTTPServiceURL}}
      eventSubscription:
        includedEventTypes:
        - keda.scaledobject.failed.v1
        excludedEventTypes:
        - keda.scaledobject.failed.v1
    `

	deploymentTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{.DeploymentName}}
  namespace: {{.TestNamespace}}
  labels:
    deploy: {{.DeploymentName}}
spec:
  replicas: 1
  selector:
    matchLabels:
      pod: {{.DeploymentName}}
  template:
    metadata:
      labels:
        pod: {{.DeploymentName}}
    spec:
      containers:
        - name: nginx
          image: 'nginxinc/nginx-unprivileged'
`

	scaledObjectTemplate = `
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: {{.ScaledObject}}
  namespace: {{.TestNamespace}}
spec:
  scaleTargetRef:
    name: {{.DeploymentName}}
  pollingInterval: 5
  cooldownPeriod: 5
  minReplicaCount: 1
  maxReplicaCount: 10
  advanced:
    horizontalPodAutoscalerConfig:
      behavior:
        scaleDown:
          stabilizationWindowSeconds: 15
  triggers:
  - type: cron
    metadata:
      timezone: Etc/UTC
      start: 3 * * * *
      end: 5 * * * *
      desiredReplicas: '4'
`
)

func TestScaledObjectGeneral(t *testing.T) {
	// setup
	t.Log("--- setting up ---")
	// Create kubernetes resources
	kc := GetKubernetesClient(t)
	data, templates := getTemplateData()
	CreateKubernetesResources(t, kc, namespace, data, templates)

	assert.True(t, WaitForAllPodRunningInNamespace(t, kc, namespace, 5, 20), "all pods should be running")

	testErrEventSourceEmitValue(t, kc, data)
	testEventSourceEmitValue(t, kc, data)
	testErrEventSourceExcludeValue(t, kc, data)
	testErrEventSourceIncludeValue(t, kc, data)
	testErrEventSourceCreation(t, kc, data)

	DeleteKubernetesResources(t, namespace, data, templates)
}

// tests error events emitted
func testErrEventSourceEmitValue(t *testing.T, _ *kubernetes.Clientset, data templateData) {
	t.Log("--- test emitting eventsource about scaledobject err---")
	KubectlApplyWithTemplate(t, data, "scaledObjectErrTemplate", scaledObjectErrTemplate)

	// wait 15 seconds to ensure event propagation
	time.Sleep(5 * time.Second)
	KubectlDeleteWithTemplate(t, data, "scaledObjectErrTemplate", scaledObjectErrTemplate)
	time.Sleep(10 * time.Second)

	out, outErr, err := ExecCommandOnSpecificPod(t, clientName, namespace, fmt.Sprintf("curl -X GET %s/getCloudEvent/%s", cloudEventHTTPServiceURL, "ScaledObjectCheckFailed"))
	assert.NotEmpty(t, out)
	assert.Empty(t, outErr)
	assert.NoError(t, err, "dont expect error requesting ")

	cloudEvents := []cloudevents.Event{}
	err = json.Unmarshal([]byte(out), &cloudEvents)

	assert.NoError(t, err, "dont expect error unmarshaling the cloudEvents")
	assert.Greater(t, len(cloudEvents), 0, "cloudEvents should have at least 1 item")

	foundEvents := []cloudevents.Event{}

	for _, cloudEvent := range cloudEvents {
		if cloudEvent.Subject() == expectedSubject {
			foundEvents = append(foundEvents, cloudEvent)
			data := map[string]string{}
			err := cloudEvent.DataAs(&data)
			t.Log("--- test emitting eventsource about scaledobject err---", "message", data["message"])

			assert.NoError(t, err)
			assert.Condition(t, func() bool {
				if data["message"] == "ScaledObject doesn't have correct scaleTargetRef specification" || data["message"] == "Target resource doesn't exist" {
					return true
				}
				return false
			}, "get filtered event")

			assert.Equal(t, cloudEvent.Type(), "keda.scaledobject.failed.v1")
			assert.Equal(t, cloudEvent.Source(), expectedSource)
			assert.Equal(t, cloudEvent.DataContentType(), "application/json")

			if lastCloudEventTime.Before(cloudEvent.Time()) {
				lastCloudEventTime = cloudEvent.Time()
			}
		}
	}
	assert.NotEmpty(t, foundEvents)
}

func testEventSourceEmitValue(t *testing.T, _ *kubernetes.Clientset, data templateData) {
	t.Log("--- test emitting eventsource about scaledobject removed---")
	KubectlApplyWithTemplate(t, data, "scaledObjectTemplate", scaledObjectTemplate)
	KubectlApplyWithTemplate(t, data, "deploymentTemplate", deploymentTemplate)

	// wait 15 seconds to ensure event propagation
	time.Sleep(5 * time.Second)
	KubectlDeleteWithTemplate(t, data, "scaledObjectTemplate", scaledObjectTemplate)
	time.Sleep(10 * time.Second)

	out, outErr, err := ExecCommandOnSpecificPod(t, clientName, namespace, fmt.Sprintf("curl -X GET %s/getCloudEvent/%s", cloudEventHTTPServiceURL, "ScaledObjectDeleted"))
	assert.NotEmpty(t, out)
	assert.Empty(t, outErr)
	assert.NoError(t, err, "dont expect error requesting ")

	cloudEvents := []cloudevents.Event{}
	err = json.Unmarshal([]byte(out), &cloudEvents)

	assert.NoError(t, err, "dont expect error unmarshaling the cloudEvents")
	assert.Greater(t, len(cloudEvents), 0, "cloudEvents should have at least 1 item")

	foundEvents := []cloudevents.Event{}

	for _, cloudEvent := range cloudEvents {
		if cloudEvent.Subject() == expectedSubject {
			foundEvents = append(foundEvents, cloudEvent)
			data := map[string]string{}
			err := cloudEvent.DataAs(&data)

			assert.NoError(t, err)
			assert.Equal(t, data["message"], "ScaledObject was deleted")
			assert.Equal(t, cloudEvent.Type(), "keda.scaledobject.removed.v1")
			assert.Equal(t, cloudEvent.Source(), expectedSource)
			assert.Equal(t, cloudEvent.DataContentType(), "application/json")

			if lastCloudEventTime.Before(cloudEvent.Time()) {
				lastCloudEventTime = cloudEvent.Time()
			}
		}
	}
	assert.NotEmpty(t, foundEvents)
}

// tests error events not emitted by
func testErrEventSourceExcludeValue(t *testing.T, _ *kubernetes.Clientset, data templateData) {
	t.Log("--- test emitting eventsource about scaledobject err with exclude filter---")

	KubectlDeleteWithTemplate(t, data, "cloudEventSourceTemplate", cloudEventSourceTemplate)
	KubectlApplyWithTemplate(t, data, "cloudEventSourceWithExcludeTemplate", cloudEventSourceWithExcludeTemplate)
	KubectlApplyWithTemplate(t, data, "scaledObjectErrTemplate", scaledObjectErrTemplate)

	// wait 15 seconds to ensure event propagation
	time.Sleep(15 * time.Second)

	out, outErr, err := ExecCommandOnSpecificPod(t, clientName, namespace, fmt.Sprintf("curl -X GET %s/getCloudEvent/%s", cloudEventHTTPServiceURL, "ScaledObjectCheckFailed"))
	assert.NotEmpty(t, out)
	assert.Empty(t, outErr)
	assert.NoError(t, err, "dont expect error requesting ")

	cloudEvents := []cloudevents.Event{}
	err = json.Unmarshal([]byte(out), &cloudEvents)

	assert.NoError(t, err, "dont expect error unmarshaling the cloudEvents")

	for _, cloudEvent := range cloudEvents {
		assert.Condition(t, func() bool {
			if cloudEvent.Subject() == expectedSubject &&
				cloudEvent.Time().After(lastCloudEventTime) &&
				cloudEvent.Type() == "keda.scaledobject.failed.v1" {
				return false
			}
			return true
		}, "get filtered event")
	}

	KubectlDeleteWithTemplate(t, data, "cloudEventSourceWithExcludeTemplate", cloudEventSourceWithExcludeTemplate)
	KubectlApplyWithTemplate(t, data, "cloudEventSourceTemplate", cloudEventSourceTemplate)
}

// tests error events in include filter
func testErrEventSourceIncludeValue(t *testing.T, _ *kubernetes.Clientset, data templateData) {
	t.Log("--- test emitting eventsource about scaledobject err with include filter---")

	KubectlDeleteWithTemplate(t, data, "cloudEventSourceTemplate", cloudEventSourceTemplate)
	KubectlApplyWithTemplate(t, data, "cloudEventSourceWithIncludeTemplate", cloudEventSourceWithIncludeTemplate)
	KubectlApplyWithTemplate(t, data, "scaledObjectErrTemplate", scaledObjectErrTemplate)

	// wait 15 seconds to ensure event propagation
	time.Sleep(15 * time.Second)

	out, outErr, err := ExecCommandOnSpecificPod(t, clientName, namespace, fmt.Sprintf("curl -X GET %s/getCloudEvent/%s", cloudEventHTTPServiceURL, "ScaledObjectCheckFailed"))
	assert.NotEmpty(t, out)
	assert.Empty(t, outErr)
	assert.NoError(t, err, "dont expect error requesting ")

	cloudEvents := []cloudevents.Event{}
	err = json.Unmarshal([]byte(out), &cloudEvents)

	assert.NoError(t, err, "dont expect error unmarshaling the cloudEvents")

	foundEvents := []cloudevents.Event{}
	for _, cloudEvent := range cloudEvents {
		if cloudEvent.Subject() == expectedSubject &&
			cloudEvent.Time().After(lastCloudEventTime) &&
			cloudEvent.Type() == "keda.scaledobject.failed.v1" {
			foundEvents = append(foundEvents, cloudEvent)
		}
	}
	assert.NotEmpty(t, foundEvents)
	KubectlDeleteWithTemplate(t, data, "cloudEventSourceWithIncludeTemplate", cloudEventSourceWithIncludeTemplate)
	KubectlApplyWithTemplate(t, data, "cloudEventSourceTemplate", cloudEventSourceTemplate)
}

// tests error event type when creation
func testErrEventSourceCreation(t *testing.T, _ *kubernetes.Clientset, data templateData) {
	t.Log("--- test emitting eventsource about scaledobject err with include filter---")

	KubectlDeleteWithTemplate(t, data, "cloudEventSourceTemplate", cloudEventSourceTemplate)

	err := KubectlApplyWithErrors(t, data, "cloudEventSourceWithErrTypeTemplate", cloudEventSourceWithErrTypeTemplate)
	assert.ErrorContains(t, err, `The CloudEventSource "eventsource-test-ce-err" is invalid:`)

	err = KubectlApplyWithErrors(t, data, "cloudEventSourceWithErrTypeTemplate2", cloudEventSourceWithErrTypeTemplate2)
	assert.ErrorContains(t, err, `setting included types and excluded types at the same time is not supported`)

	KubectlApplyWithTemplate(t, data, "cloudEventSourceTemplate", cloudEventSourceTemplate)
}

// help function to load template data
func getTemplateData() (templateData, []Template) {
	return templateData{
			TestNamespace:              namespace,
			ScaledObject:               scaledObjectName,
			DeploymentName:             deploymentName,
			ClientName:                 clientName,
			CloudEventSourceName:       cloudeventSourceName,
			CloudeventSourceErrName:    cloudeventSourceErrName,
			CloudeventSourceErrName2:   cloudeventSourceErrName2,
			CloudEventHTTPReceiverName: cloudEventHTTPReceiverName,
			CloudEventHTTPServiceName:  cloudEventHTTPServiceName,
			CloudEventHTTPServiceURL:   cloudEventHTTPServiceURL,
			ClusterName:                clusterName,
		}, []Template{
			{Name: "cloudEventHTTPReceiverTemplate", Config: cloudEventHTTPReceiverTemplate},
			{Name: "cloudEventHTTPServiceTemplate", Config: cloudEventHTTPServiceTemplate},
			{Name: "clientTemplate", Config: clientTemplate},
			{Name: "cloudEventSourceTemplate", Config: cloudEventSourceTemplate},
		}
}
