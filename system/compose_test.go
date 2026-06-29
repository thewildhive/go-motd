package system

import "testing"

func TestParseComposePSJSONLines(t *testing.T) {
	payload := []byte(`{"State":"running","Health":"healthy"}
{"State":"exited"}
{"State":"running","Health":"unhealthy"}`)
	status, err := parseComposePSJSON(payload)
	if err != nil {
		t.Fatalf("parseComposePSJSON failed: %v", err)
	}
	if status.Total != 3 || status.Online != 1 {
		t.Fatalf("unexpected compose status: %+v", status)
	}
}

func TestParseComposePSJSONArray(t *testing.T) {
	payload := []byte(`[{"State":"running","Status":"Up 2 minutes"},{"State":"exited","Status":"Exited"}]`)
	status, err := parseComposePSJSON(payload)
	if err != nil {
		t.Fatalf("parseComposePSJSON failed: %v", err)
	}
	if status.Total != 2 || status.Online != 1 {
		t.Fatalf("unexpected compose status: %+v", status)
	}
}

func TestComposeContainerOnline(t *testing.T) {
	if !composeContainerOnline(composePSLine{State: "running", Health: "healthy"}) {
		t.Fatal("expected running healthy container online")
	}
	if composeContainerOnline(composePSLine{State: "running", Health: "unhealthy"}) {
		t.Fatal("expected unhealthy container offline")
	}
	if !composeContainerOnline(composePSLine{Status: "Up 10 seconds"}) {
		t.Fatal("expected Up status online")
	}
}
