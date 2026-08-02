package domain

import (
	"encoding/json"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestContainerServiceTemplateEnvironmentはYAMLとJSONでRoundTripできる(t *testing.T) {
	want := ContainerServiceTemplate{
		SourceContainer: "myapp-web",
		Environment: map[string]string{
			"APP_ENV":        "cell",
			"EXPLICIT_EMPTY": "",
		},
	}

	yamlData, err := yaml.Marshal(want)
	if err != nil {
		t.Fatalf("YAML marshal error = %v", err)
	}
	var fromYAML ContainerServiceTemplate
	if err := yaml.Unmarshal(yamlData, &fromYAML); err != nil {
		t.Fatalf("YAML unmarshal error = %v", err)
	}
	if !reflect.DeepEqual(fromYAML, want) {
		t.Fatalf("YAML round trip = %#v, want %#v", fromYAML, want)
	}

	jsonData, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("JSON marshal error = %v", err)
	}
	var fromJSON ContainerServiceTemplate
	if err := json.Unmarshal(jsonData, &fromJSON); err != nil {
		t.Fatalf("JSON unmarshal error = %v", err)
	}
	if !reflect.DeepEqual(fromJSON, want) {
		t.Fatalf("JSON round trip = %#v, want %#v", fromJSON, want)
	}
}
