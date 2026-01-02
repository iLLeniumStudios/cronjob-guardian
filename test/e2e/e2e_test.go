package e2e

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/iLLeniumStudios/cronjob-guardian/test/utils"
)

// namespace where the project is deployed in
const namespace = "cronjob-guardian-system"

// serviceAccountName created for the project
const serviceAccountName = "cronjob-guardian-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "cronjob-guardian-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "cronjob-guardian-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up the curl pod for metrics")
		cmd := exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace)
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("make", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("make", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				// Get the name of the controller-manager pod
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				// Validate the pod's status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("ensuring ClusterRoleBinding for metrics access exists")
			// Delete if exists (ignore errors)
			cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found")
			_, _ = utils.Run(cmd)
			// Create new binding
			cmd = exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=cronjob-guardian-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("Serving metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks
	})

	// ============================================================================
	// CronJobMonitor Lifecycle Tests
	// ============================================================================
	Context("CronJobMonitor", func() {
		const testNS = "cronjob-guardian-e2e"

		BeforeAll(func() {
			By("creating test namespace")
			cmd := exec.Command("kubectl", "create", "ns", testNS)
			_, _ = utils.Run(cmd) // Ignore error if exists
		})

		AfterAll(func() {
			By("cleaning up test namespace")
			cmd := exec.Command("kubectl", "delete", "ns", testNS, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})

		It("should create a CronJobMonitor and reach Active phase", func() {
			By("creating a test CronJob")
			cronJobYAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-test-cron
  namespace: ` + testNS + `
  labels:
    app: e2e-test
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo hello"]
          restartPolicy: Never
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJobYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating a CronJobMonitor")
			monitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-test-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      app: e2e-test
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for monitor to reach Active phase")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-test-monitor",
					"-n", testNS, "-o", "jsonpath={.status.phase}")
				output, _ := utils.Run(cmd)
				return output
			}, 60*time.Second, time.Second).Should(Equal("Active"))

			By("verifying monitor tracks the CronJob")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-test-monitor",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("1"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-test-monitor", "-n", testNS)
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "cronjob", "e2e-test-cron", "-n", testNS)
			_, _ = utils.Run(cmd)
		})

		It("should update monitor when selector changes", func() {
			By("creating two test CronJobs with different labels")
			cronJob1YAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-cron-alpha
  namespace: ` + testNS + `
  labels:
    team: alpha
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo alpha"]
          restartPolicy: Never
`
			cronJob2YAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-cron-beta
  namespace: ` + testNS + `
  labels:
    team: beta
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo beta"]
          restartPolicy: Never
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJob1YAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJob2YAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating monitor that selects team=alpha")
			monitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-selector-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      team: alpha
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor tracks only alpha CronJob")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-selector-monitor",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("1"))

			By("updating monitor to select team=beta")
			updatedMonitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-selector-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      team: beta
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", updatedMonitorYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor now tracks beta CronJob")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-selector-monitor",
					"-n", testNS, "-o", "jsonpath={.status.cronJobs[0].name}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("e2e-cron-beta"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-selector-monitor", "-n", testNS)
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "cronjob", "e2e-cron-alpha", "e2e-cron-beta", "-n", testNS)
			_, _ = utils.Run(cmd)
		})

		It("should delete monitor and clean up resources", func() {
			By("creating a CronJob and Monitor")
			cronJobYAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-delete-cron
  namespace: ` + testNS + `
  labels:
    test: delete
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo delete test"]
          restartPolicy: Never
`
			monitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-delete-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      test: delete
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJobYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for monitor to be ready")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-delete-monitor",
					"-n", testNS, "-o", "jsonpath={.status.phase}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("Active"))

			By("deleting the monitor")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-delete-monitor", "-n", testNS)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor is deleted")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-delete-monitor", "-n", testNS)
				_, err := utils.Run(cmd)
				return err != nil // Should error because resource doesn't exist
			}, 30*time.Second, time.Second).Should(BeTrue())

			By("cleaning up CronJob")
			cmd = exec.Command("kubectl", "delete", "cronjob", "e2e-delete-cron", "-n", testNS)
			_, _ = utils.Run(cmd)
		})

		It("should match CronJobs by label selector", func() {
			By("creating CronJobs with different labels")
			for _, team := range []string{"frontend", "backend", "data"} {
				yaml := fmt.Sprintf(`
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-label-%s
  namespace: %s
  labels:
    department: engineering
    team: %s
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo %s"]
          restartPolicy: Never
`, team, testNS, team, team)
				cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", yaml))
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating monitor that selects all engineering CronJobs")
			monitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-label-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      department: engineering
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor tracks all 3 CronJobs")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-label-monitor",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("3"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-label-monitor", "-n", testNS)
			_, _ = utils.Run(cmd)
			for _, team := range []string{"frontend", "backend", "data"} {
				cmd = exec.Command("kubectl", "delete", "cronjob", fmt.Sprintf("e2e-label-%s", team), "-n", testNS)
				_, _ = utils.Run(cmd)
			}
		})

		It("should match CronJobs by name selector", func() {
			By("creating named CronJobs")
			for _, name := range []string{"cron-one", "cron-two", "cron-three"} {
				yaml := fmt.Sprintf(`
apiVersion: batch/v1
kind: CronJob
metadata:
  name: %s
  namespace: %s
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo %s"]
          restartPolicy: Never
`, name, testNS, name)
				cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", yaml))
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred())
			}

			By("creating monitor that selects only cron-one and cron-two by name")
			monitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-name-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchNames:
      - cron-one
      - cron-two
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor tracks only 2 CronJobs")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-name-monitor",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("2"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-name-monitor", "-n", testNS)
			_, _ = utils.Run(cmd)
			for _, name := range []string{"cron-one", "cron-two", "cron-three"} {
				cmd = exec.Command("kubectl", "delete", "cronjob", name, "-n", testNS)
				_, _ = utils.Run(cmd)
			}
		})

		It("should handle same CronJob matched by multiple monitors", func() {
			By("creating a CronJob with multiple labels")
			cronJobYAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-multi-cron
  namespace: ` + testNS + `
  labels:
    app: multi-test
    tier: backend
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo multi"]
          restartPolicy: Never
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJobYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating first monitor that selects app=multi-test")
			monitor1YAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-multi-monitor-1
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      app: multi-test
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitor1YAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating second monitor that selects tier=backend")
			monitor2YAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-multi-monitor-2
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      tier: backend
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitor2YAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying both monitors track the same CronJob")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-multi-monitor-1",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("1"))

			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-multi-monitor-2",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("1"))

			By("verifying both monitors are Active")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-multi-monitor-1",
					"-n", testNS, "-o", "jsonpath={.status.phase}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("Active"))

			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-multi-monitor-2",
					"-n", testNS, "-o", "jsonpath={.status.phase}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("Active"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-multi-monitor-1", "e2e-multi-monitor-2", "-n", testNS)
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "cronjob", "e2e-multi-cron", "-n", testNS)
			_, _ = utils.Run(cmd)
		})

		It("should transition through status phases", func() {
			By("creating a monitor without any matching CronJobs")
			monitorYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: e2e-phase-monitor
  namespace: ` + testNS + `
spec:
  selector:
    matchLabels:
      phase-test: "true"
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor shows 0 CronJobs initially")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-phase-monitor",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("0"))

			By("creating a CronJob that matches the selector")
			cronJobYAML := `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: e2e-phase-cron
  namespace: ` + testNS + `
  labels:
    phase-test: "true"
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo phase"]
          restartPolicy: Never
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJobYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying monitor transitions to Active with CronJob tracked")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-phase-monitor",
					"-n", testNS, "-o", "jsonpath={.status.phase}")
				output, _ := utils.Run(cmd)
				return output
			}, 60*time.Second, time.Second).Should(Equal("Active"))

			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "cronjobmonitor", "e2e-phase-monitor",
					"-n", testNS, "-o", "jsonpath={.status.summary.totalCronJobs}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("1"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "cronjobmonitor", "e2e-phase-monitor", "-n", testNS)
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "cronjob", "e2e-phase-cron", "-n", testNS)
			_, _ = utils.Run(cmd)
		})
	})

	// ============================================================================
	// AlertChannel Lifecycle Tests
	// ============================================================================
	Context("AlertChannel", func() {
		const channelSecretNS = "cronjob-guardian-system"

		It("should create a webhook AlertChannel and reach Ready status", func() {
			By("creating a secret containing the webhook URL")
			secretYAML := `
apiVersion: v1
kind: Secret
metadata:
  name: e2e-webhook-url
  namespace: cronjob-guardian-system
type: Opaque
stringData:
  url: "http://mock-receiver.default.svc:8080/webhook"
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", secretYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating a webhook AlertChannel")
			channelYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: e2e-webhook-channel
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: e2e-webhook-url
      namespace: cronjob-guardian-system
      key: url
    method: POST
    headers:
      Content-Type: application/json
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", channelYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for channel to reach Ready status")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "alertchannel", "e2e-webhook-channel",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, _ := utils.Run(cmd)
				return output
			}, 60*time.Second, time.Second).Should(Equal("True"))

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "alertchannel", "e2e-webhook-channel")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "secret", "e2e-webhook-url", "-n", channelSecretNS)
			_, _ = utils.Run(cmd)
		})

		It("should update AlertChannel configuration", func() {
			By("creating a secret for the initial URL")
			secretYAML := `
apiVersion: v1
kind: Secret
metadata:
  name: e2e-update-url
  namespace: cronjob-guardian-system
type: Opaque
stringData:
  url: "http://initial-receiver:8080/webhook"
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", secretYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating initial webhook AlertChannel")
			channelYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: e2e-update-channel
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: e2e-update-url
      namespace: cronjob-guardian-system
      key: url
    method: POST
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", channelYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for channel to be ready")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "alertchannel", "e2e-update-channel",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("True"))

			By("updating the secret with new URL")
			updatedSecretYAML := `
apiVersion: v1
kind: Secret
metadata:
  name: e2e-update-url
  namespace: cronjob-guardian-system
type: Opaque
stringData:
  url: "http://updated-receiver:8080/webhook"
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", updatedSecretYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying secret was updated")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "secret", "e2e-update-url", "-n", channelSecretNS,
					"-o", "jsonpath={.data.url}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("aHR0cDovL3VwZGF0ZWQtcmVjZWl2ZXI6ODA4MC93ZWJob29r")) // base64 encoded

			By("cleaning up")
			cmd = exec.Command("kubectl", "delete", "alertchannel", "e2e-update-channel")
			_, _ = utils.Run(cmd)
			cmd = exec.Command("kubectl", "delete", "secret", "e2e-update-url", "-n", channelSecretNS)
			_, _ = utils.Run(cmd)
		})

		It("should delete AlertChannel and remove from dispatcher", func() {
			By("creating a secret for the delete test")
			secretYAML := `
apiVersion: v1
kind: Secret
metadata:
  name: e2e-delete-url
  namespace: cronjob-guardian-system
type: Opaque
stringData:
  url: "http://delete-test:8080/webhook"
`
			cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", secretYAML))
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("creating an AlertChannel")
			channelYAML := `
apiVersion: guardian.illenium.net/v1alpha1
kind: AlertChannel
metadata:
  name: e2e-delete-channel
spec:
  type: webhook
  webhook:
    urlSecretRef:
      name: e2e-delete-url
      namespace: cronjob-guardian-system
      key: url
    method: POST
`
			cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", channelYAML))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("waiting for channel to be ready")
			Eventually(func() string {
				cmd := exec.Command("kubectl", "get", "alertchannel", "e2e-delete-channel",
					"-o", "jsonpath={.status.conditions[?(@.type=='Ready')].status}")
				output, _ := utils.Run(cmd)
				return output
			}, 30*time.Second, time.Second).Should(Equal("True"))

			By("deleting the channel")
			cmd = exec.Command("kubectl", "delete", "alertchannel", "e2e-delete-channel")
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred())

			By("verifying channel is deleted")
			Eventually(func() bool {
				cmd := exec.Command("kubectl", "get", "alertchannel", "e2e-delete-channel")
				_, err := utils.Run(cmd)
				return err != nil
			}, 30*time.Second, time.Second).Should(BeTrue())

			By("cleaning up secret")
			cmd = exec.Command("kubectl", "delete", "secret", "e2e-delete-url", "-n", channelSecretNS)
			_, _ = utils.Run(cmd)
		})
	})

	// ============================================================================
	// Job Execution Recording Tests
	// ============================================================================
	Context("Job Execution", func() {
		const testNS = "cronjob-guardian-e2e"

		BeforeAll(func() {
			By("creating test namespace")
			cmd := exec.Command("kubectl", "create", "ns", testNS)
			_, _ = utils.Run(cmd)
		})

		AfterAll(func() {
			By("cleaning up test namespace")
			cmd := exec.Command("kubectl", "delete", "ns", testNS, "--ignore-not-found")
			_, _ = utils.Run(cmd)
		})

		It("should record successful job execution", func() {
			cronName := "e2e-success-cron"
			monitorName := "e2e-success-monitor"
			labelValue := "success"
			exitCode := 0
			backoffLimit := 1

			testJobExecution(testNS, cronName, monitorName, labelValue, exitCode, backoffLimit, true)
		})

		It("should record failed job execution with exit code", func() {
			cronName := "e2e-fail-cron"
			monitorName := "e2e-fail-monitor"
			labelValue := "failure"
			exitCode := 1
			backoffLimit := 0

			testJobExecution(testNS, cronName, monitorName, labelValue, exitCode, backoffLimit, false)
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}

// testJobExecution is a helper function that creates a CronJob, monitor, triggers a job,
// and verifies the execution is recorded. It handles both success and failure cases.
//
//nolint:unparam // backoffLimit varies between test cases
func testJobExecution(
	testNS, cronName, monitorName, labelValue string,
	exitCode, backoffLimit int,
	expectSuccess bool,
) {
	By(fmt.Sprintf("creating a CronJob that %s", map[bool]string{true: "succeeds", false: "fails"}[expectSuccess]))

	cronJobYAML := fmt.Sprintf(`
apiVersion: batch/v1
kind: CronJob
metadata:
  name: %s
  namespace: %s
  labels:
    job-test: %s
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      backoffLimit: %d
      template:
        spec:
          containers:
          - name: main
            image: busybox:latest
            command: ["/bin/sh", "-c", "echo 'Job execution'; exit %d"]
          restartPolicy: Never
`, cronName, testNS, labelValue, backoffLimit, exitCode)

	cmd := exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", cronJobYAML))
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	By("creating a monitor for the CronJob")
	monitorYAML := fmt.Sprintf(`
apiVersion: guardian.illenium.net/v1alpha1
kind: CronJobMonitor
metadata:
  name: %s
  namespace: %s
spec:
  selector:
    matchLabels:
      job-test: %s
`, monitorName, testNS, labelValue)

	cmd = exec.Command("sh", "-c", fmt.Sprintf("echo '%s' | kubectl apply -f -", monitorYAML))
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	By("manually triggering the CronJob")
	jobName := fmt.Sprintf("%s-manual-%d", cronName, time.Now().Unix())
	cmd = exec.Command("kubectl", "create", "job", jobName,
		"--from", fmt.Sprintf("cronjob/%s", cronName), "-n", testNS)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred())

	if expectSuccess {
		By("waiting for job to complete successfully")
		Eventually(func() string {
			cmd := exec.Command("kubectl", "get", "job", jobName,
				"-n", testNS, "-o", "jsonpath={.status.succeeded}")
			output, _ := utils.Run(cmd)
			return output
		}, 2*time.Minute, time.Second).Should(Equal("1"))

		By("verifying monitor status shows execution")
		Eventually(func() int {
			cmd := exec.Command("kubectl", "get", "cronjobmonitor", monitorName,
				"-n", testNS, "-o", "jsonpath={.status.cronJobs[0].metrics.totalRuns}")
			output, _ := utils.Run(cmd)
			var count int
			_, _ = fmt.Sscanf(output, "%d", &count)
			return count
		}, 60*time.Second, time.Second).Should(BeNumerically(">=", 1))
	} else {
		By("waiting for job to fail")
		Eventually(func() string {
			cmd := exec.Command("kubectl", "get", "job", jobName,
				"-n", testNS, "-o", "jsonpath={.status.failed}")
			output, _ := utils.Run(cmd)
			return output
		}, 2*time.Minute, time.Second).Should(Equal("1"))

		By("verifying monitor tracks the failed execution")
		Eventually(func() int {
			cmd := exec.Command("kubectl", "get", "cronjobmonitor", monitorName,
				"-n", testNS, "-o", "jsonpath={.status.cronJobs[0].metrics.failedRuns}")
			output, _ := utils.Run(cmd)
			var count int
			_, _ = fmt.Sscanf(output, "%d", &count)
			return count
		}, 60*time.Second, time.Second).Should(BeNumerically(">=", 1))
	}

	By("cleaning up")
	cmd = exec.Command("kubectl", "delete", "cronjobmonitor", monitorName, "-n", testNS)
	_, _ = utils.Run(cmd)
	cmd = exec.Command("kubectl", "delete", "cronjob", cronName, "-n", testNS)
	_, _ = utils.Run(cmd)
	cmd = exec.Command("kubectl", "delete", "job", jobName, "-n", testNS)
	_, _ = utils.Run(cmd)
}
