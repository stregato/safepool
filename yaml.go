package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

//func ReadYaml(s storage.Exchanger, name string, out interface{}) error {
//	data, err := Read(s, name)
//	if err != nil {
//		return err
//	}
//	return yaml.Unmarshal(data, out)
//}
//
//func WriteYaml(s storage.Exchanger, name string, in interface{}) error {
//	d, err := yaml.Marshal(in)
//	if err != nil {
//		return err
//	}
//	return Write(s, name, d)
//}

func ReadYamlFile(name string, out interface{}) error {
	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, out)
}

func WriteYamlFile(name string, in interface{}) error {
	d, err := yaml.Marshal(in)
	if err != nil {
		return err
	}
	return os.WriteFile(name, d, 0533)
}
