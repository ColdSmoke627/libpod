package hooks

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	current "github.com/projectatomic/libpod/pkg/hooks/1.0.0"
	"github.com/stretchr/testify/assert"
	"golang.org/x/text/language"
)

// path is the path to an example hook executable.
var path string

func TestGoodNew(t *testing.T) {
	ctx := context.Background()

	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	for i, name := range []string{
		"01-my-hook.json",
		"01-UPPERCASE.json",
		"02-another-hook.json",
	} {
		jsonPath := filepath.Join(dir, name)
		var extraStages string
		if i == 0 {
			extraStages = ", \"poststart\", \"poststop\""
		}
		err = ioutil.WriteFile(jsonPath, []byte(fmt.Sprintf("{\"version\": \"1.0.0\", \"hook\": {\"path\": \"%s\", \"timeout\": %d}, \"when\": {\"always\": true}, \"stages\": [\"prestart\"%s]}", path, i+1, extraStages)), 0644)
		if err != nil {
			t.Fatal(err)
		}
	}

	lang, err := language.Parse("und-u-va-posix")
	if err != nil {
		t.Fatal(err)
	}

	manager, err := New(ctx, []string{dir}, []string{}, lang)
	if err != nil {
		t.Fatal(err)
	}

	config := &rspec.Spec{}
	extensionStages, err := manager.Hooks(config, map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}

	one := 1
	two := 2
	three := 3
	assert.Equal(t, &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
			{
				Path:    path,
				Timeout: &two,
			},
			{
				Path:    path,
				Timeout: &three,
			},
		},
		Poststart: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
		},
		Poststop: []rspec.Hook{
			{
				Path:    path,
				Timeout: &one,
			},
		},
	}, config.Hooks)

	var nilExtensionStages map[string][]rspec.Hook
	assert.Equal(t, nilExtensionStages, extensionStages)
}

func TestBadNew(t *testing.T) {
	ctx := context.Background()

	dir, err := ioutil.TempDir("", "hooks-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	jsonPath := filepath.Join(dir, "a.json")
	err = ioutil.WriteFile(jsonPath, []byte("{\"version\": \"-1\"}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	lang, err := language.Parse("und-u-va-posix")
	if err != nil {
		t.Fatal(err)
	}

	_, err = New(ctx, []string{dir}, []string{}, lang)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^parsing hook \"[^\"]*a.json\": unrecognized hook version: \"-1\"$", err.Error())
}

func TestBrokenMatch(t *testing.T) {
	manager := Manager{
		hooks: map[string]*current.Hook{
			"a.json": {
				Version: current.Version,
				Hook: rspec.Hook{
					Path: "/a/b/c",
				},
				When: current.When{
					Commands: []string{"["},
				},
				Stages: []string{"prestart"},
			},
		},
	}
	config := &rspec.Spec{
		Process: &rspec.Process{
			Args: []string{"/bin/sh"},
		},
	}
	extensionStages, err := manager.Hooks(config, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^matching hook \"a\\.json\": command: error parsing regexp: .*", err.Error())

	var nilExtensionStages map[string][]rspec.Hook
	assert.Equal(t, nilExtensionStages, extensionStages)
}

func TestInvalidStage(t *testing.T) {
	always := true
	manager := Manager{
		hooks: map[string]*current.Hook{
			"a.json": {
				Version: current.Version,
				Hook: rspec.Hook{
					Path: "/a/b/c",
				},
				When: current.When{
					Always: &always,
				},
				Stages: []string{"does-not-exist"},
			},
		},
	}
	extensionStages, err := manager.Hooks(&rspec.Spec{}, map[string]string{}, false)
	if err == nil {
		t.Fatal("unexpected success")
	}
	assert.Regexp(t, "^hook \"a\\.json\": unknown stage \"does-not-exist\"$", err.Error())

	var nilExtensionStages map[string][]rspec.Hook
	assert.Equal(t, nilExtensionStages, extensionStages)
}

func TestExtensionStage(t *testing.T) {
	always := true
	manager := Manager{
		hooks: map[string]*current.Hook{
			"a.json": {
				Version: current.Version,
				Hook: rspec.Hook{
					Path: "/a/b/c",
				},
				When: current.When{
					Always: &always,
				},
				Stages: []string{"prestart", "a", "b"},
			},
		},
		extensionStages: []string{"a", "b", "c"},
	}

	config := &rspec.Spec{}
	extensionStages, err := manager.Hooks(config, map[string]string{}, false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, &rspec.Hooks{
		Prestart: []rspec.Hook{
			{
				Path: "/a/b/c",
			},
		},
	}, config.Hooks)

	assert.Equal(t, map[string][]rspec.Hook{
		"a": {
			{
				Path: "/a/b/c",
			},
		},
		"b": {
			{
				Path: "/a/b/c",
			},
		},
	}, extensionStages)
}

func init() {
	if runtime.GOOS != "windows" {
		path = "/bin/sh"
	} else {
		panic("we need a reliable executable path on Windows")
	}
}
