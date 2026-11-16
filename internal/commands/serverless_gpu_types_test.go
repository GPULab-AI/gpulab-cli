package commands

import (
	"reflect"
	"strings"
	"testing"

	"github.com/GPULab-AI/gpulab-cli/internal/api"
)

// Mirrors the real gpu_types rows: names repeat with different VRAM.
var testGPUOptions = []api.ServerlessGPUTypeOption{
	{ID: 1, GPUName: "NVIDIA GeForce RTX 4090", GPUTotalMemory: 24576, FreeGPUsCount: 2},
	{ID: 2, GPUName: "NVIDIA GeForce RTX 3090 Ti", GPUTotalMemory: 24576, FreeGPUsCount: 2},
	{ID: 3, GPUName: "NVIDIA GeForce RTX 3090", GPUTotalMemory: 24576, FreeGPUsCount: 18},
	{ID: 4, GPUName: "NVIDIA GeForce RTX 4090", GPUTotalMemory: 49140, FreeGPUsCount: 0},
}

func TestResolveGPUTypeByIDAndUniqueName(t *testing.T) {
	cases := map[string]int{
		"3":                          3,
		"NVIDIA GeForce RTX 3090 Ti": 2,
		"nvidia geforce rtx 3090":    3, // exact (case-insensitive) beats substring
		"3090 Ti":                    2,
		"RTX 3090":                   3, // short name, not the Ti
		"3090":                       3, // whole-word suffix, not the Ti
		"ti":                         2,
	}
	for in, want := range cases {
		got, err := resolveServerlessGPUTypeOption(testGPUOptions, in)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", in, err)
		}
		if got.ID != want {
			t.Fatalf("%q: got id %d, want %d", in, got.ID, want)
		}
	}
}

func TestResolveGPUTypeAmbiguousNameListsIDs(t *testing.T) {
	for _, in := range []string{"NVIDIA GeForce RTX 4090", "4090", "RTX 4090", "GeForce"} {
		_, err := resolveServerlessGPUTypeOption(testGPUOptions, in)
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("%q: want ambiguous error, got %v", in, err)
		}
	}
	_, err := resolveServerlessGPUTypeOption(testGPUOptions, "4090")
	if !strings.Contains(err.Error(), "1 = ") || !strings.Contains(err.Error(), "4 = ") || !strings.Contains(err.Error(), "48GB") {
		t.Fatalf("error should list the candidate IDs with VRAM: %v", err)
	}
}

func TestGPUTypeListSetAddRemove(t *testing.T) {
	got, err := resolveServerlessGPUTypeList(testGPUOptions, []int{1}, []string{"1", "3"}, nil, nil)
	if err != nil || !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("set: got %v, %v", got, err)
	}

	got, err = resolveServerlessGPUTypeList(testGPUOptions, []int{1}, nil, []string{"3090 Ti", "3"}, nil)
	if err != nil || !reflect.DeepEqual(got, []int{1, 2, 3}) {
		t.Fatalf("add keeps order and appends: got %v, %v", got, err)
	}

	got, err = resolveServerlessGPUTypeList(testGPUOptions, []int{1, 2, 3}, nil, []string{"1"}, []string{"2"})
	if err != nil || !reflect.DeepEqual(got, []int{1, 3}) {
		t.Fatalf("remove + dedupe: got %v, %v", got, err)
	}

	if _, err := resolveServerlessGPUTypeList(testGPUOptions, []int{1}, nil, nil, []string{"1"}); err == nil {
		t.Fatal("removing the last type must fail")
	}

	if _, err := resolveServerlessGPUTypeList(testGPUOptions, nil, []string{"1", "2", "3", "4", "1", "2"}, nil, nil); err != nil {
		t.Fatalf("duplicates collapse before the max check: %v", err)
	}
}

func TestServiceGPUTypeIDsFallsBackToSingleType(t *testing.T) {
	legacy := api.ServerlessService{GPUTypeID: 3}
	if !reflect.DeepEqual(legacy.ServiceGPUTypeIDs(), []int{3}) || legacy.SupportsGPUTypeList() {
		t.Fatalf("legacy payload: %v %v", legacy.ServiceGPUTypeIDs(), legacy.SupportsGPUTypeList())
	}

	multi := api.ServerlessService{GPUTypeID: 1, GPUTypes: []api.ServerlessServiceGPUType{{ID: 1}, {ID: 3}}}
	if !reflect.DeepEqual(multi.ServiceGPUTypeIDs(), []int{1, 3}) || !multi.SupportsGPUTypeList() {
		t.Fatalf("multi payload: %v", multi.ServiceGPUTypeIDs())
	}
}

func TestGPUSummary(t *testing.T) {
	s := api.ServerlessService{GPUTypes: []api.ServerlessServiceGPUType{
		{ID: 1, Name: "NVIDIA GeForce RTX 4090", VRAMMB: 24576},
		{ID: 3, Name: "NVIDIA GeForce RTX 3090", VRAMMB: 24576},
	}}
	if got := serverlessGPUSummary(s); got != "RTX 4090 24GB → RTX 3090 24GB" {
		t.Fatalf("got %q", got)
	}
}
