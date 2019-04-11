package main

import (
	"fmt"
	"os"

	"github.com/brancz/uclient"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/tools/clientcmd"
)

func main() {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic("error building kubeconfig")
	}

	uc := uclient.NewForConfig(cfg)

	f, err := os.Open("example/service.yaml")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	res := map[string]interface{}{}
	err = yaml.NewYAMLOrJSONDecoder(f, 100).Decode(&res)
	if err != nil {
		panic(err)
	}

	u := &unstructured.Unstructured{Object: res}
	c, err := uc.ForUnstructured()
	if err != nil {
		panic(err)
	}

	_, err := c.Create(u)
	if err != nil {
		panic(err)
	}

	fmt.Printf("successfully created %s/%s (%s.%s)", u.GetName(), u.GetNamespace(), u.GetKind(), u.GetAPIVersion())
}
