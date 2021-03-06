/*
Copyright 2014 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/onsi/ginkgo"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	coreclientset "k8s.io/client-go/kubernetes/typed/core/v1"
	e2elog "k8s.io/kubernetes/test/e2e/framework/log"
	imageutils "k8s.io/kubernetes/test/utils/image"
)

const (
	// EndpointHTTPPort is an endpoint HTTP port for testing.
	EndpointHTTPPort = 8080
	// EndpointUDPPort is an endpoint UDP port for testing.
	EndpointUDPPort       = 8081
	testContainerHTTPPort = 8080
	// ClusterHTTPPort is a cluster HTTP port for testing.
	ClusterHTTPPort = 80
	// ClusterUDPPort is a cluster UDP port for testing.
	ClusterUDPPort             = 90
	testPodName                = "test-container-pod"
	hostTestPodName            = "host-test-container-pod"
	nodePortServiceName        = "node-port-service"
	sessionAffinityServiceName = "session-affinity-service"
	// wait time between poll attempts of a Service vip and/or nodePort.
	// coupled with testTries to produce a net timeout value.
	hitEndpointRetryDelay = 2 * time.Second
	// Number of retries to hit a given set of endpoints. Needs to be high
	// because we verify iptables statistical rr loadbalancing.
	testTries = 30
	// Maximum number of pods in a test, to make test work in large clusters.
	maxNetProxyPodsCount = 10
	// SessionAffinityChecks is number of checks to hit a given set of endpoints when enable session affinity.
	SessionAffinityChecks = 10
	// RegexIPv4 is a regex to match IPv4 addresses
	RegexIPv4 = "(?:\\d+)\\.(?:\\d+)\\.(?:\\d+)\\.(?:\\d+)"
	// RegexIPv6 is a regex to match IPv6 addresses
	RegexIPv6 = "(?:(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){6})(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:::(?:(?:(?:[0-9a-fA-F]{1,4})):){5})(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})))?::(?:(?:(?:[0-9a-fA-F]{1,4})):){4})(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){0,1}(?:(?:[0-9a-fA-F]{1,4})))?::(?:(?:(?:[0-9a-fA-F]{1,4})):){3})(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){0,2}(?:(?:[0-9a-fA-F]{1,4})))?::(?:(?:(?:[0-9a-fA-F]{1,4})):){2})(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){0,3}(?:(?:[0-9a-fA-F]{1,4})))?::(?:(?:[0-9a-fA-F]{1,4})):)(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){0,4}(?:(?:[0-9a-fA-F]{1,4})))?::)(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9]))\\.){3}(?:(?:25[0-5]|(?:[1-9]|1[0-9]|2[0-4])?[0-9])))))))|(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){0,5}(?:(?:[0-9a-fA-F]{1,4})))?::)(?:(?:[0-9a-fA-F]{1,4})))|(?:(?:(?:(?:(?:(?:[0-9a-fA-F]{1,4})):){0,6}(?:(?:[0-9a-fA-F]{1,4})))?::))))"
)

var netexecImageName = imageutils.GetE2EImage(imageutils.Netexec)

// NewNetworkingTestConfig creates and sets up a new test config helper.
func NewNetworkingTestConfig(f *Framework) *NetworkingTestConfig {
	config := &NetworkingTestConfig{f: f, Namespace: f.Namespace.Name, HostNetwork: true}
	ginkgo.By(fmt.Sprintf("Performing setup for networking test in namespace %v", config.Namespace))
	config.setup(getServiceSelector())
	return config
}

// NewCoreNetworkingTestConfig creates and sets up a new test config helper for Node E2E.
func NewCoreNetworkingTestConfig(f *Framework, hostNetwork bool) *NetworkingTestConfig {
	config := &NetworkingTestConfig{f: f, Namespace: f.Namespace.Name, HostNetwork: hostNetwork}
	ginkgo.By(fmt.Sprintf("Performing setup for networking test in namespace %v", config.Namespace))
	config.setupCore(getServiceSelector())
	return config
}

func getServiceSelector() map[string]string {
	ginkgo.By("creating a selector")
	selectorName := "selector-" + string(uuid.NewUUID())
	serviceSelector := map[string]string{
		selectorName: "true",
	}
	return serviceSelector
}

// NetworkingTestConfig is a convenience class around some utility methods
// for testing kubeproxy/networking/services/endpoints.
type NetworkingTestConfig struct {
	// TestContaienrPod is a test pod running the netexec image. It is capable
	// of executing tcp/udp requests against ip:port.
	TestContainerPod *v1.Pod
	// HostTestContainerPod is a pod running using the hostexec image.
	HostTestContainerPod *v1.Pod
	// if the HostTestContainerPod is running with HostNetwork=true.
	HostNetwork bool
	// EndpointPods are the pods belonging to the Service created by this
	// test config. Each invocation of `setup` creates a service with
	// 1 pod per node running the netexecImage.
	EndpointPods []*v1.Pod
	f            *Framework
	podClient    *PodClient
	// NodePortService is a Service with Type=NodePort spanning over all
	// endpointPods.
	NodePortService *v1.Service
	// SessionAffinityService is a Service with SessionAffinity=ClientIP
	// spanning over all endpointPods.
	SessionAffinityService *v1.Service
	// ExternalAddrs is a list of external IPs of nodes in the cluster.
	ExternalAddrs []string
	// Nodes is a list of nodes in the cluster.
	Nodes []v1.Node
	// MaxTries is the number of retries tolerated for tests run against
	// endpoints and services created by this config.
	MaxTries int
	// The ClusterIP of the Service reated by this test config.
	ClusterIP string
	// External ip of first node for use in nodePort testing.
	NodeIP string
	// The http/udp nodePorts of the Service.
	NodeHTTPPort int
	NodeUDPPort  int
	// The kubernetes namespace within which all resources for this
	// config are created
	Namespace string
}

// DialFromEndpointContainer executes a curl via kubectl exec in an endpoint container.
func (config *NetworkingTestConfig) DialFromEndpointContainer(protocol, targetIP string, targetPort, maxTries, minTries int, expectedEps sets.String) {
	config.DialFromContainer(protocol, config.EndpointPods[0].Status.PodIP, targetIP, EndpointHTTPPort, targetPort, maxTries, minTries, expectedEps)
}

// DialFromTestContainer executes a curl via kubectl exec in a test container.
func (config *NetworkingTestConfig) DialFromTestContainer(protocol, targetIP string, targetPort, maxTries, minTries int, expectedEps sets.String) {
	config.DialFromContainer(protocol, config.TestContainerPod.Status.PodIP, targetIP, testContainerHTTPPort, targetPort, maxTries, minTries, expectedEps)
}

// diagnoseMissingEndpoints prints debug information about the endpoints that
// are NOT in the given list of foundEndpoints. These are the endpoints we
// expected a response from.
func (config *NetworkingTestConfig) diagnoseMissingEndpoints(foundEndpoints sets.String) {
	for _, e := range config.EndpointPods {
		if foundEndpoints.Has(e.Name) {
			continue
		}
		e2elog.Logf("\nOutput of kubectl describe pod %v/%v:\n", e.Namespace, e.Name)
		desc, _ := RunKubectl(
			"describe", "pod", e.Name, fmt.Sprintf("--namespace=%v", e.Namespace))
		e2elog.Logf(desc)
	}
}

// EndpointHostnames returns a set of hostnames for existing endpoints.
func (config *NetworkingTestConfig) EndpointHostnames() sets.String {
	expectedEps := sets.NewString()
	for _, p := range config.EndpointPods {
		expectedEps.Insert(p.Name)
	}
	return expectedEps
}

// DialFromContainer executes a curl via kubectl exec in a test container,
// which might then translate to a tcp or udp request based on the protocol
// argument in the url.
// - minTries is the minimum number of curl attempts required before declaring
//   success. Set to 0 if you'd like to return as soon as all endpoints respond
//   at least once.
// - maxTries is the maximum number of curl attempts. If this many attempts pass
//   and we don't see all expected endpoints, the test fails.
// - expectedEps is the set of endpointnames to wait for. Typically this is also
//   the hostname reported by each pod in the service through /hostName.
// maxTries == minTries will confirm that we see the expected endpoints and no
// more for maxTries. Use this if you want to eg: fail a readiness check on a
// pod and confirm it doesn't show up as an endpoint.
func (config *NetworkingTestConfig) DialFromContainer(protocol, containerIP, targetIP string, containerHTTPPort, targetPort, maxTries, minTries int, expectedEps sets.String) {
	ipPort := net.JoinHostPort(containerIP, strconv.Itoa(containerHTTPPort))
	// The current versions of curl included in CentOS and RHEL distros
	// misinterpret square brackets around IPv6 as globbing, so use the -g
	// argument to disable globbing to handle the IPv6 case.
	cmd := fmt.Sprintf("curl -g -q -s 'http://%s/dial?request=hostName&protocol=%s&host=%s&port=%d&tries=1'",
		ipPort,
		protocol,
		targetIP,
		targetPort)

	eps := sets.NewString()

	for i := 0; i < maxTries; i++ {
		stdout, stderr, err := config.f.ExecShellInPodWithFullOutput(config.HostTestContainerPod.Name, cmd)
		if err != nil {
			// A failure to kubectl exec counts as a try, not a hard fail.
			// Also note that we will keep failing for maxTries in tests where
			// we confirm unreachability.
			e2elog.Logf("Failed to execute %q: %v, stdout: %q, stderr %q", cmd, err, stdout, stderr)
		} else {
			var output map[string][]string
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				e2elog.Logf("WARNING: Failed to unmarshal curl response. Cmd %v run in %v, output: %s, err: %v",
					cmd, config.HostTestContainerPod.Name, stdout, err)
				continue
			}

			for _, hostName := range output["responses"] {
				trimmed := strings.TrimSpace(hostName)
				if trimmed != "" {
					eps.Insert(trimmed)
				}
			}
		}
		e2elog.Logf("Waiting for endpoints: %v", expectedEps.Difference(eps))

		// Check against i+1 so we exit if minTries == maxTries.
		if (eps.Equal(expectedEps) || eps.Len() == 0 && expectedEps.Len() == 0) && i+1 >= minTries {
			return
		}
		// TODO: get rid of this delay #36281
		time.Sleep(hitEndpointRetryDelay)
	}

	config.diagnoseMissingEndpoints(eps)
	Failf("Failed to find expected endpoints:\nTries %d\nCommand %v\nretrieved %v\nexpected %v\n", maxTries, cmd, eps, expectedEps)
}

// GetEndpointsFromTestContainer executes a curl via kubectl exec in a test container.
func (config *NetworkingTestConfig) GetEndpointsFromTestContainer(protocol, targetIP string, targetPort, tries int) (sets.String, error) {
	return config.GetEndpointsFromContainer(protocol, config.TestContainerPod.Status.PodIP, targetIP, testContainerHTTPPort, targetPort, tries)
}

// GetEndpointsFromContainer executes a curl via kubectl exec in a test container,
// which might then translate to a tcp or udp request based on the protocol argument
// in the url. It returns all different endpoints from multiple retries.
// - tries is the number of curl attempts. If this many attempts pass and
//   we don't see any endpoints, the test fails.
func (config *NetworkingTestConfig) GetEndpointsFromContainer(protocol, containerIP, targetIP string, containerHTTPPort, targetPort, tries int) (sets.String, error) {
	ipPort := net.JoinHostPort(containerIP, strconv.Itoa(containerHTTPPort))
	// The current versions of curl included in CentOS and RHEL distros
	// misinterpret square brackets around IPv6 as globbing, so use the -g
	// argument to disable globbing to handle the IPv6 case.
	cmd := fmt.Sprintf("curl -g -q -s 'http://%s/dial?request=hostName&protocol=%s&host=%s&port=%d&tries=1'",
		ipPort,
		protocol,
		targetIP,
		targetPort)

	eps := sets.NewString()

	for i := 0; i < tries; i++ {
		stdout, stderr, err := config.f.ExecShellInPodWithFullOutput(config.HostTestContainerPod.Name, cmd)
		if err != nil {
			// A failure to kubectl exec counts as a try, not a hard fail.
			// Also note that we will keep failing for maxTries in tests where
			// we confirm unreachability.
			e2elog.Logf("Failed to execute %q: %v, stdout: %q, stderr: %q", cmd, err, stdout, stderr)
		} else {
			e2elog.Logf("Tries: %d, in try: %d, stdout: %v, stderr: %v, command run in: %#v", tries, i, stdout, stderr, config.HostTestContainerPod)
			var output map[string][]string
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				e2elog.Logf("WARNING: Failed to unmarshal curl response. Cmd %v run in %v, output: %s, err: %v",
					cmd, config.HostTestContainerPod.Name, stdout, err)
				continue
			}

			for _, hostName := range output["responses"] {
				trimmed := strings.TrimSpace(hostName)
				if trimmed != "" {
					eps.Insert(trimmed)
				}
			}
			// TODO: get rid of this delay #36281
			time.Sleep(hitEndpointRetryDelay)
		}
	}
	return eps, nil
}

// DialFromNode executes a tcp or udp request based on protocol via kubectl exec
// in a test container running with host networking.
// - minTries is the minimum number of curl attempts required before declaring
//   success. Set to 0 if you'd like to return as soon as all endpoints respond
//   at least once.
// - maxTries is the maximum number of curl attempts. If this many attempts pass
//   and we don't see all expected endpoints, the test fails.
// maxTries == minTries will confirm that we see the expected endpoints and no
// more for maxTries. Use this if you want to eg: fail a readiness check on a
// pod and confirm it doesn't show up as an endpoint.
func (config *NetworkingTestConfig) DialFromNode(protocol, targetIP string, targetPort, maxTries, minTries int, expectedEps sets.String) {
	var cmd string
	if protocol == "udp" {
		// TODO: It would be enough to pass 1s+epsilon to timeout, but unfortunately
		// busybox timeout doesn't support non-integer values.
		cmd = fmt.Sprintf("echo hostName | nc -w 1 -u %s %d", targetIP, targetPort)
	} else {
		ipPort := net.JoinHostPort(targetIP, strconv.Itoa(targetPort))
		// The current versions of curl included in CentOS and RHEL distros
		// misinterpret square brackets around IPv6 as globbing, so use the -g
		// argument to disable globbing to handle the IPv6 case.
		cmd = fmt.Sprintf("curl -g -q -s --max-time 15 --connect-timeout 1 http://%s/hostName", ipPort)
	}

	// TODO: This simply tells us that we can reach the endpoints. Check that
	// the probability of hitting a specific endpoint is roughly the same as
	// hitting any other.
	eps := sets.NewString()

	filterCmd := fmt.Sprintf("%s | grep -v '^\\s*$'", cmd)
	for i := 0; i < maxTries; i++ {
		stdout, stderr, err := config.f.ExecShellInPodWithFullOutput(config.HostTestContainerPod.Name, filterCmd)
		if err != nil || len(stderr) > 0 {
			// A failure to exec command counts as a try, not a hard fail.
			// Also note that we will keep failing for maxTries in tests where
			// we confirm unreachability.
			e2elog.Logf("Failed to execute %q: %v, stdout: %q, stderr: %q", filterCmd, err, stdout, stderr)
		} else {
			trimmed := strings.TrimSpace(stdout)
			if trimmed != "" {
				eps.Insert(trimmed)
			}
		}

		// Check against i+1 so we exit if minTries == maxTries.
		if eps.Equal(expectedEps) && i+1 >= minTries {
			e2elog.Logf("Found all expected endpoints: %+v", eps.List())
			return
		}

		e2elog.Logf("Waiting for %+v endpoints (expected=%+v, actual=%+v)", expectedEps.Difference(eps).List(), expectedEps.List(), eps.List())

		// TODO: get rid of this delay #36281
		time.Sleep(hitEndpointRetryDelay)
	}

	config.diagnoseMissingEndpoints(eps)
	Failf("Failed to find expected endpoints:\nTries %d\nCommand %v\nretrieved %v\nexpected %v\n", maxTries, cmd, eps, expectedEps)
}

// GetSelfURL executes a curl against the given path via kubectl exec into a
// test container running with host networking, and fails if the output
// doesn't match the expected string.
func (config *NetworkingTestConfig) GetSelfURL(port int32, path string, expected string) {
	cmd := fmt.Sprintf("curl -i -q -s --connect-timeout 1 http://localhost:%d%s", port, path)
	ginkgo.By(fmt.Sprintf("Getting kube-proxy self URL %s", path))
	config.executeCurlCmd(cmd, expected)
}

// GetSelfURLStatusCode executes a curl against the given path via kubectl exec into a
// test container running with host networking, and fails if the returned status
// code doesn't match the expected string.
func (config *NetworkingTestConfig) GetSelfURLStatusCode(port int32, path string, expected string) {
	// check status code
	cmd := fmt.Sprintf("curl -o /dev/null -i -q -s -w %%{http_code} --connect-timeout 1 http://localhost:%d%s", port, path)
	ginkgo.By(fmt.Sprintf("Checking status code against http://localhost:%d%s", port, path))
	config.executeCurlCmd(cmd, expected)
}

func (config *NetworkingTestConfig) executeCurlCmd(cmd string, expected string) {
	// These are arbitrary timeouts. The curl command should pass on first try,
	// unless remote server is starved/bootstrapping/restarting etc.
	const retryInterval = 1 * time.Second
	const retryTimeout = 30 * time.Second
	podName := config.HostTestContainerPod.Name
	var msg string
	if pollErr := wait.PollImmediate(retryInterval, retryTimeout, func() (bool, error) {
		stdout, err := RunHostCmd(config.Namespace, podName, cmd)
		if err != nil {
			msg = fmt.Sprintf("failed executing cmd %v in %v/%v: %v", cmd, config.Namespace, podName, err)
			e2elog.Logf(msg)
			return false, nil
		}
		if !strings.Contains(stdout, expected) {
			msg = fmt.Sprintf("successfully executed %v in %v/%v, but output '%v' doesn't contain expected string '%v'", cmd, config.Namespace, podName, stdout, expected)
			e2elog.Logf(msg)
			return false, nil
		}
		return true, nil
	}); pollErr != nil {
		e2elog.Logf("\nOutput of kubectl describe pod %v/%v:\n", config.Namespace, podName)
		desc, _ := RunKubectl(
			"describe", "pod", podName, fmt.Sprintf("--namespace=%v", config.Namespace))
		e2elog.Logf("%s", desc)
		Failf("Timed out in %v: %v", retryTimeout, msg)
	}
}

func (config *NetworkingTestConfig) createNetShellPodSpec(podName, hostname string) *v1.Pod {
	probe := &v1.Probe{
		InitialDelaySeconds: 10,
		TimeoutSeconds:      30,
		PeriodSeconds:       10,
		SuccessThreshold:    1,
		FailureThreshold:    3,
		Handler: v1.Handler{
			HTTPGet: &v1.HTTPGetAction{
				Path: "/healthz",
				Port: intstr.IntOrString{IntVal: EndpointHTTPPort},
			},
		},
	}
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: config.Namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "webserver",
					Image:           netexecImageName,
					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"/netexec",
						fmt.Sprintf("--http-port=%d", EndpointHTTPPort),
						fmt.Sprintf("--udp-port=%d", EndpointUDPPort),
					},
					Ports: []v1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: EndpointHTTPPort,
						},
						{
							Name:          "udp",
							ContainerPort: EndpointUDPPort,
							Protocol:      v1.ProtocolUDP,
						},
					},
					LivenessProbe:  probe,
					ReadinessProbe: probe,
				},
			},
			NodeSelector: map[string]string{
				"kubernetes.io/hostname": hostname,
			},
		},
	}
	return pod
}

func (config *NetworkingTestConfig) createTestPodSpec() *v1.Pod {
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      testPodName,
			Namespace: config.Namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:            "webserver",
					Image:           netexecImageName,
					ImagePullPolicy: v1.PullIfNotPresent,
					Command: []string{
						"/netexec",
						fmt.Sprintf("--http-port=%d", EndpointHTTPPort),
						fmt.Sprintf("--udp-port=%d", EndpointUDPPort),
					},
					Ports: []v1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: testContainerHTTPPort,
						},
					},
				},
			},
		},
	}
	return pod
}

func (config *NetworkingTestConfig) createNodePortServiceSpec(svcName string, selector map[string]string, enableSessionAffinity bool) *v1.Service {
	sessionAffinity := v1.ServiceAffinityNone
	if enableSessionAffinity {
		sessionAffinity = v1.ServiceAffinityClientIP
	}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: svcName,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Ports: []v1.ServicePort{
				{Port: ClusterHTTPPort, Name: "http", Protocol: v1.ProtocolTCP, TargetPort: intstr.FromInt(EndpointHTTPPort)},
				{Port: ClusterUDPPort, Name: "udp", Protocol: v1.ProtocolUDP, TargetPort: intstr.FromInt(EndpointUDPPort)},
			},
			Selector:        selector,
			SessionAffinity: sessionAffinity,
		},
	}
}

func (config *NetworkingTestConfig) createNodePortService(selector map[string]string) {
	config.NodePortService = config.createService(config.createNodePortServiceSpec(nodePortServiceName, selector, false))
}

func (config *NetworkingTestConfig) createSessionAffinityService(selector map[string]string) {
	config.SessionAffinityService = config.createService(config.createNodePortServiceSpec(sessionAffinityServiceName, selector, true))
}

// DeleteNodePortService deletes NodePort service.
func (config *NetworkingTestConfig) DeleteNodePortService() {
	err := config.getServiceClient().Delete(config.NodePortService.Name, nil)
	ExpectNoError(err, "error while deleting NodePortService. err:%v)", err)
	time.Sleep(15 * time.Second) // wait for kube-proxy to catch up with the service being deleted.
}

func (config *NetworkingTestConfig) createTestPods() {
	testContainerPod := config.createTestPodSpec()
	hostTestContainerPod := NewExecPodSpec(config.Namespace, hostTestPodName, config.HostNetwork)

	config.createPod(testContainerPod)
	config.createPod(hostTestContainerPod)

	ExpectNoError(config.f.WaitForPodRunning(testContainerPod.Name))
	ExpectNoError(config.f.WaitForPodRunning(hostTestContainerPod.Name))

	var err error
	config.TestContainerPod, err = config.getPodClient().Get(testContainerPod.Name, metav1.GetOptions{})
	if err != nil {
		Failf("Failed to retrieve %s pod: %v", testContainerPod.Name, err)
	}

	config.HostTestContainerPod, err = config.getPodClient().Get(hostTestContainerPod.Name, metav1.GetOptions{})
	if err != nil {
		Failf("Failed to retrieve %s pod: %v", hostTestContainerPod.Name, err)
	}
}

func (config *NetworkingTestConfig) createService(serviceSpec *v1.Service) *v1.Service {
	_, err := config.getServiceClient().Create(serviceSpec)
	ExpectNoError(err, fmt.Sprintf("Failed to create %s service: %v", serviceSpec.Name, err))

	err = WaitForService(config.f.ClientSet, config.Namespace, serviceSpec.Name, true, 5*time.Second, 45*time.Second)
	ExpectNoError(err, fmt.Sprintf("error while waiting for service:%s err: %v", serviceSpec.Name, err))

	createdService, err := config.getServiceClient().Get(serviceSpec.Name, metav1.GetOptions{})
	ExpectNoError(err, fmt.Sprintf("Failed to create %s service: %v", serviceSpec.Name, err))

	return createdService
}

// setupCore sets up the pods and core test config
// mainly for simplified node e2e setup
func (config *NetworkingTestConfig) setupCore(selector map[string]string) {
	ginkgo.By("Creating the service pods in kubernetes")
	podName := "netserver"
	config.EndpointPods = config.createNetProxyPods(podName, selector)

	ginkgo.By("Creating test pods")
	config.createTestPods()

	epCount := len(config.EndpointPods)
	config.MaxTries = epCount*epCount + testTries
}

// setup includes setupCore and also sets up services
func (config *NetworkingTestConfig) setup(selector map[string]string) {
	config.setupCore(selector)

	ginkgo.By("Getting node addresses")
	ExpectNoError(WaitForAllNodesSchedulable(config.f.ClientSet, 10*time.Minute))
	nodeList := GetReadySchedulableNodesOrDie(config.f.ClientSet)
	config.ExternalAddrs = NodeAddresses(nodeList, v1.NodeExternalIP)

	SkipUnlessNodeCountIsAtLeast(2)
	config.Nodes = nodeList.Items

	ginkgo.By("Creating the service on top of the pods in kubernetes")
	config.createNodePortService(selector)
	config.createSessionAffinityService(selector)

	for _, p := range config.NodePortService.Spec.Ports {
		switch p.Protocol {
		case v1.ProtocolUDP:
			config.NodeUDPPort = int(p.NodePort)
		case v1.ProtocolTCP:
			config.NodeHTTPPort = int(p.NodePort)
		default:
			continue
		}
	}
	config.ClusterIP = config.NodePortService.Spec.ClusterIP
	if len(config.ExternalAddrs) != 0 {
		config.NodeIP = config.ExternalAddrs[0]
	} else {
		internalAddrs := NodeAddresses(nodeList, v1.NodeInternalIP)
		config.NodeIP = internalAddrs[0]
	}
}

func (config *NetworkingTestConfig) cleanup() {
	nsClient := config.getNamespacesClient()
	nsList, err := nsClient.List(metav1.ListOptions{})
	if err == nil {
		for _, ns := range nsList.Items {
			if strings.Contains(ns.Name, config.f.BaseName) && ns.Name != config.Namespace {
				nsClient.Delete(ns.Name, nil)
			}
		}
	}
}

// shuffleNodes copies nodes from the specified slice into a copy in random
// order. It returns a new slice.
func shuffleNodes(nodes []v1.Node) []v1.Node {
	shuffled := make([]v1.Node, len(nodes))
	perm := rand.Perm(len(nodes))
	for i, j := range perm {
		shuffled[j] = nodes[i]
	}
	return shuffled
}

func (config *NetworkingTestConfig) createNetProxyPods(podName string, selector map[string]string) []*v1.Pod {
	ExpectNoError(WaitForAllNodesSchedulable(config.f.ClientSet, 10*time.Minute))
	nodeList := GetReadySchedulableNodesOrDie(config.f.ClientSet)

	// To make this test work reasonably fast in large clusters,
	// we limit the number of NetProxyPods to no more than
	// maxNetProxyPodsCount on random nodes.
	nodes := shuffleNodes(nodeList.Items)
	if len(nodes) > maxNetProxyPodsCount {
		nodes = nodes[:maxNetProxyPodsCount]
	}

	// create pods, one for each node
	createdPods := make([]*v1.Pod, 0, len(nodes))
	for i, n := range nodes {
		podName := fmt.Sprintf("%s-%d", podName, i)
		hostname, _ := n.Labels["kubernetes.io/hostname"]
		pod := config.createNetShellPodSpec(podName, hostname)
		pod.ObjectMeta.Labels = selector
		createdPod := config.createPod(pod)
		createdPods = append(createdPods, createdPod)
	}

	// wait that all of them are up
	runningPods := make([]*v1.Pod, 0, len(nodes))
	for _, p := range createdPods {
		ExpectNoError(config.f.WaitForPodReady(p.Name))
		rp, err := config.getPodClient().Get(p.Name, metav1.GetOptions{})
		ExpectNoError(err)
		runningPods = append(runningPods, rp)
	}

	return runningPods
}

// DeleteNetProxyPod deletes the first endpoint pod and waits for it being removed.
func (config *NetworkingTestConfig) DeleteNetProxyPod() {
	pod := config.EndpointPods[0]
	config.getPodClient().Delete(pod.Name, metav1.NewDeleteOptions(0))
	config.EndpointPods = config.EndpointPods[1:]
	// wait for pod being deleted.
	err := WaitForPodToDisappear(config.f.ClientSet, config.Namespace, pod.Name, labels.Everything(), time.Second, wait.ForeverTestTimeout)
	if err != nil {
		Failf("Failed to delete %s pod: %v", pod.Name, err)
	}
	// wait for endpoint being removed.
	err = WaitForServiceEndpointsNum(config.f.ClientSet, config.Namespace, nodePortServiceName, len(config.EndpointPods), time.Second, wait.ForeverTestTimeout)
	if err != nil {
		Failf("Failed to remove endpoint from service: %s", nodePortServiceName)
	}
	// wait for kube-proxy to catch up with the pod being deleted.
	time.Sleep(5 * time.Second)
}

func (config *NetworkingTestConfig) createPod(pod *v1.Pod) *v1.Pod {
	return config.getPodClient().Create(pod)
}

func (config *NetworkingTestConfig) getPodClient() *PodClient {
	if config.podClient == nil {
		config.podClient = config.f.PodClient()
	}
	return config.podClient
}

func (config *NetworkingTestConfig) getServiceClient() coreclientset.ServiceInterface {
	return config.f.ClientSet.CoreV1().Services(config.Namespace)
}

func (config *NetworkingTestConfig) getNamespacesClient() coreclientset.NamespaceInterface {
	return config.f.ClientSet.CoreV1().Namespaces()
}

// CheckReachabilityFromPod checks reachability from the specified pod.
func CheckReachabilityFromPod(expectToBeReachable bool, timeout time.Duration, namespace, pod, target string) {
	cmd := fmt.Sprintf("wget -T 5 -qO- %q", target)
	err := wait.PollImmediate(Poll, timeout, func() (bool, error) {
		_, err := RunHostCmd(namespace, pod, cmd)
		if expectToBeReachable && err != nil {
			e2elog.Logf("Expect target to be reachable. But got err: %v. Retry until timeout", err)
			return false, nil
		}

		if !expectToBeReachable && err == nil {
			e2elog.Logf("Expect target NOT to be reachable. But it is reachable. Retry until timeout")
			return false, nil
		}
		return true, nil
	})
	ExpectNoError(err)
}

// HTTPPokeParams is a struct for HTTP poke parameters.
type HTTPPokeParams struct {
	Timeout        time.Duration
	ExpectCode     int // default = 200
	BodyContains   string
	RetriableCodes []int
}

// HTTPPokeResult is a struct for HTTP poke result.
type HTTPPokeResult struct {
	Status HTTPPokeStatus
	Code   int    // HTTP code: 0 if the connection was not made
	Error  error  // if there was any error
	Body   []byte // if code != 0
}

// HTTPPokeStatus is string for representing HTTP poke status.
type HTTPPokeStatus string

const (
	// HTTPSuccess is HTTP poke status which is success.
	HTTPSuccess HTTPPokeStatus = "Success"
	// HTTPError is HTTP poke status which is error.
	HTTPError HTTPPokeStatus = "UnknownError"
	// HTTPTimeout is HTTP poke status which is timeout.
	HTTPTimeout HTTPPokeStatus = "TimedOut"
	// HTTPRefused is HTTP poke status which is connection refused.
	HTTPRefused HTTPPokeStatus = "ConnectionRefused"
	// HTTPRetryCode is HTTP poke status which is retry code.
	HTTPRetryCode HTTPPokeStatus = "RetryCode"
	// HTTPWrongCode is HTTP poke status which is wrong code.
	HTTPWrongCode HTTPPokeStatus = "WrongCode"
	// HTTPBadResponse is HTTP poke status which is bad response.
	HTTPBadResponse HTTPPokeStatus = "BadResponse"
	// Any time we add new errors, we should audit all callers of this.
)

// PokeHTTP tries to connect to a host on a port for a given URL path.  Callers
// can specify additional success parameters, if desired.
//
// The result status will be characterized as precisely as possible, given the
// known users of this.
//
// The result code will be zero in case of any failure to connect, or non-zero
// if the HTTP transaction completed (even if the other test params make this a
// failure).
//
// The result error will be populated for any status other than Success.
//
// The result body will be populated if the HTTP transaction was completed, even
// if the other test params make this a failure).
func PokeHTTP(host string, port int, path string, params *HTTPPokeParams) HTTPPokeResult {
	hostPort := net.JoinHostPort(host, strconv.Itoa(port))
	url := fmt.Sprintf("http://%s%s", hostPort, path)

	ret := HTTPPokeResult{}

	// Sanity check inputs, because it has happened.  These are the only things
	// that should hard fail the test - they are basically ASSERT()s.
	if host == "" {
		Failf("Got empty host for HTTP poke (%s)", url)
		return ret
	}
	if port == 0 {
		Failf("Got port==0 for HTTP poke (%s)", url)
		return ret
	}

	// Set default params.
	if params == nil {
		params = &HTTPPokeParams{}
	}
	if params.ExpectCode == 0 {
		params.ExpectCode = http.StatusOK
	}

	e2elog.Logf("Poking %q", url)

	resp, err := httpGetNoConnectionPoolTimeout(url, params.Timeout)
	if err != nil {
		ret.Error = err
		neterr, ok := err.(net.Error)
		if ok && neterr.Timeout() {
			ret.Status = HTTPTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			ret.Status = HTTPRefused
		} else {
			ret.Status = HTTPError
		}
		e2elog.Logf("Poke(%q): %v", url, err)
		return ret
	}

	ret.Code = resp.StatusCode

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ret.Status = HTTPError
		ret.Error = fmt.Errorf("error reading HTTP body: %v", err)
		e2elog.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}
	ret.Body = make([]byte, len(body))
	copy(ret.Body, body)

	if resp.StatusCode != params.ExpectCode {
		for _, code := range params.RetriableCodes {
			if resp.StatusCode == code {
				ret.Error = fmt.Errorf("retriable status code: %d", resp.StatusCode)
				ret.Status = HTTPRetryCode
				e2elog.Logf("Poke(%q): %v", url, ret.Error)
				return ret
			}
		}
		ret.Status = HTTPWrongCode
		ret.Error = fmt.Errorf("bad status code: %d", resp.StatusCode)
		e2elog.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}

	if params.BodyContains != "" && !strings.Contains(string(body), params.BodyContains) {
		ret.Status = HTTPBadResponse
		ret.Error = fmt.Errorf("response does not contain expected substring: %q", string(body))
		e2elog.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}

	ret.Status = HTTPSuccess
	e2elog.Logf("Poke(%q): success", url)
	return ret
}

// Does an HTTP GET, but does not reuse TCP connections
// This masks problems where the iptables rule has changed, but we don't see it
func httpGetNoConnectionPoolTimeout(url string, timeout time.Duration) (*http.Response, error) {
	tr := utilnet.SetTransportDefaults(&http.Transport{
		DisableKeepAlives: true,
	})
	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	return client.Get(url)
}

// UDPPokeParams is a struct for UDP poke parameters.
type UDPPokeParams struct {
	Timeout  time.Duration
	Response string
}

// UDPPokeResult is a struct for UDP poke result.
type UDPPokeResult struct {
	Status   UDPPokeStatus
	Error    error  // if there was any error
	Response []byte // if code != 0
}

// UDPPokeStatus is string for representing UDP poke status.
type UDPPokeStatus string

const (
	// UDPSuccess is UDP poke status which is success.
	UDPSuccess UDPPokeStatus = "Success"
	// UDPError is UDP poke status which is error.
	UDPError UDPPokeStatus = "UnknownError"
	// UDPTimeout is UDP poke status which is timeout.
	UDPTimeout UDPPokeStatus = "TimedOut"
	// UDPRefused is UDP poke status which is connection refused.
	UDPRefused UDPPokeStatus = "ConnectionRefused"
	// UDPBadResponse is UDP poke status which is bad response.
	UDPBadResponse UDPPokeStatus = "BadResponse"
	// Any time we add new errors, we should audit all callers of this.
)

// PokeUDP tries to connect to a host on a port and send the given request. Callers
// can specify additional success parameters, if desired.
//
// The result status will be characterized as precisely as possible, given the
// known users of this.
//
// The result error will be populated for any status other than Success.
//
// The result response will be populated if the UDP transaction was completed, even
// if the other test params make this a failure).
func PokeUDP(host string, port int, request string, params *UDPPokeParams) UDPPokeResult {
	hostPort := net.JoinHostPort(host, strconv.Itoa(port))
	url := fmt.Sprintf("udp://%s", hostPort)

	ret := UDPPokeResult{}

	// Sanity check inputs, because it has happened.  These are the only things
	// that should hard fail the test - they are basically ASSERT()s.
	if host == "" {
		Failf("Got empty host for UDP poke (%s)", url)
		return ret
	}
	if port == 0 {
		Failf("Got port==0 for UDP poke (%s)", url)
		return ret
	}

	// Set default params.
	if params == nil {
		params = &UDPPokeParams{}
	}

	e2elog.Logf("Poking %v", url)

	con, err := net.Dial("udp", hostPort)
	if err != nil {
		ret.Status = UDPError
		ret.Error = err
		e2elog.Logf("Poke(%q): %v", url, err)
		return ret
	}

	_, err = con.Write([]byte(fmt.Sprintf("%s\n", request)))
	if err != nil {
		ret.Error = err
		neterr, ok := err.(net.Error)
		if ok && neterr.Timeout() {
			ret.Status = UDPTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			ret.Status = UDPRefused
		} else {
			ret.Status = UDPError
		}
		e2elog.Logf("Poke(%q): %v", url, err)
		return ret
	}

	if params.Timeout != 0 {
		err = con.SetDeadline(time.Now().Add(params.Timeout))
		if err != nil {
			ret.Status = UDPError
			ret.Error = err
			e2elog.Logf("Poke(%q): %v", url, err)
			return ret
		}
	}

	bufsize := len(params.Response) + 1
	if bufsize == 0 {
		bufsize = 4096
	}
	var buf = make([]byte, bufsize)
	n, err := con.Read(buf)
	if err != nil {
		ret.Error = err
		neterr, ok := err.(net.Error)
		if ok && neterr.Timeout() {
			ret.Status = UDPTimeout
		} else if strings.Contains(err.Error(), "connection refused") {
			ret.Status = UDPRefused
		} else {
			ret.Status = UDPError
		}
		e2elog.Logf("Poke(%q): %v", url, err)
		return ret
	}
	ret.Response = buf[0:n]

	if params.Response != "" && string(ret.Response) != params.Response {
		ret.Status = UDPBadResponse
		ret.Error = fmt.Errorf("response does not match expected string: %q", string(ret.Response))
		e2elog.Logf("Poke(%q): %v", url, ret.Error)
		return ret
	}

	ret.Status = UDPSuccess
	e2elog.Logf("Poke(%q): success", url)
	return ret
}

// TestHitNodesFromOutside checkes HTTP connectivity from outside.
func TestHitNodesFromOutside(externalIP string, httpPort int32, timeout time.Duration, expectedHosts sets.String) error {
	return TestHitNodesFromOutsideWithCount(externalIP, httpPort, timeout, expectedHosts, 1)
}

// TestHitNodesFromOutsideWithCount checkes HTTP connectivity from outside with count.
func TestHitNodesFromOutsideWithCount(externalIP string, httpPort int32, timeout time.Duration, expectedHosts sets.String,
	countToSucceed int) error {
	e2elog.Logf("Waiting up to %v for satisfying expectedHosts for %v times", timeout, countToSucceed)
	hittedHosts := sets.NewString()
	count := 0
	condition := func() (bool, error) {
		result := PokeHTTP(externalIP, int(httpPort), "/hostname", &HTTPPokeParams{Timeout: 1 * time.Second})
		if result.Status != HTTPSuccess {
			return false, nil
		}

		hittedHost := strings.TrimSpace(string(result.Body))
		if !expectedHosts.Has(hittedHost) {
			e2elog.Logf("Error hitting unexpected host: %v, reset counter: %v", hittedHost, count)
			count = 0
			return false, nil
		}
		if !hittedHosts.Has(hittedHost) {
			hittedHosts.Insert(hittedHost)
			e2elog.Logf("Missing %+v, got %+v", expectedHosts.Difference(hittedHosts), hittedHosts)
		}
		if hittedHosts.Equal(expectedHosts) {
			count++
			if count >= countToSucceed {
				return true, nil
			}
		}
		return false, nil
	}

	if err := wait.Poll(time.Second, timeout, condition); err != nil {
		return fmt.Errorf("error waiting for expectedHosts: %v, hittedHosts: %v, count: %v, expected count: %v",
			expectedHosts, hittedHosts, count, countToSucceed)
	}
	return nil
}

// TestUnderTemporaryNetworkFailure blocks outgoing network traffic on 'node'. Then runs testFunc and returns its status.
// At the end (even in case of errors), the network traffic is brought back to normal.
// This function executes commands on a node so it will work only for some
// environments.
func TestUnderTemporaryNetworkFailure(c clientset.Interface, ns string, node *v1.Node, testFunc func()) {
	host, err := GetNodeExternalIP(node)
	if err != nil {
		Failf("Error getting node external ip : %v", err)
	}
	masterAddresses := GetAllMasterAddresses(c)
	ginkgo.By(fmt.Sprintf("block network traffic from node %s to the master", node.Name))
	defer func() {
		// This code will execute even if setting the iptables rule failed.
		// It is on purpose because we may have an error even if the new rule
		// had been inserted. (yes, we could look at the error code and ssh error
		// separately, but I prefer to stay on the safe side).
		ginkgo.By(fmt.Sprintf("Unblock network traffic from node %s to the master", node.Name))
		for _, masterAddress := range masterAddresses {
			UnblockNetwork(host, masterAddress)
		}
	}()

	e2elog.Logf("Waiting %v to ensure node %s is ready before beginning test...", resizeNodeReadyTimeout, node.Name)
	if !WaitForNodeToBe(c, node.Name, v1.NodeReady, true, resizeNodeReadyTimeout) {
		Failf("Node %s did not become ready within %v", node.Name, resizeNodeReadyTimeout)
	}
	for _, masterAddress := range masterAddresses {
		BlockNetwork(host, masterAddress)
	}

	e2elog.Logf("Waiting %v for node %s to be not ready after simulated network failure", resizeNodeNotReadyTimeout, node.Name)
	if !WaitForNodeToBe(c, node.Name, v1.NodeReady, false, resizeNodeNotReadyTimeout) {
		Failf("Node %s did not become not-ready within %v", node.Name, resizeNodeNotReadyTimeout)
	}

	testFunc()
	// network traffic is unblocked in a deferred function
}
