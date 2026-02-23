package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dsablic/codemium/internal/provider"
)

func TestBitbucketListProjects(t *testing.T) {
	page := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2.0/workspaces/myws/projects" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		page++
		if page == 1 {
			json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{
					{"key": "PROJ1", "name": "Project One"},
					{"key": "PROJ2", "name": "Project Two"},
				},
				"next": "http://" + r.Host + "/2.0/workspaces/myws/projects?page=2",
			})
		} else {
			json.NewEncoder(w).Encode(map[string]any{
				"values": []map[string]any{
					{"key": "PROJ3", "name": "Project Three"},
				},
			})
		}
	}))
	defer server.Close()

	bb := provider.NewBitbucket("token", "user", server.URL, nil)
	projects, err := bb.ListProjects(context.Background(), "myws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(projects) != 3 {
		t.Fatalf("expected 3 projects, got %d", len(projects))
	}
	if projects[0].Key != "PROJ1" || projects[0].Name != "Project One" {
		t.Errorf("unexpected first project: %+v", projects[0])
	}
	if projects[2].Key != "PROJ3" {
		t.Errorf("unexpected third project: %+v", projects[2])
	}
}

func TestBitbucketListProjectsAuth(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"values": []map[string]any{}})
	}))
	defer server.Close()

	bb := provider.NewBitbucket("token", "user", server.URL, nil)
	bb.ListProjects(context.Background(), "ws")

	if len(gotAuth) < 6 || gotAuth[:6] != "Basic " {
		t.Errorf("expected Basic auth, got %s", gotAuth)
	}
}
