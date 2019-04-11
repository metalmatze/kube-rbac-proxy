package kubetest

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/brancz/uclient"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func CreatedManifests(config *rest.Config, paths ...string) Setup {
	client := uclient.NewForConfig(config)

	return func(ctx *ScenarioContext) error {
		for _, path := range paths {
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			if len(content) == 0 {
				return fmt.Errorf("manifest has no content: %s", path)
			}

			r := bytes.NewReader(content)

			res := map[string]interface{}{}
			err = kubeyaml.NewYAMLOrJSONDecoder(r, r.Len()).Decode(&res)
			if err != nil {
				return err
			}

			u := &unstructured.Unstructured{Object: res}
			c, err := client.ForUnstructured(u)
			if err != nil {
				return err
			}

			_, err = c.Create(u, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			ctx.AddFinalizer(func() error {
				return c.Delete(u.GetName(), nil)
			})
		}
		return nil
	}
}

// PodsAreReady waits for a number if replicas matching the given labels to be ready.
// Returns a func directly (not Setup or Conditions) as it can be used in Given and When steps
func PodsAreReady(client kubernetes.Interface, replicas int, labels string) func(*ScenarioContext) error {
	return func(ctx *ScenarioContext) error {
		return wait.Poll(time.Second, time.Minute, func() (bool, error) {
			list, err := client.CoreV1().Pods(ctx.Namespace).List(metav1.ListOptions{
				LabelSelector: labels,
			})
			if err != nil {
				return false, fmt.Errorf("failed to list pods: %v", err)
			}

			runningAndReady := 0
			for _, p := range list.Items {
				isRunningAndReady, err := podRunningAndReady(p)
				if err != nil {
					return false, err
				}

				if isRunningAndReady {
					runningAndReady++
				}
			}

			if runningAndReady == replicas {
				return true, nil
			}
			return false, nil
		})
	}
}

// podRunningAndReady returns whether a pod is running and each container has
// passed it's ready state.
func podRunningAndReady(pod corev1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case corev1.PodFailed, corev1.PodSucceeded:
		return false, fmt.Errorf("pod completed")
	case corev1.PodRunning:
		for _, cond := range pod.Status.Conditions {
			if cond.Type != corev1.PodReady {
				continue
			}
			return cond.Status == corev1.ConditionTrue, nil
		}
		return false, fmt.Errorf("pod ready condition not found")
	}
	return false, nil
}

func Sleep(d time.Duration) Condition {
	return func(ctx *ScenarioContext) error {
		time.Sleep(d)
		return nil
	}
}

type RunOptions struct {
	ServiceAccount string
}

func RunSucceeds(client kubernetes.Interface, image string, name string, command []string, opts *RunOptions) Check {
	return func(ctx *ScenarioContext) error {
		logs, err := run(client, ctx, image, name, command, opts)
		if err != nil {
			_, _ = fmt.Fprint(os.Stderr, string(logs))
			return err
		}
		return nil
	}
}

func RunFails(client kubernetes.Interface, image string, name string, command []string, opts *RunOptions) Check {
	return func(ctx *ScenarioContext) error {
		logs, err := run(client, ctx, image, name, command, opts)
		if err == nil {
			_, _ = fmt.Fprint(os.Stderr, string(logs))
			return fmt.Errorf("expected run to fail")
		}
		if err != errRun {
			return err
		}
		return nil
	}
}

var errRun = fmt.Errorf("failed to run")

// run the command and return the Check with the container's logs
func run(client kubernetes.Interface, ctx *ScenarioContext, image string, name string, command []string, opts *RunOptions) ([]byte, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ctx.Namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    name,
				Image:   image,
				Command: command,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}

	if opts != nil {
		pod.Spec.ServiceAccountName = opts.ServiceAccount
	}

	ctx.AddFinalizer(func() error {
		return client.CoreV1().Pods(ctx.Namespace).Delete(pod.ObjectMeta.Name, nil)
	})

	_, err := client.CoreV1().Pods(ctx.Namespace).Create(pod)
	if err != nil {
		return nil, fmt.Errorf("failed to create pod: %v", err)
	}

	watch, err := client.CoreV1().Pods(ctx.Namespace).Watch(metav1.SingleObject(pod.ObjectMeta))
	if err != nil {
		return nil, fmt.Errorf("failed to watch pod: %v", err)
	}

	for event := range watch.ResultChan() {
		pod := event.Object.(*corev1.Pod)
		phase := pod.Status.Phase

		if phase == corev1.PodFailed {
			logs, _ := podLogs(client, ctx.Namespace, name, name)
			return logs, errRun
		}
		if phase == corev1.PodSucceeded {
			break
		}
	}

	logs, _ := podLogs(client, ctx.Namespace, name, name)
	return logs, nil
}

func podLogs(client kubernetes.Interface, namespace, pod, container string) ([]byte, error) {
	rest := client.CoreV1().Pods(namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: container,
		Follow:    false,
	})

	stream, err := rest.Stream()
	if err != nil {
		return nil, err
	}
	return ioutil.ReadAll(stream)
}

func CreateNamespace(client kubernetes.Interface, name string) error {
	_, err := client.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create namespace with name %v", name)
	}
	return nil
}

func DeleteNamespace(client kubernetes.Interface, name string) error {
	return client.CoreV1().Namespaces().Delete(name, nil)
}
