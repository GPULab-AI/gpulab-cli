package api

import (
	"encoding/json"
	"testing"
)

// The serverless list payload omits the configuration block that the detail
// payload carries. Decoding an omitted field as an empty string reported every
// endpoint as having no autoscaling policy attached.
func TestServerlessServiceOmittedPolicyIsNotEmpty(t *testing.T) {
	listRow := `{"uuid":"u1","name":"flux","autoscaling_enabled":true}`

	var service ServerlessService
	if err := json.Unmarshal([]byte(listRow), &service); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if service.AutoscalingTemplateKey.Known() {
		t.Error("template key reported as known, but the payload never sent it")
	}
	if service.AutoscalingPolicyCode.Known() {
		t.Error("policy code reported as known, but the payload never sent it")
	}

	out, err := json.Marshal(service)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round map[string]interface{}
	if err := json.Unmarshal(out, &round); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	for _, key := range []string{"autoscaling_template_key", "autoscaling_policy_code", "autoscaling_metrics_config"} {
		if got, ok := round[key]; !ok || got != nil {
			t.Errorf("%s = %#v, want null so consumers can tell unknown from none", key, got)
		}
	}
}

func TestServerlessServiceDetailPolicyIsKnown(t *testing.T) {
	detailRow := `{"uuid":"u1","autoscaling_enabled":true,
		"autoscaling_template_key":"lcm_modelq_queue",
		"autoscaling_policy_code":"return 1;",
		"cuda_version":null}`

	var service ServerlessService
	if err := json.Unmarshal([]byte(detailRow), &service); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !service.AutoscalingTemplateKey.Set() {
		t.Error("template key should be set")
	}
	if got := service.AutoscalingTemplateKey.String(); got != "lcm_modelq_queue" {
		t.Errorf("template key = %q", got)
	}
	// An explicit null is an answer: the field is known, and it is empty.
	if !service.CUDAVersion.Known() {
		t.Error("an explicit null should count as known")
	}
	if service.CUDAVersion.Set() {
		t.Error("an explicit null should not count as set")
	}
}

func TestOptStringOr(t *testing.T) {
	var unknown OptString
	if got := unknown.Or("?"); got != "?" {
		t.Errorf("unknown.Or(?) = %q, want ?", got)
	}
	known := NewOptString("")
	if got := known.Or("?"); got != "" {
		t.Errorf("known-empty.Or(?) = %q, want empty", got)
	}
}
