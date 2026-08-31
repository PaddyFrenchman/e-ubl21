package kubernetes

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type objectKey struct {
	kind string
	name string
}

func TestWorkloadsAreIndependentAndOperationallyBounded(t *testing.T) {
	objects := loadObjects(t)
	wantArgs := map[string]string{
		"e-ubl21-api":            "serve",
		"e-ubl21-orchestrator":   "orchestrator",
		"e-ubl21-worker-xsd":     "worker-xsd",
		"e-ubl21-worker-en16931": "worker-en16931",
	}
	for name, wantArg := range wantArgs {
		deployment := requireObject(t, objects, "Deployment", name)
		if _, configured := objectMap(t, deployment["spec"])["replicas"]; configured {
			t.Fatalf("Deployment/%s declares spec.replicas even though an autoscaler owns it", name)
		}
		container := firstContainer(t, deployment)
		args := stringSlice(t, container["args"])
		if !reflect.DeepEqual(args, []string{wantArg}) {
			t.Fatalf("Deployment/%s args = %v, want [%s]", name, args, wantArg)
		}
		for _, field := range []string{"startupProbe", "readinessProbe", "livenessProbe", "resources", "securityContext"} {
			if _, ok := container[field]; !ok {
				t.Fatalf("Deployment/%s container has no %s", name, field)
			}
		}
		resources := objectMap(t, container["resources"])
		if len(objectMap(t, resources["requests"])) == 0 || len(objectMap(t, resources["limits"])) == 0 {
			t.Fatalf("Deployment/%s resources must contain requests and limits", name)
		}
	}

	xsdEnv := environment(t, firstContainer(t, requireObject(t, objects, "Deployment", "e-ubl21-worker-xsd")))
	enEnv := environment(t, firstContainer(t, requireObject(t, objects, "Deployment", "e-ubl21-worker-en16931")))
	assertEnv(t, xsdEnv, "EUBL21_WORKER_XSD_CONCURRENCY", "2")
	assertEnv(t, xsdEnv, "EUBL21_WORKER_XSD_PREFETCH", "2")
	assertEnv(t, xsdEnv, "EUBL21_WORKER_XSD_TIMEOUT", "30s")
	assertEnv(t, enEnv, "EUBL21_WORKER_EN16931_CONCURRENCY", "1")
	assertEnv(t, enEnv, "EUBL21_WORKER_EN16931_PREFETCH", "1")
	assertEnv(t, enEnv, "EUBL21_WORKER_EN16931_TIMEOUT", "45s")
}

func TestScaleToZeroWorkersActivateForOneQueuedRequest(t *testing.T) {
	objects := loadObjects(t)
	for _, name := range []string{"e-ubl21-worker-xsd", "e-ubl21-worker-en16931"} {
		scaledObject := requireObject(t, objects, "ScaledObject", name)
		metadata := jetStreamTriggerMetadata(t, scaledObject)
		if got := stringValue(t, metadata["activationLagThreshold"]); got != "0" {
			t.Fatalf("ScaledObject/%s activationLagThreshold = %q, want 0 so one request activates from zero replicas", name, got)
		}
	}
}

func TestResourceMetricsDependencyIsDocumented(t *testing.T) {
	content, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	documentation := string(content)
	for _, required := range []string{"Kubernetes Metrics Server", "/apis/metrics.k8s.io/v1beta1/nodes"} {
		if !strings.Contains(documentation, required) {
			t.Fatalf("README does not document autoscaling prerequisite %q", required)
		}
	}
}

func TestStageAutoscalersUseDifferentJetStreamConsumers(t *testing.T) {
	objects := loadObjects(t)
	xsd := requireObject(t, objects, "ScaledObject", "e-ubl21-worker-xsd")
	en := requireObject(t, objects, "ScaledObject", "e-ubl21-worker-en16931")

	assertScaleTarget(t, xsd, "e-ubl21-worker-xsd", 0, 20)
	assertScaleTarget(t, en, "e-ubl21-worker-en16931", 0, 30)
	xsdStream, xsdConsumer := jetStreamTrigger(t, xsd)
	enStream, enConsumer := jetStreamTrigger(t, en)
	if xsdStream != "EUBL21_PROD_VALIDATION_REQUEST_XSD" || xsdConsumer != "EUBL21_PROD_VALIDATION_XSD_WORKER" {
		t.Fatalf("XSD scaler topology = %s/%s", xsdStream, xsdConsumer)
	}
	if enStream != "EUBL21_PROD_VALIDATION_REQUEST_EN16931" || enConsumer != "EUBL21_PROD_VALIDATION_EN16931_WORKER" {
		t.Fatalf("EN scaler topology = %s/%s", enStream, enConsumer)
	}
	if xsdStream == enStream || xsdConsumer == enConsumer {
		t.Fatalf("stage scalers share JetStream topology: XSD %s/%s, EN %s/%s", xsdStream, xsdConsumer, enStream, enConsumer)
	}
	assertHasTrigger(t, xsd, "cpu")
	assertHasTrigger(t, en, "cpu")

	api := requireObject(t, objects, "HorizontalPodAutoscaler", "e-ubl21-api")
	if got := stringValue(t, nested(t, api, "spec", "scaleTargetRef", "name")); got != "e-ubl21-api" {
		t.Fatalf("API HPA target = %q", got)
	}
	orchestrator := requireObject(t, objects, "ScaledObject", "e-ubl21-orchestrator")
	assertScaleTarget(t, orchestrator, "e-ubl21-orchestrator", 1, 10)
}

func TestProvisionerOwnsIndependentConsumerDeliverySettings(t *testing.T) {
	objects := loadObjects(t)
	job := requireObject(t, objects, "Job", "e-ubl21-provision")
	container := firstContainer(t, job)
	if args := stringSlice(t, container["args"]); !reflect.DeepEqual(args, []string{"provision"}) {
		t.Fatalf("provision Job args = %v", args)
	}
	env := environment(t, container)
	assertEnv(t, env, "EUBL21_STAGE_XSD_ACK_WAIT", "90s")
	assertEnv(t, env, "EUBL21_STAGE_XSD_MAX_ACK_PENDING", "64")
	assertEnv(t, env, "EUBL21_STAGE_EN16931_ACK_WAIT", "120s")
	assertEnv(t, env, "EUBL21_STAGE_EN16931_MAX_ACK_PENDING", "32")
	assertEnv(t, env, "EUBL21_STAGE_PEPPOL_ACK_WAIT", "120s")
	assertEnv(t, env, "EUBL21_STAGE_PEPPOL_MAX_ACK_PENDING", "32")

	var pdbs []string
	for key := range objects {
		if key.kind == "PodDisruptionBudget" {
			pdbs = append(pdbs, key.name)
		}
	}
	sort.Strings(pdbs)
	if want := []string{"e-ubl21-api", "e-ubl21-orchestrator"}; !reflect.DeepEqual(pdbs, want) {
		t.Fatalf("PodDisruptionBudgets = %v, want %v", pdbs, want)
	}
}

func loadObjects(t *testing.T) map[objectKey]map[string]any {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	objects := make(map[objectKey]map[string]any)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" || entry.Name() == "kustomization.yaml" {
			continue
		}
		file, err := os.Open(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		decoder := yaml.NewDecoder(file)
		for {
			var object map[string]any
			err = decoder.Decode(&object)
			if err == io.EOF {
				break
			}
			if err != nil {
				file.Close()
				t.Fatalf("decode %s: %v", entry.Name(), err)
			}
			if len(object) == 0 {
				continue
			}
			key := objectKey{kind: stringValue(t, object["kind"]), name: stringValue(t, nested(t, object, "metadata", "name"))}
			if _, exists := objects[key]; exists {
				file.Close()
				t.Fatalf("duplicate %s/%s", key.kind, key.name)
			}
			objects[key] = object
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return objects
}

func requireObject(t *testing.T, objects map[objectKey]map[string]any, kind, name string) map[string]any {
	t.Helper()
	object, ok := objects[objectKey{kind: kind, name: name}]
	if !ok {
		t.Fatalf("missing %s/%s", kind, name)
	}
	return object
}

func firstContainer(t *testing.T, object map[string]any) map[string]any {
	t.Helper()
	containers := objectSlice(t, nested(t, object, "spec", "template", "spec", "containers"))
	if len(containers) != 1 {
		t.Fatalf("%s has %d containers, want 1", stringValue(t, nested(t, object, "metadata", "name")), len(containers))
	}
	return objectMap(t, containers[0])
}

func environment(t *testing.T, container map[string]any) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for _, raw := range objectSlice(t, container["env"]) {
		env := objectMap(t, raw)
		if value, ok := env["value"]; ok {
			result[stringValue(t, env["name"])] = stringValue(t, value)
		}
	}
	return result
}

func assertEnv(t *testing.T, env map[string]string, name, want string) {
	t.Helper()
	if got := env[name]; got != want {
		t.Fatalf("environment %s = %q, want %q", name, got, want)
	}
}

func assertScaleTarget(t *testing.T, object map[string]any, target string, min, max int) {
	t.Helper()
	if got := stringValue(t, nested(t, object, "spec", "scaleTargetRef", "name")); got != target {
		t.Fatalf("ScaledObject target = %q, want %q", got, target)
	}
	if got := intValue(t, nested(t, object, "spec", "minReplicaCount")); got != min {
		t.Fatalf("ScaledObject/%s minReplicaCount = %d, want %d", target, got, min)
	}
	if got := intValue(t, nested(t, object, "spec", "maxReplicaCount")); got != max {
		t.Fatalf("ScaledObject/%s maxReplicaCount = %d, want %d", target, got, max)
	}
}

func jetStreamTrigger(t *testing.T, object map[string]any) (string, string) {
	t.Helper()
	metadata := jetStreamTriggerMetadata(t, object)
	return stringValue(t, metadata["stream"]), stringValue(t, metadata["consumer"])
}

func jetStreamTriggerMetadata(t *testing.T, object map[string]any) map[string]any {
	t.Helper()
	for _, raw := range objectSlice(t, nested(t, object, "spec", "triggers")) {
		trigger := objectMap(t, raw)
		if stringValue(t, trigger["type"]) == "nats-jetstream" {
			return objectMap(t, trigger["metadata"])
		}
	}
	t.Fatalf("ScaledObject has no nats-jetstream trigger")
	return nil
}

func assertHasTrigger(t *testing.T, object map[string]any, triggerType string) {
	t.Helper()
	for _, raw := range objectSlice(t, nested(t, object, "spec", "triggers")) {
		if stringValue(t, objectMap(t, raw)["type"]) == triggerType {
			return
		}
	}
	t.Fatalf("ScaledObject has no %s trigger", triggerType)
}

func nested(t *testing.T, object map[string]any, path ...string) any {
	t.Helper()
	var current any = object
	for _, key := range path {
		mapping := objectMap(t, current)
		var ok bool
		current, ok = mapping[key]
		if !ok {
			t.Fatalf("missing field %s", strings.Join(path, "."))
		}
	}
	return current
}

func objectMap(t *testing.T, value any) map[string]any {
	t.Helper()
	mapping, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value %#v is not an object", value)
	}
	return mapping
}

func objectSlice(t *testing.T, value any) []any {
	t.Helper()
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("value %#v is not a list", value)
	}
	return items
}

func stringSlice(t *testing.T, value any) []string {
	t.Helper()
	items := objectSlice(t, value)
	result := make([]string, len(items))
	for index, item := range items {
		result[index] = stringValue(t, item)
	}
	return result
}

func stringValue(t *testing.T, value any) string {
	t.Helper()
	text, ok := value.(string)
	if !ok {
		t.Fatalf("value %#v is not a string", value)
	}
	return text
}

func intValue(t *testing.T, value any) int {
	t.Helper()
	number, ok := value.(int)
	if !ok {
		t.Fatalf("value %#v is not an int", value)
	}
	return number
}
