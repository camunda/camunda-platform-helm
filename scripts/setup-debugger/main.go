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

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"golang.org/x/sync/errgroup"
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

func main() {
	var kubeconfig *string
	if home := homeDir(); home != "" {
		kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}
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

	g.Go(func() error {
		return FetchConfigPropsMain(FetchConfigPropsRequest{
			Namespace:           namespace,
			Config:              config,
			PodDisplayName:      "Identity",
			DeploymentName:      "integration-identity",
			ManagementPort:      8082,
			ContextPath:         "/identity",
			SkipJavaToolOptions: false,
			LocalDebugPort:      5009,
		})
	})

	g.Go(func() error {
		return FetchConfigPropsMain(FetchConfigPropsRequest{
			Namespace:           namespace,
			Config:              config,
			PodDisplayName:      "Optimize",
			DeploymentName:      "integration-optimize",
			ManagementPort:      8092,
			ContextPath:         "/optimize",
			SkipJavaToolOptions: false,
			LocalDebugPort:      5008,
		})
	})

	g.Go(func() error {
		return FetchConfigPropsMain(FetchConfigPropsRequest{
			Namespace:           namespace,
			Config:              config,
			PodDisplayName:      "Connectors",
			DeploymentName:      "integration-connectors",
			ManagementPort:      8080,
			ContextPath:         "/connectors",
			SkipJavaToolOptions: false,
			LocalDebugPort:      5007,
		})
	})

	g.Go(func() error {
		return FetchConfigPropsMain(FetchConfigPropsRequest{
			Namespace:           namespace,
			Config:              config,
			PodDisplayName:      "Zeebe",
			StatefulSetName:     "integration-zeebe",
			ManagementPort:      9600,
			ContextPath:         "/orchestration",
			SkipJavaToolOptions: false,
			LocalDebugPort:      5006,
		})
	})

//	modelerFetchReq := FetchConfigPropsRequest{
//		Namespace: namespace,
//		Config: config,
//		PodDisplayName: "Modeler",
//		DeploymentName: "integration-web-modeler-restapi",
//		ManagementPort: 8091,
//		ContextPath: "/modeler",
//		SkipJavaToolOptions: true,
//	}
//	FetchConfigPropsMain(modelerFetchReq)

	if err := g.Wait(); err != nil {
		log.Fatalf("Error: %v", err)
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
	revision, err := GetRevision(req.Namespace, req.Config, podName)
	if err != nil {
		return fmt.Errorf("[%s] getting revision: %w", req.PodDisplayName, err)
	}
	fmt.Printf(req.PodDisplayName+" pod name: %s\n", podName)
	fmt.Printf(req.PodDisplayName+" pod revision: %s\n", revision.Labels.Revision)
	fmt.Printf(req.PodDisplayName+" pod source: %s\n", revision.Labels.Source)

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

	go func() {
		if err := WaitUntilPodIsReady(req.Namespace, req.Config, podName); err != nil {
			log.Printf("[%s] waiting for pod ready: %v", req.PodDisplayName, err)
		} else if err := FetchConfigProps(podName, req.ManagementPort, req.ContextPath); err != nil {
			log.Printf("[%s] fetching config props: %v", req.PodDisplayName, err)
		}
		fmt.Println("Bye...")
		close(stopCh)
		wg.Done()
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

	cmd := exec.Command("skopeo", "inspect", "docker://docker.io/"+imageName)
	var out strings.Builder
	cmd.Stdout = &out
	retryMax := 20
	err = nil
	for i := 0; i < retryMax; i++ {
		cmd = exec.Command("skopeo", "inspect", "docker://docker.io/"+imageName)
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
