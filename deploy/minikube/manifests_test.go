package minikube

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMinikubeDatabaseContract(t *testing.T) {
	t.Parallel()

	root := filepath.Clean(filepath.Join("..", ".."))
	read := func(path string) string {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		return string(content)
	}

	values := read("deploy/minikube/yugabytedb-values.yaml")
	for _, required := range []string{
		"ephemeral: false",
		"master: 1",
		"tserver: 1",
		"ysql_enable_auth: \"true\"",
		"app.kubernetes.io/part-of: e-ubl21",
	} {
		if !strings.Contains(values, required) {
			t.Errorf("YugabyteDB values do not contain %q", required)
		}
	}

	databaseRunner := read("devsetup/minikube-yugabytedb.sh")
	for _, required := range []string{
		"yugabyte-2025.2.6.tgz",
		"26518b8dfb7389dd6e97bc034df59838236c1a0c15d6f7482a1cf9a7dd4b80a0",
		"yugabytedb/yugabyte:2025.2.6.0-b111",
		"apply_migration_job initial",
		"apply_smoke_job initial",
		"delete pod yb-tserver-0",
		"apply_migration_job restart",
		"DATABASE_ARTIFACT_DIR/summary.json",
		"CURRENT_PHASE=\"database-readiness\"",
		"write_summary fail \"$CURRENT_PHASE\"",
		"\"readiness\": \"${READINESS_RESULT}\"",
	} {
		if !strings.Contains(databaseRunner, required) {
			t.Errorf("database runner does not contain %q", required)
		}
	}

	for _, manifest := range []string{"deploy/minikube/api.yaml", "deploy/minikube/worker.yaml"} {
		content := read(manifest)
		if !strings.Contains(content, "name: EUBL21_DATABASE_URL") ||
			!strings.Contains(content, "name: eubl21-db-auth") ||
			!strings.Contains(content, "key: database-url") {
			t.Errorf("%s does not inject the generic database Secret", manifest)
		}
	}

	topologyRunner := read("devsetup/regression-minikube.sh")
	databaseIndex := strings.Index(topologyRunner, "minikube-yugabytedb.sh")
	applicationIndex := strings.Index(topologyRunner, "apply -k deploy/minikube")
	if databaseIndex < 0 || applicationIndex < 0 || databaseIndex > applicationIndex {
		t.Error("Minikube runner does not gate application deployment on YugabyteDB qualification")
	}

	localRunner := read("run-regression-tests.sh")
	if !strings.Contains(localRunner, "helm minikube sha256sum") {
		t.Error("comprehensive regression runner does not require the database deployment tools")
	}
	workflow := read(".github/workflows/regression-minikube.yml")
	if !strings.Contains(workflow, "database/summary.json") {
		t.Error("Minikube CI summary does not publish YugabyteDB evidence")
	}
}
