/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	guardianv1alpha1 "github.com/iLLeniumStudios/cronjob-guardian/api/v1alpha1"
	"github.com/iLLeniumStudios/cronjob-guardian/test/utils"
)

const (
	// DefaultTimeout is the default timeout for waiting operations
	DefaultTimeout = 2 * time.Minute
	// DefaultInterval is the default polling interval
	DefaultInterval = time.Second
	// TestNamespace is the namespace for E2E tests
	TestNamespace = "cronjob-guardian-e2e"
)

// CreateNamespace creates a test namespace
func CreateNamespace(name string) error {
	cmd := exec.Command("kubectl", "create", "ns", name)
	_, err := utils.Run(cmd)
	return err
}

// DeleteNamespace deletes a namespace
func DeleteNamespace(name string) error {
	cmd := exec.Command("kubectl", "delete", "ns", name, "--ignore-not-found")
	_, err := utils.Run(cmd)
	return err
}

// ApplyYAML applies a YAML file using kubectl
func ApplyYAML(path string) error {
	cmd := exec.Command("kubectl", "apply", "-f", path)
	_, err := utils.Run(cmd)
	return err
}

// DeleteYAML deletes resources defined in a YAML file
func DeleteYAML(path string) error {
	cmd := exec.Command("kubectl", "delete", "-f", path, "--ignore-not-found")
	_, err := utils.Run(cmd)
	return err
}

// CreateCronJob creates a CronJob in the specified namespace
func CreateCronJob(name, namespace, schedule string, successfulImage bool) error {
	image := "busybox"
	command := []string{"/bin/sh", "-c", "echo 'Hello'; exit 0"}
	if !successfulImage {
		command = []string{"/bin/sh", "-c", "echo 'Failed'; exit 1"}
	}

	yaml := fmt.Sprintf(`
apiVersion: batch/v1
kind: CronJob
metadata:
  name: %s
  namespace: %s
spec:
  schedule: "%s"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: %s
            command: %v
          restartPolicy: Never
`, name, namespace, schedule, image, command)

	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = nil
	// Use echo to pipe the yaml
	fullCmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", yaml))
	_, err := utils.Run(fullCmd)
	return err
}

// TriggerCronJob manually triggers a CronJob by creating a Job
func TriggerCronJob(cronJobName, namespace string) (string, error) {
	jobName := fmt.Sprintf("%s-manual-%d", cronJobName, time.Now().Unix())
	cmd := exec.Command("kubectl", "create", "job", jobName,
		"--from", fmt.Sprintf("cronjob/%s", cronJobName),
		"-n", namespace)
	_, err := utils.Run(cmd)
	return jobName, err
}

// WaitForJobCompletion waits for a job to complete (success or failure)
func WaitForJobCompletion(jobName, namespace string, timeout time.Duration) error {
	cmd := exec.Command("kubectl", "wait", "job", jobName,
		"--for=condition=complete",
		"-n", namespace,
		fmt.Sprintf("--timeout=%s", timeout.String()))
	_, err := utils.Run(cmd)
	if err != nil {
		// Check if it failed instead
		cmd = exec.Command("kubectl", "wait", "job", jobName,
			"--for=condition=failed",
			"-n", namespace,
			fmt.Sprintf("--timeout=%s", timeout.String()))
		_, err = utils.Run(cmd)
	}
	return err
}

// GetCronJobMonitor retrieves a CronJobMonitor by name
func GetCronJobMonitor(name, namespace string) (*guardianv1alpha1.CronJobMonitor, error) {
	cmd := exec.Command("kubectl", "get", "cronjobmonitor", name,
		"-n", namespace,
		"-o", "jsonpath={.status.phase}")
	output, err := utils.Run(cmd)
	if err != nil {
		return nil, err
	}

	monitor := &guardianv1alpha1.CronJobMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: guardianv1alpha1.CronJobMonitorStatus{
			Phase: output,
		},
	}
	return monitor, nil
}

// WaitForMonitorPhase waits for a CronJobMonitor to reach a specific phase
func WaitForMonitorPhase(name, namespace string, phase string, timeout time.Duration) {
	gomega.Eventually(func() string {
		cmd := exec.Command("kubectl", "get", "cronjobmonitor", name,
			"-n", namespace,
			"-o", "jsonpath={.status.phase}")
		output, err := utils.Run(cmd)
		if err != nil {
			return ""
		}
		return output
	}, timeout, DefaultInterval).Should(gomega.Equal(phase))
}

// WaitForChannelReady waits for an AlertChannel to become ready
func WaitForChannelReady(name string, timeout time.Duration) {
	gomega.Eventually(func() string {
		cmd := exec.Command("kubectl", "get", "alertchannel", name,
			"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
		output, err := utils.Run(cmd)
		if err != nil {
			return ""
		}
		return output
	}, timeout, DefaultInterval).Should(gomega.Equal("True"))
}

// GetMonitorCronJobCount returns the number of CronJobs tracked by a monitor
func GetMonitorCronJobCount(name, namespace string) (int, error) {
	cmd := exec.Command("kubectl", "get", "cronjobmonitor", name,
		"-n", namespace,
		"-o", "jsonpath={.status.summary.totalCronJobs}")
	output, err := utils.Run(cmd)
	if err != nil {
		return 0, err
	}
	var count int
	_, err = fmt.Sscanf(output, "%d", &count)
	return count, err
}

// CreateTestSecret creates a secret for testing
func CreateTestSecret(name, namespace string, data map[string]string) error {
	args := []string{"create", "secret", "generic", name, "-n", namespace}
	for k, v := range data {
		args = append(args, fmt.Sprintf("--from-literal=%s=%s", k, v))
	}
	cmd := exec.Command("kubectl", args...)
	_, err := utils.Run(cmd)
	return err
}

// DeleteResource deletes a Kubernetes resource
func DeleteResource(kind, name, namespace string) error {
	args := []string{"delete", kind, name, "--ignore-not-found"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	cmd := exec.Command("kubectl", args...)
	_, err := utils.Run(cmd)
	return err
}

// ResourceExists checks if a resource exists
func ResourceExists(kind, name, namespace string) bool {
	args := []string{"get", kind, name}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	cmd := exec.Command("kubectl", args...)
	_, err := utils.Run(cmd)
	return err == nil
}

// Placeholder types for compilation - these would be replaced by actual k8s client calls
var _ = types.NamespacedName{}
var _ = batchv1.CronJob{}
var _ = corev1.Secret{}
var _ = context.Background()
