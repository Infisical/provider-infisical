package config

import (
	"context"
	"testing"

	"github.com/crossplane/upjet/v2/pkg/config"
)

func TestWithPlaceholderID(t *testing.T) {
	e := withPlaceholderID(config.IdentifierFromProvider)

	cases := map[string]struct {
		externalName string
		wantID       string
	}{
		"NotCreatedYet": {externalName: "", wantID: emptyIDPlaceholder},
		"Created":       {externalName: "426627eb-1d56-4125-a60a-275633db7574", wantID: "426627eb-1d56-4125-a60a-275633db7574"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			id, err := e.GetIDFn(context.Background(), tc.externalName, nil, nil)
			if err != nil {
				t.Fatalf("GetIDFn: unexpected error: %v", err)
			}
			if id != tc.wantID {
				t.Errorf("GetIDFn: want %q, got %q", tc.wantID, id)
			}
		})
	}

	if _, err := e.GetExternalNameFn(map[string]any{"id": emptyIDPlaceholder}); err == nil {
		t.Error("GetExternalNameFn: want an error for the placeholder ID, got none")
	}
	if _, err := e.GetExternalNameFn(map[string]any{"id": ""}); err == nil {
		t.Error("GetExternalNameFn: want an error for an empty ID, got none")
	}
	name, err := e.GetExternalNameFn(map[string]any{"id": "426627eb-1d56-4125-a60a-275633db7574"})
	if err != nil || name != "426627eb-1d56-4125-a60a-275633db7574" {
		t.Errorf("GetExternalNameFn: want the real ID, got %q, %v", name, err)
	}
}
