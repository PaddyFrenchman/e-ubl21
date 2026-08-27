package regression

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type kubernetesPodList struct {
	Items []struct {
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Name  string `json:"name"`
				Image string `json:"image"`
			} `json:"containers"`
		} `json:"spec"`
		Status struct {
			Phase             string `json:"phase"`
			ContainerStatuses []struct {
				Name         string `json:"name"`
				Ready        bool   `json:"ready"`
				RestartCount int    `json:"restartCount"`
				ImageID      string `json:"imageID"`
			} `json:"containerStatuses"`
		} `json:"status"`
	} `json:"items"`
}

type kubernetesSnapshot struct {
	SchemaVersion int                     `json:"schemaVersion"`
	Pods          []kubernetesPodSnapshot `json:"pods"`
}

type kubernetesPodSnapshot struct {
	Namespace  string                        `json:"namespace"`
	Name       string                        `json:"name"`
	Phase      string                        `json:"phase"`
	Containers []kubernetesContainerSnapshot `json:"containers"`
}

type kubernetesContainerSnapshot struct {
	Name         string `json:"name"`
	Image        string `json:"image"`
	ImageID      string `json:"imageId,omitempty"`
	Ready        bool   `json:"ready"`
	RestartCount int    `json:"restartCount"`
}

func collectKubernetesEvidence(ctx context.Context, cfg EvidenceConfig, runner evidenceCommandRunner) ([]EvidenceEntry, []error) {
	kubectl := cfg.KubectlExecutable
	if kubectl == "" {
		kubectl = "kubectl"
	}
	baseArgs := kubernetesScopeArgs(cfg.KubernetesNamespace)
	var entries []EvidenceEntry
	var collectionErrors []error
	record := func(entry EvidenceEntry, err error) {
		entries = append(entries, entry)
		if err != nil {
			collectionErrors = append(collectionErrors, err)
		}
	}

	eventArgs := append(append([]string(nil), baseArgs...), "get", "events", "-o", "json")
	events, truncated, err := runner(ctx, maxCommandBytes, kubectl, eventArgs...)
	if err != nil {
		record(failedEvidence("kubernetes/events", err), err)
	} else {
		entry := EvidenceEntry{Name: "kubernetes/events", Status: "captured", Bytes: len(events), Truncated: truncated}
		err = writeKubernetesFile(cfg.OutputDir, "events.json", events)
		if err != nil {
			entry = failedEvidence(entry.Name, err)
		}
		record(entry, err)
	}

	podArgs := append(append([]string(nil), baseArgs...), "get", "pods", "-o", "json")
	if cfg.KubernetesSelector != "" {
		podArgs = append(podArgs, "--selector", cfg.KubernetesSelector)
	}
	podJSON, _, err := runner(ctx, maxCommandBytes, kubectl, podArgs...)
	if err != nil {
		record(failedEvidence("kubernetes/pods", err), err)
		return entries, collectionErrors
	}
	var pods kubernetesPodList
	if err := json.Unmarshal(podJSON, &pods); err != nil {
		err = fmt.Errorf("decode kubernetes pods: %w", err)
		record(failedEvidence("kubernetes/pods", err), err)
		return entries, collectionErrors
	}
	snapshot := sanitizeKubernetesPods(pods)
	snapshotJSON, err := marshalEvidenceJSON(snapshot)
	if err == nil {
		err = writeKubernetesFile(cfg.OutputDir, "pods.json", snapshotJSON)
	}
	if err != nil {
		record(failedEvidence("kubernetes/pods", err), err)
	} else {
		record(EvidenceEntry{Name: "kubernetes/pods", Status: "captured", Bytes: len(snapshotJSON)}, nil)
	}

	for _, pod := range snapshot.Pods {
		for _, container := range pod.Containers {
			baseName := safeKubernetesLogName(pod.Namespace, pod.Name, container.Name)
			requests := []struct {
				name string
				args []string
			}{{baseName, []string{"--namespace", pod.Namespace, "logs", pod.Name, "--container", container.Name, "--tail", "2000"}}}
			if container.RestartCount > 0 {
				previousArgs := append([]string(nil), requests[0].args...)
				previousArgs = append(previousArgs, "--previous")
				requests = append(requests, struct {
					name string
					args []string
				}{baseName + ".previous", previousArgs})
			}
			for _, request := range requests {
				content, truncated, commandErr := runner(ctx, maxLogBytes, kubectl, request.args...)
				if commandErr != nil {
					record(failedEvidence("logs/"+request.name, commandErr), commandErr)
					continue
				}
				content, redactionTruncated := redactServiceLog(content)
				truncated = truncated || redactionTruncated
				path := filepath.Join(cfg.OutputDir, "logs", request.name+".log")
				err = os.MkdirAll(filepath.Dir(path), 0o755)
				if err == nil {
					err = writeFileAtomically(path, content)
				}
				entry := EvidenceEntry{Name: "logs/" + request.name, Status: "captured", Bytes: len(content), Truncated: truncated}
				if err != nil {
					entry = failedEvidence(entry.Name, err)
				}
				record(entry, err)
			}
		}
	}
	return entries, collectionErrors
}

func kubernetesScopeArgs(namespace string) []string {
	if strings.TrimSpace(namespace) == "" {
		return []string{"--all-namespaces"}
	}
	return []string{"--namespace", namespace}
}

func sanitizeKubernetesPods(pods kubernetesPodList) kubernetesSnapshot {
	snapshot := kubernetesSnapshot{SchemaVersion: 1}
	for _, pod := range pods.Items {
		item := kubernetesPodSnapshot{Namespace: pod.Metadata.Namespace, Name: pod.Metadata.Name, Phase: pod.Status.Phase}
		statuses := map[string]struct {
			ready    bool
			restarts int
			imageID  string
		}{}
		for _, status := range pod.Status.ContainerStatuses {
			statuses[status.Name] = struct {
				ready    bool
				restarts int
				imageID  string
			}{status.Ready, status.RestartCount, status.ImageID}
		}
		for _, container := range pod.Spec.Containers {
			status := statuses[container.Name]
			item.Containers = append(item.Containers, kubernetesContainerSnapshot{
				Name: container.Name, Image: container.Image, ImageID: status.imageID,
				Ready: status.ready, RestartCount: status.restarts,
			})
		}
		sort.Slice(item.Containers, func(i, j int) bool { return item.Containers[i].Name < item.Containers[j].Name })
		snapshot.Pods = append(snapshot.Pods, item)
	}
	sort.Slice(snapshot.Pods, func(i, j int) bool {
		left, right := snapshot.Pods[i], snapshot.Pods[j]
		return left.Namespace < right.Namespace || (left.Namespace == right.Namespace && left.Name < right.Name)
	})
	return snapshot
}

func safeKubernetesLogName(namespace, pod, container string) string {
	value := strings.ToLower(namespace + "." + pod + "." + container)
	var output strings.Builder
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.' || char == '-' || char == '_' {
			output.WriteRune(char)
		} else {
			output.WriteByte('-')
		}
	}
	return strings.Trim(output.String(), ".-_")
}

func writeKubernetesFile(outputDir, name string, content []byte) error {
	path := filepath.Join(outputDir, "kubernetes", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomically(path, content)
}
