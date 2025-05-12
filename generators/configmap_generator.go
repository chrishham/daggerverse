package generators

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Input struct {
	Namespace  string       `json:"namespace"`
	AppName    string       `json:"appName"`
	ConfigMaps []ConfigData `json:"configMaps"`
}

type ConfigData struct {
	Name string            `json:"name"`
	Data map[string]string `json:"data"`
}

type ConfigMap struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   Metadata          `yaml:"metadata"`
	Data       map[string]string `yaml:"data"`
}

type Metadata struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

// Custom YAML encoder that wraps all data values in single quotes
func encodeYAMLWithQuotes(configMap ConfigMap) ([]byte, error) {
	node := yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "apiVersion"},
			{Kind: yaml.ScalarNode, Value: configMap.APIVersion},

			{Kind: yaml.ScalarNode, Value: "kind"},
			{Kind: yaml.ScalarNode, Value: configMap.Kind},

			{Kind: yaml.ScalarNode, Value: "metadata"},
			metadataNode(configMap.Metadata),

			{Kind: yaml.ScalarNode, Value: "data"},
			dataNode(configMap.Data),
		},
	}
	return yaml.Marshal(&node)
}

func metadataNode(meta Metadata) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "name"},
			{Kind: yaml.ScalarNode, Value: meta.Name},
			{Kind: yaml.ScalarNode, Value: "namespace"},
			{Kind: yaml.ScalarNode, Value: meta.Namespace},
		},
	}
}

func dataNode(data map[string]string) *yaml.Node {
	content := []*yaml.Node{}
	for k, v := range data {
		content = append(content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			&yaml.Node{Kind: yaml.ScalarNode, Value: v, Style: yaml.SingleQuotedStyle},
		)
	}
	return &yaml.Node{Kind: yaml.MappingNode, Content: content}
}

func main() {
	inputFile := flag.String("file", "", "Path to JSON file")
	flag.Parse()

	if *inputFile == "" {
		fmt.Println("[-] Error: --file is required")
		os.Exit(1)
	}

	file, err := os.ReadFile(*inputFile)
	if err != nil {
		fmt.Printf("[-] Error reading file: %v\n", err)
		os.Exit(1)
	}

	var input Input
	if err := json.Unmarshal(file, &input); err != nil {
		fmt.Printf("[-] Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	if len(input.ConfigMaps) == 0 {
		fmt.Println("[INFO] No ConfigMaps found in JSON.")
		os.Exit(0)
	}

	appName := input.AppName + "-config"

	for _, cm := range input.ConfigMaps {
		configMap := ConfigMap{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Metadata: Metadata{
				Name:      appName,
				Namespace: input.Namespace,
			},
			Data: cm.Data,
		}

		yamlBytes, err := encodeYAMLWithQuotes(configMap)
		if err != nil {
			fmt.Printf("[-] Error encoding YAML: %v\n", err)
			continue
		}

		outputFile := fmt.Sprintf("%s-configmap.yaml", cm.Name)
		if err := os.WriteFile(outputFile, yamlBytes, 0644); err != nil {
			fmt.Printf("[-] Error writing YAML file: %v\n", err)
			continue
		}

		fmt.Printf("[INFO] ConfigMap YAML generated: %s\n", outputFile)
	}
}
