package commands

import (
	"encoding/json"
	"testing"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
)

func logsFromJSON(t *testing.T, payload string) *api.ServerlessPage[api.ServerlessAutoscalingLog] {
	t.Helper()
	var page api.ServerlessPage[api.ServerlessAutoscalingLog]
	if err := json.Unmarshal([]byte(payload), &page); err != nil {
		t.Fatalf("unmarshal logs: %v", err)
	}
	return &page
}

// current_replicas and desired_replicas are both written after reconciliation,
// so they read equal even on an evaluation that replaced a replica. Comparing
// them counted no scaling at all on an endpoint churning every minute.
func TestAutoscalingStatsCountsScalingFromReconcile(t *testing.T) {
	page := logsFromJSON(t, `{"data":[
		{"created_at":"2026-08-20T14:45:35+05:30","level":"info","event":"evaluation",
		 "current_replicas":1,"desired_replicas":1,
		 "context":{"reconcile":{"current":1,"desired":1,"scaled_up":1,"scaled_down":0}}},
		{"created_at":"2026-08-20T14:44:36+05:30","level":"info","event":"evaluation",
		 "current_replicas":1,"desired_replicas":1,
		 "context":{"reconcile":{"current":0,"desired":1,"scaled_up":1,"scaled_down":1}}}
	]}`)

	got := computeAutoscalingStats(page)
	if got.ScaleUps != 2 {
		t.Errorf("scale-ups = %d, want 2", got.ScaleUps)
	}
	if got.ScaleDowns != 1 {
		t.Errorf("scale-downs = %d, want 1", got.ScaleDowns)
	}
	if got.DerivedLastScaledAt != "2026-08-20T14:45:35+05:30" {
		t.Errorf("derived last scaled = %q, want the newest scaling evaluation", got.DerivedLastScaledAt)
	}
}

// The policy asking for replicas it never receives is the real symptom of a
// starved endpoint. The HTTP counters stay clean throughout.
func TestAutoscalingStatsCountsBlockedEvaluations(t *testing.T) {
	page := logsFromJSON(t, `{"data":[
		{"created_at":"2026-08-20T14:30:00+05:30","level":"error","event":"evaluation_failed",
		 "message":"Autoscaling policy evaluation failed.","metrics":{"queue_num":36,"queue_time":67},
		 "context":{"exception":"No compatible GPU is currently available for this service."}},
		{"created_at":"2026-08-20T14:29:00+05:30","level":"info","event":"evaluation",
		 "metrics":{"queue_num":30,"queue_time":50},
		 "context":{"reconcile":{"current":1,"desired":4,"scaled_up":0,"scaled_down":0}}}
	]}`)

	got := computeAutoscalingStats(page)
	if got.Blocked != 2 {
		t.Errorf("blocked = %d, want 2 (one failed evaluation, one that got nothing)", got.Blocked)
	}
	if got.LastBlockReason != "No compatible GPU is currently available for this service." {
		t.Errorf("block reason = %q", got.LastBlockReason)
	}
	if got.LatestMetrics["queue_num"] != 36 {
		t.Errorf("latest queue_num = %v, want 36 from the newest sample", got.LatestMetrics["queue_num"])
	}
}

// A ModelQ-backed endpoint takes work off Redis, so its HTTP request queue and
// error rate read clean while a real backlog builds behind it.
func TestHealthVerdictCatchesStarvedModelQEndpoint(t *testing.T) {
	svc := api.ServerlessService{ActiveReplicasCount: 1, MinReplicas: 1, MaxReplicas: 4}
	clean := requestStats{Total: 100, Success: 100}
	starved := autoscalingStats{
		Blocked:       12,
		LatestMetrics: map[string]float64{"queue_num": 36, "queue_time": 67},
	}

	if got := healthVerdict(clean, svc, starved, nil, true); got != "unhealthy" {
		t.Errorf("verdict = %q, want unhealthy: 36 tasks queued and 12 evaluations got no capacity", got)
	}

	idle := autoscalingStats{LatestMetrics: map[string]float64{"queue_num": 0, "queue_time": 0}}
	if got := healthVerdict(clean, svc, idle, nil, true); got != "healthy" {
		t.Errorf("verdict = %q, want healthy when nothing is queued and nothing is blocked", got)
	}
}

func TestQueueBacklogIgnoresTimeMetrics(t *testing.T) {
	// queue_time is a latency, not a depth; a long wait with an empty queue
	// must not read as a backlog.
	as := autoscalingStats{LatestMetrics: map[string]float64{"queue_num": 0, "queue_time": 900}}
	name, depth, ok := queueBacklog(as)
	if !ok || name != "queue_num" {
		t.Fatalf("backlog metric = %q (ok=%v), want queue_num", name, ok)
	}
	if depth != 0 {
		t.Errorf("depth = %v, want 0", depth)
	}
}

func TestScalingCellNeverCallsUnknownNone(t *testing.T) {
	fromList := api.ServerlessService{AutoscalingEnabled: true}
	if got := scalingCell(fromList); got != "on:?" {
		t.Errorf("scalingCell(list row) = %q, want on:? — an unreported policy is not an absent one", got)
	}

	hydrated := api.ServerlessService{
		AutoscalingEnabled:     true,
		AutoscalingTemplateKey: api.NewOptString("lcm_modelq_queue"),
		AutoscalingPolicyCode:  api.NewOptString("return 1;"),
	}
	if got := scalingCell(hydrated); got != "on:lcm_modelq_queue" {
		t.Errorf("scalingCell(hydrated) = %q", got)
	}

	off := api.ServerlessService{AutoscalingEnabled: false}
	if got := scalingCell(off); got != "off" {
		t.Errorf("scalingCell(disabled) = %q, want off", got)
	}
}

// Building an update from a list row would send blank configuration fields and
// erase the endpoint's autoscaling policy.
func TestUpdateRequestRejectsServiceWithoutConfiguration(t *testing.T) {
	fromList := api.ServerlessService{Name: "flux", AutoscalingEnabled: true}
	if _, err := serverlessRequestFromService(fromList); err == nil {
		t.Fatal("building an update from a list row should fail, not silently erase the policy")
	}

	fromDetail := api.ServerlessService{
		Name:                  "flux",
		AutoscalingEnabled:    true,
		AutoscalingPolicyCode: api.NewOptString("return 1;"),
	}
	req, err := serverlessRequestFromService(fromDetail)
	if err != nil {
		t.Fatalf("building an update from a detail row: %v", err)
	}
	if req.AutoscalingPolicyCode != "return 1;" {
		t.Errorf("policy code = %q, want it carried through", req.AutoscalingPolicyCode)
	}
}

// An update request is a full replacement, so every preserved field has to go
// out on the wire in the shape the API validates — plain strings, never the
// OptString wrapper the response decodes into.
func TestUpdateRequestMarshalsPreservedFieldsAsStrings(t *testing.T) {
	req, err := serverlessRequestFromService(api.ServerlessService{
		Name:                     "flux",
		AutoscalingEnabled:       true,
		AutoscalingTemplateKey:   api.NewOptString("lcm_modelq_queue"),
		AutoscalingPolicyCode:    api.NewOptString("return 1;"),
		AutoscalingMetricsConfig: api.NewOptString(`[{"name":"queue_num"}]`),
		EnvironmentVariables:     api.NewOptString("A=1"),
		CUDAVersion:              api.NewOptString(""),
	})
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	encoded, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var wire map[string]interface{}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for field, want := range map[string]string{
		"autoscaling_template_key":   "lcm_modelq_queue",
		"autoscaling_policy_code":    "return 1;",
		"autoscaling_metrics_config": `[{"name":"queue_num"}]`,
		"environment_variables":      "A=1",
	} {
		got, ok := wire[field].(string)
		if !ok {
			t.Errorf("%s = %#v, want a string", field, wire[field])
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
}
