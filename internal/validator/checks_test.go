package validator

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeResponse struct {
	output string
	err    error
}

type fakeRunner struct {
	responses []fakeResponse
	calls     [][]string
}

func (f *fakeRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string(nil), args...))
	if len(f.responses) == 0 {
		return nil, errors.New("unexpected runner call")
	}
	response := f.responses[0]
	f.responses = f.responses[1:]
	return []byte(response.output), response.err
}

func TestObjectExistsAndFieldEquals(t *testing.T) {
	t.Run("object exists", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"metadata":{"name":"test-record"}}`}}}
		result, err := (ObjectExists{
			Kind: "configmap", Namespace: "kubecrypt-test", Name: "test-record",
		}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Success)
	})

	t.Run("cluster scoped object exists", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"metadata":{"name":"kube-system"}}`}}}
		result, err := (ObjectExists{Kind: "namespace", Name: "kube-system"}).
			Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Success)
		wantArgs := []string{"get", "namespace", "kube-system", "--output=json"}
		if !reflect.DeepEqual(runner.calls[0], wantArgs) {
			t.Fatalf("runner args = %#v, want %#v", runner.calls[0], wantArgs)
		}
	})

	t.Run("field differs", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"data":{"status":"pending"}}`}}}
		result, err := (FieldEquals{
			Kind: "configmap", Namespace: "kubecrypt-test", Name: "test-record",
			Field: "data.status", Value: "admitted",
		}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Wrong)
		assertContains(t, result.Message, `"pending"`)
	})

	t.Run("field matches", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"data":{"status":"admitted"}}`}}}
		result, err := (FieldEquals{
			Kind: "configmap", Namespace: "kubecrypt-test", Name: "test-record",
			Field: "data.status", Value: "admitted",
		}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Success)
	})

	t.Run("indexed container image matches", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"spec":{"containers":[{"name":"nginx","image":"nginx:1.27"}]}}`}}}
		result, err := (FieldEquals{
			Kind: "pod", Namespace: "kubecrypt-beginner", Name: "nginx",
			Field: "spec.containers[0].image", Value: "nginx:1.27",
		}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Success)
	})

	t.Run("indexed container image differs", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"spec":{"containers":[{"name":"nginx","image":"nginx:1.26"}]}}`}}}
		result, err := (FieldEquals{
			Kind: "pod", Namespace: "kubecrypt-beginner", Name: "nginx",
			Field: "spec.containers[0].image", Value: "nginx:1.27",
		}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Wrong)
		assertContains(t, result.Message, `"nginx:1.26"`)
	})

	t.Run("missing indexed field is wrong", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"spec":{"containers":[]}}`}}}
		result, err := (FieldEquals{
			Kind: "pod", Namespace: "kubecrypt-beginner", Name: "nginx",
			Field: "spec.containers[0].image", Value: "nginx:1.27",
		}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Wrong)
		assertContains(t, result.Message, "has no field")
	})
}

func TestNodeTopology(t *testing.T) {
	nodes := `{"items":[
		{"metadata":{"labels":{"node-role.kubernetes.io/control-plane":""}},"status":{"conditions":[{"type":"Ready","status":"True"}]}},
		{"metadata":{"labels":{}},"status":{"conditions":[{"type":"Ready","status":"True"}]}},
		{"metadata":{"labels":{}},"status":{"conditions":[{"type":"Ready","status":"True"}]}}
	]}`
	runner := &fakeRunner{responses: []fakeResponse{{output: nodes}}}
	result, err := (NodeTopology{Count: 3, ControlPlanes: 1, Workers: 2}).
		Evaluate(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, result, Success)
	assertContains(t, result.Message, "3 Ready nodes")
	wantArgs := []string{"get", "nodes", "--output=json"}
	if !reflect.DeepEqual(runner.calls[0], wantArgs) {
		t.Fatalf("runner args = %#v, want %#v", runner.calls[0], wantArgs)
	}
}

func TestDeploymentExistsStates(t *testing.T) {
	t.Run("wrong when absent", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{
			output: `Error from server (NotFound): deployments.apps "test-workload" not found`,
			err:    errors.New("exit status 1"),
		}}}

		result, err := (DeploymentExists{Namespace: "kubecrypt-test", Name: "test-workload"}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertStatus(t, result, Wrong)
		assertContains(t, result.Message, "does not exist")
	})

	t.Run("success when present", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{{output: `{"metadata":{"name":"test-workload"}}`}}}
		result, err := (DeploymentExists{Namespace: "kubecrypt-test", Name: "test-workload"}).Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		assertStatus(t, result, Success)
		wantArgs := []string{"get", "deployment", "test-workload", "--namespace", "kubecrypt-test", "--output=json"}
		if !reflect.DeepEqual(runner.calls[0], wantArgs) {
			t.Fatalf("runner args = %#v, want %#v", runner.calls[0], wantArgs)
		}
	})
}

func TestDeploymentAvailableStates(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		wantStatus Status
		wantText   string
	}{
		{
			name:       "generation converging",
			json:       `{"metadata":{"name":"test-workload","generation":3},"spec":{"replicas":1},"status":{"observedGeneration":2,"availableReplicas":1}}`,
			wantStatus: Converging,
			wantText:   "has not observed generation 3",
		},
		{
			name:       "replicas converging",
			json:       `{"metadata":{"name":"test-workload","generation":3},"spec":{"replicas":2},"status":{"observedGeneration":3,"availableReplicas":1}}`,
			wantStatus: Converging,
			wantText:   "1/2 available",
		},
		{
			name:       "success",
			json:       `{"metadata":{"name":"test-workload","generation":3},"spec":{"replicas":2},"status":{"observedGeneration":3,"availableReplicas":2}}`,
			wantStatus: Success,
			wantText:   "2/2 available",
		},
		{
			name:       "default replica",
			json:       `{"metadata":{"name":"test-workload","generation":1},"spec":{},"status":{"observedGeneration":1,"availableReplicas":1}}`,
			wantStatus: Success,
			wantText:   "1/1 available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{responses: []fakeResponse{{output: tt.json}}}
			result, err := (DeploymentAvailable{Namespace: "kubecrypt-test", Name: "test-workload"}).Evaluate(context.Background(), runner)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			assertStatus(t, result, tt.wantStatus)
			assertContains(t, result.Message, tt.wantText)
		})
	}
}

func TestDeploymentAvailableRequiresAuthoredReplicaCount(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{{output: `{"metadata":{"name":"front-desk","generation":1},"spec":{"replicas":0},"status":{"observedGeneration":1,"availableReplicas":0}}`}}}
	result, err := (DeploymentAvailable{
		Namespace: "kubecrypt-front-desk", Name: "front-desk", RequiredReplicas: 1,
	}).Evaluate(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus(t, result, Wrong)
	assertContains(t, result.Message, "expected 1")
}

func TestPodReadyStates(t *testing.T) {
	tests := []struct {
		name       string
		json       string
		minReady   int
		wantStatus Status
		wantText   string
	}{
		{
			name:       "no matching pods converging",
			json:       `{"items":[]}`,
			wantStatus: Converging,
			wantText:   "no Pods match",
		},
		{
			name:       "unready converging",
			json:       podList(`{"metadata":{"name":"test-workload-a"},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"False"}]}}`),
			wantStatus: Converging,
			wantText:   "0/1 required",
		},
		{
			name:       "failed is wrong",
			json:       podList(`{"metadata":{"name":"test-workload-a"},"status":{"phase":"Failed"}}`),
			wantStatus: Wrong,
			wantText:   "is Failed",
		},
		{
			name: "enough ready succeeds",
			json: podList(
				`{"metadata":{"name":"test-workload-a"},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}`,
				`{"metadata":{"name":"test-workload-b"},"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}`,
			),
			minReady:   2,
			wantStatus: Success,
			wantText:   "2 Pods",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{responses: []fakeResponse{{output: tt.json}}}
			result, err := (PodReady{
				Namespace: "kubecrypt-test",
				Selector:  "app=test-workload",
				MinReady:  tt.minReady,
			}).Evaluate(context.Background(), runner)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			assertStatus(t, result, tt.wantStatus)
			assertContains(t, result.Message, tt.wantText)
		})
	}
}

func TestContainersHealthyStates(t *testing.T) {
	tests := []struct {
		name       string
		state      string
		wantStatus Status
		wantText   string
	}{
		{
			name:       "known failure wait is wrong",
			state:      `{"waiting":{"reason":"ImagePullBackOff"}}`,
			wantStatus: Wrong,
			wantText:   "ImagePullBackOff",
		},
		{
			name:       "transient wait is converging",
			state:      `{"waiting":{"reason":"ContainerCreating"}}`,
			wantStatus: Converging,
			wantText:   "ContainerCreating",
		},
		{
			name:       "failed termination is wrong",
			state:      `{"terminated":{"exitCode":12}}`,
			wantStatus: Wrong,
			wantText:   "exit code 12",
		},
		{
			name:       "successful termination is converging",
			state:      `{"terminated":{"exitCode":0}}`,
			wantStatus: Converging,
			wantText:   "exit code 0",
		},
		{
			name:       "running succeeds",
			state:      `{"running":{"startedAt":"2026-09-03T00:00:00Z"}}`,
			wantStatus: Success,
			wantText:   "all containers",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := `{"metadata":{"name":"test-workload-a"},"status":{"phase":"Running","containerStatuses":[{"name":"workload","state":` + tt.state + `}]}}`
			runner := &fakeRunner{responses: []fakeResponse{{output: podList(item)}}}
			result, err := (ContainersHealthy{Namespace: "kubecrypt-test", Selector: "app=test-workload"}).Evaluate(context.Background(), runner)
			if err != nil {
				t.Fatalf("Evaluate() error = %v", err)
			}
			assertStatus(t, result, tt.wantStatus)
			assertContains(t, result.Message, tt.wantText)
		})
	}
}

func TestContainersHealthyWrongOutranksEarlierConvergingState(t *testing.T) {
	item := `{
		"metadata":{"name":"test-workload-a"},
		"status":{"phase":"Running","containerStatuses":[
			{"name":"sidecar","state":{"waiting":{"reason":"ContainerCreating"}}},
			{"name":"apparition","state":{"waiting":{"reason":"CrashLoopBackOff"}}}
		]}
	}`
	runner := &fakeRunner{responses: []fakeResponse{{output: podList(item)}}}

	result, err := (ContainersHealthy{Namespace: "kubecrypt-test", Selector: "app=test-workload"}).Evaluate(context.Background(), runner)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertStatus(t, result, Wrong)
	assertContains(t, result.Message, "CrashLoopBackOff")
}

func TestMalformedOutputIsExplainableError(t *testing.T) {
	runner := &fakeRunner{responses: []fakeResponse{{output: `not-json`}}}
	result, err := (PodReady{Namespace: "kubecrypt-test", Selector: "app=test-workload"}).Evaluate(context.Background(), runner)
	if err == nil {
		t.Fatal("Evaluate() error = nil, want malformed output error")
	}
	assertStatus(t, result, Wrong)
	assertContains(t, result.Message, "malformed Pod list output")
}

func TestAllCombinesResultsWithDeterministicSeverity(t *testing.T) {
	check := All(
		staticCheck(Result{Status: Success, Message: "deployment exists"}),
		staticCheck(Result{Status: Wrong, Message: "container failed"}),
		staticCheck(Result{Status: Converging, Message: "pod becoming ready"}),
	)

	result, err := check.Evaluate(context.Background(), &fakeRunner{})
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	assertStatus(t, result, Wrong)
	if result.Message != "deployment exists; container failed; pod becoming ready" {
		t.Fatalf("message = %q", result.Message)
	}
}

func TestPodCreationChecksGradeStoredObject(t *testing.T) {
	checks := All(
		ObjectExists{Kind: "pod", Namespace: "kubecrypt-beginner", Name: "nginx"},
		FieldEquals{
			Kind: "pod", Namespace: "kubecrypt-beginner", Name: "nginx",
			Field: "spec.containers[0].image", Value: "nginx:1.27",
		},
	)
	missing := fakeResponse{
		output: `Error from server (NotFound): pods "nginx" not found`,
		err:    errors.New("not found"),
	}

	t.Run("missing pod is incomplete", func(t *testing.T) {
		runner := &fakeRunner{responses: []fakeResponse{missing, missing}}
		result, err := checks.Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Wrong)
		assertContains(t, result.Message, "does not exist")
	})

	t.Run("wrong image is rejected", func(t *testing.T) {
		wrong := `{"spec":{"containers":[{"name":"web","image":"nginx:1.26"}]}}`
		runner := &fakeRunner{responses: []fakeResponse{
			{output: wrong},
			{output: wrong},
		}}
		result, err := checks.Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Wrong)
		assertContains(t, result.Message, `"nginx:1.26"`)
	})

	t.Run("declarative pod spec is accepted without Ready", func(t *testing.T) {
		manifest := `{"spec":{"containers":[{"name":"web","image":"nginx:1.27"}]}}`
		runner := &fakeRunner{responses: []fakeResponse{
			{output: manifest},
			{output: manifest},
		}}
		result, err := checks.Evaluate(context.Background(), runner)
		if err != nil {
			t.Fatal(err)
		}
		assertStatus(t, result, Success)
	})
}

func TestNewCheck(t *testing.T) {
	check, err := NewCheck(Definition{
		Type:      CheckPodReady,
		Namespace: "kubecrypt-test",
		Selector:  "app=test-workload",
		MinReady:  2,
	})
	if err != nil {
		t.Fatalf("NewCheck() error = %v", err)
	}
	got, ok := check.(PodReady)
	if !ok {
		t.Fatalf("NewCheck() type = %T, want PodReady", check)
	}
	if got.MinReady != 2 {
		t.Fatalf("MinReady = %d, want 2", got.MinReady)
	}

	if _, err := NewCheck(Definition{Type: "unknown"}); err == nil {
		t.Fatal("NewCheck() unknown type error = nil")
	}
}

func podList(items ...string) string {
	return `{"items":[` + strings.Join(items, ",") + `]}`
}

func staticCheck(result Result) Check {
	return CheckFunc(func(context.Context, Runner) (Result, error) {
		return result, nil
	})
}

func assertStatus(t *testing.T, result Result, want Status) {
	t.Helper()
	if result.Status != want {
		t.Fatalf("status = %s, want %s (message: %q)", result.Status, want, result.Message)
	}
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("%q does not contain %q", got, want)
	}
}
