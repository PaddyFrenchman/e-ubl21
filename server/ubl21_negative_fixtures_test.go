package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/PaddyFrenchman/e-ubl21/queue"
)

func TestTrackedUBLNegativeFixturesShortCircuitBeforeEN16931(t *testing.T) {
	type mutation struct {
		ID          string `json:"id"`
		Description string `json:"description"`
	}
	type fixture struct {
		ID             string    `json:"id"`
		DocumentType   string    `json:"documentType"`
		File           string    `json:"file"`
		Mutation       *mutation `json:"mutation"`
		ExpectedFinal  string    `json:"expectedFinal"`
		ExpectedStages []struct {
			Stage   string `json:"stage"`
			Outcome string `json:"outcome"`
		} `json:"expectedStages"`
	}
	var manifest struct {
		Fixtures []fixture `json:"fixtures"`
	}
	manifestPath := filepath.Join("..", "createsamples", "samples", "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}

	negativeCount := 0
	for _, candidate := range manifest.Fixtures {
		if candidate.Mutation == nil || candidate.ExpectedFinal != "invalid" {
			continue
		}
		negativeCount++
		t.Run(candidate.Mutation.ID, func(t *testing.T) {
			if candidate.Mutation.Description == "" || len(candidate.ExpectedStages) != 1 ||
				candidate.ExpectedStages[0].Stage != "ubl21-xsd" ||
				candidate.ExpectedStages[0].Outcome != "standards-invalid" {
				t.Fatalf("manifest does not declare an XSD-terminal failure: %+v", candidate)
			}
			content, err := os.ReadFile(filepath.Join("..", "createsamples", "samples", filepath.FromSlash(candidate.File)))
			if err != nil {
				t.Fatal(err)
			}
			source := &memoryValidationContentSource{content: content}
			enCalled := false
			pipeline := newDocumentValidationPipeline(source, en16931ValidatorFunc(func(context.Context, io.Reader) (queue.ValidationStageResult, error) {
				enCalled = true
				return queue.ValidationStageResult{}, nil
			}), 10*time.Second)
			pipeline.xsdValidate = func(ctx context.Context, schemaPath string, xml io.Reader, _ time.Duration) error {
				command := exec.CommandContext(ctx, "xmllint", "--nonet", "--noout", "--schema", schemaPath, "-")
				command.Stdin = xml
				output, err := command.CombinedOutput()
				if err == nil {
					return nil
				}
				var exitError *exec.ExitError
				if errors.As(err, &exitError) {
					return fmt.Errorf("%w: %s", errSchemaStandardsInvalid, output)
				}
				return err
			}
			result, validationErr := pipeline.Validate(
				context.Background(), candidate.ID, candidate.DocumentType,
				filepath.Join("..", "XSD", "maindoc", "UBL-Invoice-2.1.xsd"),
			)
			if enCalled || source.opens != 1 {
				t.Fatalf("later semantic stage executed: enCalled=%t content opens=%d", enCalled, source.opens)
			}
			if validationErr != nil || result.Status != queue.StatusInvalid || len(result.Stages) != 1 || result.Stages[0].Valid {
				t.Fatalf("XSD-negative result/error = %+v / %v", result, validationErr)
			}
		})
	}
	if negativeCount != 6 {
		t.Fatalf("tracked UBL negative fixtures = %d, want 6", negativeCount)
	}
}
