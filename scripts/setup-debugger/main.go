package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

type Port struct {
	LocalPort int
	PodPort   int
}

type PortForwardAPodRequest struct {
	// RestConfig is the kubernetes config
	RestConfig *rest.Config
	// Pod is the selected pod for this port forwarding
	Pod v1.Pod

	Ports []Port
	// Steams configures where to write or read input from
	Streams genericclioptions.IOStreams
	// StopCh is the channel used to manage the port forward lifecycle
	StopCh <-chan struct{}
	// ReadyCh communicates when the tunnel is ready to receive traffic
	ReadyCh chan struct{}
}

// debugEnvVarNames is the set of environment variables that this tool injects
// in patch mode and removes in cleanup mode. Anything outside this set is
// preserved untouched on the workload.
var debugEnvVarNames = map[string]struct{}{
	"JAVA_TOOL_OPTIONS":                           {},
	"MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE":   {},
	"MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES": {},
}

type Component struct {
	PodDisplayName      string
	DeploymentName      string
	StatefulSetName     string
	ManagementPort      int
	ContextPath         string
	SkipJavaToolOptions bool
	LocalDebugPort      int
}

var components = []Component{
	{PodDisplayName: "Identity", DeploymentName: "integration-identity", ManagementPort: 8082, ContextPath: "/identity", LocalDebugPort: 5009},
	{PodDisplayName: "Optimize", DeploymentName: "integration-optimize", ManagementPort: 8092, ContextPath: "/optimize", LocalDebugPort: 5008},
	{PodDisplayName: "Connectors", DeploymentName: "integration-connectors", ManagementPort: 8080, ContextPath: "/connectors", LocalDebugPort: 5007},
	{PodDisplayName: "Zeebe", StatefulSetName: "integration-zeebe", ManagementPort: 9600, ContextPath: "/orchestration", LocalDebugPort: 5006},
}

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
	cleanup := flag.Bool("cleanup", false, "remove debug env vars from each workload and force a pod restart")
	deleteConfigprops := flag.Bool("delete-configprops", false, "(cleanup mode) also delete local configprops-*.json files in the working directory")
	flag.Parse()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalf("Failed to build kubeconfig: %v", err)
	}

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules,
		configOverrides)

	namespace, _, err := kubeConfig.Namespace()

	g, _ := errgroup.WithContext(context.Background())

	for _, c := range components {
		c := c
		if *cleanup {
			g.Go(func() error {
				return CleanupMain(namespace, config, c)
			})
		} else {
			g.Go(func() error {
				return FetchConfigPropsMain(FetchConfigPropsRequest{
					Namespace:           namespace,
					Config:              config,
					PodDisplayName:      c.PodDisplayName,
					DeploymentName:      c.DeploymentName,
					StatefulSetName:     c.StatefulSetName,
					ManagementPort:      c.ManagementPort,
					ContextPath:         c.ContextPath,
					SkipJavaToolOptions: c.SkipJavaToolOptions,
					LocalDebugPort:      c.LocalDebugPort,
				})
			})
		}
	}

	if err := g.Wait(); err != nil {
		log.Fatalf("Error: %v", err)
	}

	if *cleanup && *deleteConfigprops {
		if err := deleteConfigPropsFiles(); err != nil {
			log.Printf("warning: failed to delete configprops files: %v", err)
		}
	}
}

type FetchConfigPropsRequest struct {
	Namespace           string
	Config              *rest.Config
	PodDisplayName      string
	DeploymentName      string
	StatefulSetName     string
	ManagementPort      int
	ContextPath         string
	SkipJavaToolOptions bool
	LocalDebugPort      int
}

func FetchConfigPropsMain(req FetchConfigPropsRequest) error {
	var wg sync.WaitGroup
	wg.Add(1)
	if req.DeploymentName != "" {
		if err := PatchDeployment(req.Namespace, req.Config, req.DeploymentName, req.SkipJavaToolOptions); err != nil {
			return fmt.Errorf("[%s] patching deployment: %w", req.PodDisplayName, err)
		}
	}
	if req.StatefulSetName != "" {
		if err := PatchStatefulSet(req.Namespace, req.Config, req.StatefulSetName, req.SkipJavaToolOptions); err != nil {
			return fmt.Errorf("[%s] patching statefulset: %w", req.PodDisplayName, err)
		}
	}

	// stopCh control the port forwarding lifecycle. When it gets closed the
	// port forward will terminate
	stopCh := make(chan struct{}, 1)
	// readyCh communicate when the port forward is ready to get traffic
	readyCh := make(chan struct{})
	// stream is used to tell the port forwarder where to place its output or
	// where to expect input if needed. For the port forwarding we just need
	// the output eventually
	stream := genericclioptions.IOStreams{
		In:     os.Stdin,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	// managing termination signal from the terminal. As you can see the stopCh
	// gets closed to gracefully handle its termination.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		fmt.Println("Bye...")
		close(stopCh)
		wg.Done()
	}()

	podName, err := GetPodName(req.Namespace, req.Config, req.DeploymentName, req.StatefulSetName)
	if err != nil {
		return fmt.Errorf("[%s] getting pod name: %w", req.PodDisplayName, err)
	}
	fmt.Printf(req.PodDisplayName+" pod name: %s\n", podName)
	revision, err := GetRevision(req.Namespace, req.Config, podName)
	if err != nil {
		// Image-revision lookup is informational; don't abort the debug setup.
		log.Printf("[%s] revision lookup skipped: %v", req.PodDisplayName, err)
	} else {
		fmt.Printf(req.PodDisplayName+" pod revision: %s\n", revision.Labels.Revision)
		fmt.Printf(req.PodDisplayName+" pod source: %s\n", revision.Labels.Source)
	}

	var portFwdErr error
	go func() {
		// PortForward the pod specified from its port 9090 to the local port
		// 8080
		portFwdErr = PortForwardAPod(PortForwardAPodRequest{
			RestConfig: req.Config,
			Pod: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      podName,
					Namespace: req.Namespace,
				},
			},
			Ports: []Port{
				{
					LocalPort: req.LocalDebugPort,
					PodPort:   5005,
				},
				{
					LocalPort: req.ManagementPort,
					PodPort:   req.ManagementPort,
				},
			},
			Streams: stream,
			StopCh:  stopCh,
			ReadyCh: readyCh,
		})
		if portFwdErr != nil {
			close(stopCh)
			wg.Done()
		}
	}()

	select {
	case <-readyCh:
		break
	case <-stopCh:
		return fmt.Errorf("[%s] port forwarding failed: %w", req.PodDisplayName, portFwdErr)
	}

	// Fetch configprops in the background but keep the port-forward alive until
	// SIGINT — that way jdb can attach to LocalDebugPort after this script
	// reports "ready". The signal handler above is the sole path that closes
	// stopCh and releases wg.
	go func() {
		if err := WaitUntilPodIsReady(req.Namespace, req.Config, podName); err != nil {
			log.Printf("[%s] waiting for pod ready: %v", req.PodDisplayName, err)
			return
		}
		if err := FetchConfigProps(podName, req.ManagementPort, req.ContextPath); err != nil {
			log.Printf("[%s] fetching config props: %v", req.PodDisplayName, err)
			return
		}
		fmt.Printf("[%s] ready — JDWP on localhost:%d, mgmt on localhost:%d\n", req.PodDisplayName, req.LocalDebugPort, req.ManagementPort)
	}()

	wg.Wait()
	return portFwdErr
}

func PortForwardAPod(req PortForwardAPodRequest) error {
	path := fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward",
		req.Pod.Namespace, req.Pod.Name)
	hostIP := strings.TrimLeft(req.RestConfig.Host, "htps:/")

	transport, upgrader, err := spdy.RoundTripperFor(req.RestConfig)
	if err != nil {
		return err
	}

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, &url.URL{Scheme: "https", Path: path, Host: hostIP})

	ports := []string{}
	for _, port := range req.Ports {
		ports = append(ports, fmt.Sprintf("%d:%d", port.LocalPort, port.PodPort))
	}
	fw, err := portforward.New(dialer, ports, req.StopCh, req.ReadyCh, req.Streams.Out, req.Streams.ErrOut)
	if err != nil {
		return err
	}
	return fw.ForwardPorts()
}

func PatchDeployment(namespace string, config *rest.Config, podName string, skipJavaTool bool) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating clientset: %w", err)
	}
	deploymentsClient := clientset.AppsV1().Deployments(namespace)
	result, getErr := deploymentsClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if getErr != nil {
		return fmt.Errorf("failed to get latest version of Deployment: %w", getErr)
	}
	found := false
	for _, env := range result.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE" {
			found = true
			break
		}
	}
	if !found {
		if !skipJavaTool {
			result.Spec.Template.Spec.Containers[0].Env = append(result.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
				Name:  "JAVA_TOOL_OPTIONS",
				Value: "-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005",
			})
		}
		result.Spec.Template.Spec.Containers[0].Env = append(result.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE",
			Value: "health,info,metrics,prometheus,configprops",
		})
		result.Spec.Template.Spec.Containers[0].Env = append(result.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES",
			Value: "ALWAYS",
		})
	}
	zero := int32(0)
	result.Spec.Replicas = &zero
	_, updateErr := deploymentsClient.Update(context.TODO(), result, metav1.UpdateOptions{})
	if updateErr != nil {
		return fmt.Errorf("failed to scale down deployment: %w", updateErr)
	}

	time.Sleep(3 * time.Second)

	result, getErr = deploymentsClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if getErr != nil {
		return fmt.Errorf("failed to get latest version of Deployment: %w", getErr)
	}
	one := int32(1)
	result.Spec.Replicas = &one
	_, updateErr = deploymentsClient.Update(context.TODO(), result, metav1.UpdateOptions{})
	if updateErr != nil {
		return fmt.Errorf("failed to scale up deployment: %w", updateErr)
	}
	time.Sleep(15 * time.Second)

	return nil
}

func PatchStatefulSet(namespace string, config *rest.Config, podName string, skipJavaTool bool) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating clientset: %w", err)
	}
	statefulsetClient := clientset.AppsV1().StatefulSets(namespace)
	result, getErr := statefulsetClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if getErr != nil {
		return fmt.Errorf("failed to get latest version of StatefulSet: %w", getErr)
	}
	found := false
	for _, env := range result.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE" {
			found = true
			break
		}
	}
	if !found {
		if !skipJavaTool {
			result.Spec.Template.Spec.Containers[0].Env = append(result.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
				Name:  "JAVA_TOOL_OPTIONS",
				Value: "-agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005",
			})
		}
		result.Spec.Template.Spec.Containers[0].Env = append(result.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "MANAGEMENT_ENDPOINTS_WEB_EXPOSURE_INCLUDE",
			Value: "health,info,metrics,prometheus,configprops",
		})
		result.Spec.Template.Spec.Containers[0].Env = append(result.Spec.Template.Spec.Containers[0].Env, v1.EnvVar{
			Name:  "MANAGEMENT_ENDPOINT_CONFIGPROPS_SHOW_VALUES",
			Value: "ALWAYS",
		})
	}
	oldReplicaCount := result.Spec.Replicas
	zero := int32(0)
	result.Spec.Replicas = &zero
	_, updateErr := statefulsetClient.Update(context.TODO(), result, metav1.UpdateOptions{})
	if updateErr != nil {
		return fmt.Errorf("failed to scale down statefulset: %w", updateErr)
	}

	time.Sleep(3 * time.Second)

	result, getErr = statefulsetClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if getErr != nil {
		return fmt.Errorf("failed to get latest version of StatefulSet: %w", getErr)
	}
	result.Spec.Replicas = oldReplicaCount
	_, updateErr = statefulsetClient.Update(context.TODO(), result, metav1.UpdateOptions{})
	if updateErr != nil {
		return fmt.Errorf("failed to scale up statefulset: %w", updateErr)
	}
	time.Sleep(60 * time.Second)

	return nil
}

func ListPods(namespace string, config *rest.Config) ([]v1.Pod, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating clientset: %w", err)
	}
	podsClient := clientset.CoreV1().Pods(namespace)

	result, getErr := podsClient.List(context.TODO(), metav1.ListOptions{})
	if getErr != nil {
		return nil, fmt.Errorf("failed to list pods: %w", getErr)
	}
	return result.Items, nil
}

func GetPodName(namespace string, config *rest.Config, deploymentName string, statefulSetName string) (string, error) {
	pods, err := ListPods(namespace, config)
	if err != nil {
		return "", err
	}
	for _, pod := range pods {
		if deploymentName != "" && strings.Contains(pod.Name, deploymentName) {
			return pod.Name, nil
		}
		if statefulSetName != "" && strings.Contains(pod.Name, statefulSetName) {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no pod found for deployment=%q statefulset=%q", deploymentName, statefulSetName)
}

type ImageRevision struct {
	Labels struct {
		Revision string `json:"org.opencontainers.image.revision"`
		Source   string `json:"org.opencontainers.image.source"`
	} `json:"Labels"`
}

func GetRevision(namespace string, config *rest.Config, podName string) (ImageRevision, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return ImageRevision{}, fmt.Errorf("creating clientset: %w", err)
	}
	podsClient := clientset.CoreV1().Pods(namespace)
	result, getErr := podsClient.Get(context.TODO(), podName, metav1.GetOptions{})
	if getErr != nil {
		return ImageRevision{}, fmt.Errorf("failed to get pod %s: %w", podName, getErr)
	}
	imageName := result.Spec.Containers[0].Image

	// Camunda images are linux/amd64 only; without these overrides skopeo on
	// macOS/arm64 fails with "no image found in manifest list for architecture".
	skopeoArgs := []string{"inspect", "--override-os", "linux", "--override-arch", "amd64", "docker://docker.io/" + imageName}
	var out strings.Builder
	retryMax := 3
	err = nil
	for i := 0; i < retryMax; i++ {
		out.Reset()
		cmd := exec.Command("skopeo", skopeoArgs...)
		cmd.Stdout = &out
		err = cmd.Run()
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return ImageRevision{}, fmt.Errorf("skopeo inspect failed for %s: %w", imageName, err)
	}

	var imageRevision ImageRevision
	if err := json.Unmarshal([]byte(out.String()), &imageRevision); err != nil {
		return ImageRevision{}, fmt.Errorf("unmarshalling image revision: %w", err)
	}
	return imageRevision, nil
}

func WaitUntilPodIsRunning(namespace string, config *rest.Config, podName string) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating clientset: %w", err)
	}
	podsClient := clientset.CoreV1().Pods(namespace)
	for {
		result, getErr := podsClient.Get(context.TODO(), podName, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get pod %s: %w", podName, getErr)
		}
		if result.Status.Phase == v1.PodRunning {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

func WaitUntilPodIsReady(namespace string, config *rest.Config, podName string) error {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("creating clientset: %w", err)
	}
	if err := WaitUntilPodIsRunning(namespace, config, podName); err != nil {
		return err
	}
	podsClient := clientset.CoreV1().Pods(namespace)
	// wait until pod is ready
	for {
		result, getErr := podsClient.Get(context.TODO(), podName, metav1.GetOptions{})
		if getErr != nil {
			return fmt.Errorf("failed to get pod %s: %w", podName, getErr)
		}
		ready := false
		for _, condition := range result.Status.Conditions {
			if condition.Type == v1.PodReady && condition.Status == v1.ConditionTrue {
				ready = true
				break
			}
		}
		if ready {
			break
		}
		time.Sleep(2 * time.Second)
	}
	return nil
}

func FetchConfigProps(podName string, managementPort int, contextPath string) error {
	managementPortStr := strconv.Itoa(managementPort)
	resp, err := http.Get("http://localhost:" + managementPortStr + "/actuator/configprops")
	if err != nil || resp.StatusCode != http.StatusOK {
		// try again but with contextPath
		resp, err = http.Get("http://localhost:" + managementPortStr + contextPath + "/actuator/configprops")
		if err != nil {
			return fmt.Errorf("fetching configprops: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("fetching configprops: unexpected status %d", resp.StatusCode)
		}
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	// write to configprops.json file
	file, err := os.Create("configprops-" + podName + ".json")
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("encoding configprops: %w", err)
	}
	return nil
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

// CleanupMain reverts a single component back to a debugger-free state. It
// removes the debug env vars (JAVA_TOOL_OPTIONS + the two configprops vars)
// and restarts the pod by scaling to 0 and back. It is idempotent — if no
// debug env vars are present, it logs a no-op and returns nil without
// disturbing the workload.
func CleanupMain(namespace string, config *rest.Config, c Component) error {
	if c.DeploymentName != "" {
		changed, err := RevertDeployment(namespace, config, c.DeploymentName)
		if err != nil {
			return fmt.Errorf("[%s] reverting deployment: %w", c.PodDisplayName, err)
		}
		if !changed {
			fmt.Printf("[%s] no debug env vars present — skipping\n", c.PodDisplayName)
			return nil
		}
		fmt.Printf("[%s] removed debug env vars and restarted pod\n", c.PodDisplayName)
		return nil
	}
	if c.StatefulSetName != "" {
		changed, err := RevertStatefulSet(namespace, config, c.StatefulSetName)
		if err != nil {
			return fmt.Errorf("[%s] reverting statefulset: %w", c.PodDisplayName, err)
		}
		if !changed {
			fmt.Printf("[%s] no debug env vars present — skipping\n", c.PodDisplayName)
			return nil
		}
		fmt.Printf("[%s] removed debug env vars and restarted pod\n", c.PodDisplayName)
		return nil
	}
	return fmt.Errorf("[%s] component has neither DeploymentName nor StatefulSetName", c.PodDisplayName)
}

// removeDebugEnvVars returns env with all entries whose name is in
// debugEnvVarNames removed. The bool return is true when at least one entry
// was removed.
func removeDebugEnvVars(env []v1.EnvVar) ([]v1.EnvVar, bool) {
	filtered := env[:0:0]
	removed := false
	for _, e := range env {
		if _, isDebug := debugEnvVarNames[e.Name]; isDebug {
			removed = true
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered, removed
}

func RevertDeployment(namespace string, config *rest.Config, name string) (bool, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, fmt.Errorf("creating clientset: %w", err)
	}
	client := clientset.AppsV1().Deployments(namespace)

	deployment, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get deployment %s: %w", name, err)
	}

	filtered, changed := removeDebugEnvVars(deployment.Spec.Template.Spec.Containers[0].Env)
	if !changed {
		return false, nil
	}
	deployment.Spec.Template.Spec.Containers[0].Env = filtered

	originalReplicas := deployment.Spec.Replicas
	zero := int32(0)
	deployment.Spec.Replicas = &zero
	if _, err := client.Update(context.TODO(), deployment, metav1.UpdateOptions{}); err != nil {
		return false, fmt.Errorf("scaling down deployment %s: %w", name, err)
	}

	time.Sleep(3 * time.Second)

	deployment, err = client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get deployment %s after scale-down: %w", name, err)
	}
	deployment.Spec.Replicas = originalReplicas
	if _, err := client.Update(context.TODO(), deployment, metav1.UpdateOptions{}); err != nil {
		return false, fmt.Errorf("scaling up deployment %s: %w", name, err)
	}
	time.Sleep(15 * time.Second)
	return true, nil
}

func RevertStatefulSet(namespace string, config *rest.Config, name string) (bool, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, fmt.Errorf("creating clientset: %w", err)
	}
	client := clientset.AppsV1().StatefulSets(namespace)

	sts, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get statefulset %s: %w", name, err)
	}

	filtered, changed := removeDebugEnvVars(sts.Spec.Template.Spec.Containers[0].Env)
	if !changed {
		return false, nil
	}
	sts.Spec.Template.Spec.Containers[0].Env = filtered

	originalReplicas := sts.Spec.Replicas
	zero := int32(0)
	sts.Spec.Replicas = &zero
	if _, err := client.Update(context.TODO(), sts, metav1.UpdateOptions{}); err != nil {
		return false, fmt.Errorf("scaling down statefulset %s: %w", name, err)
	}

	time.Sleep(3 * time.Second)

	sts, err = client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("get statefulset %s after scale-down: %w", name, err)
	}
	sts.Spec.Replicas = originalReplicas
	if _, err := client.Update(context.TODO(), sts, metav1.UpdateOptions{}); err != nil {
		return false, fmt.Errorf("scaling up statefulset %s: %w", name, err)
	}
	time.Sleep(60 * time.Second)
	return true, nil
}

func deleteConfigPropsFiles() error {
	matches, err := filepath.Glob("configprops-*.json")
	if err != nil {
		return err
	}
	for _, m := range matches {
		if err := os.Remove(m); err != nil {
			log.Printf("warning: removing %s: %v", m, err)
			continue
		}
		fmt.Printf("removed %s\n", m)
	}
	return nil
}
