// Copyright © 2021 The Knative Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Component versions are generated at buildtime via the hack/build.sh script
const ServingVersion = `1.16.0`
const KourierVersion = `1.16.0`
const EventingVersion = `1.16.0`

// Kourier installs Kourier networking layer from Github YAML files
func Kourier() error {
	fmt.Println("🕸️ Installing Kourier networking layer v" + KourierVersion + " ...")

	if err := retryingApply("https://files.m.daocloud.io/github.com/knative-sandbox/net-kourier/releases/download/knative-v" + KourierVersion + "/kourier.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}
	if err := waitForPodsReady("kourier-system"); err != nil {
		return fmt.Errorf("kourier: %w", err)
	}
	if err := waitForPodsReady("knative-serving"); err != nil {
		return fmt.Errorf("serving: %w", err)
	}
	fmt.Println("    Kourier installed...")

	ingress := exec.Command("kubectl", "patch", "configmap/config-network", "--namespace", "knative-serving", "--type", "merge", "--patch", "{\"data\":{\"ingress.class\":\"kourier.ingress.networking.knative.dev\"}}")
	if err := runCommand(ingress); err != nil {
		return fmt.Errorf("ingress error: %w", err)
	}
	fmt.Println("    Ingress patched...")

	fmt.Println("    Finished installing Kourier Networking layer")

	return nil
}

// KourierKind runs the kind-specific setup for Kourier
func KourierKind() error {
	fmt.Println("🕸️ Configuring Kourier for Kind...")

	config := `apiVersion: v1
kind: Service
metadata:
  name: kourier-ingress
  namespace: kourier-system
  labels:
    networking.knative.dev/ingress-provider: kourier
spec:
  type: NodePort
  selector:
    app: 3scale-kourier-gateway
  ports:
    - name: http2
      nodePort: 31080
      port: 80
      targetPort: 8080`

	kourierIngress := exec.Command("kubectl", "apply", "-f", "-")
	kourierIngress.Stdin = strings.NewReader(config)
	if err := runCommand(kourierIngress); err != nil {
		return fmt.Errorf("kourier service: %w", err)
	}

	fmt.Println("    Kourier service installed...")

	domainDns := exec.Command("kubectl", "patch", "configmap", "-n", "knative-serving", "config-domain", "-p", "{\"data\": {\"127.0.0.1.sslip.io\": \"\"}}")
	if err := runCommand(domainDns); err != nil {
		return fmt.Errorf("domain dns: %w", err)
	}
	fmt.Println("    Domain DNS set up...")
	fmt.Println("    Finished configuring Kourier")

	return nil
}

// KourierMinikube runs the minikube-specific setup for Kourier
func KourierMinikube() error {
	fmt.Println("🕸️ Configuring Kourier for Minikube...")

	if err := retryingApply("https://files.m.daocloud.io/github.com/knative/serving/releases/download/knative-v" + ServingVersion + "/serving-default-domain.yaml"); err != nil {
		return fmt.Errorf("default domain: %w", err)
	}
	if err := waitForPodsReady("knative-serving"); err != nil {
		return fmt.Errorf("core: %w", err)
	}

	fmt.Println("    Domain DNS set up...")

	fmt.Println("    Finished configuring Kourier")
	return nil
}

// Serving installs Knative Serving from Github YAML files
func Serving(registries string) error {
	fmt.Println("🍿 Installing Knative Serving v" + ServingVersion + " ...")
	baseURL := "https://files.m.daocloud.io/github.com/knative/serving/releases/download/knative-v" + ServingVersion

	if err := retryingApply(baseURL + "/serving-crds.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if err := waitForCRDsEstablished(); err != nil {
		return fmt.Errorf("crds: %w", err)
	}
	fmt.Println("    CRDs installed...")

	if err := retryingApply(baseURL + "/serving-core.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if err := waitForPodsReady("knative-serving"); err != nil {
		return fmt.Errorf("core: %w", err)
	}

	fmt.Println("    Core installed...")

	if registries != "" {
		configPatch := fmt.Sprintf(`{"data":{"registries-skipping-tag-resolving":"%s"}}`, registries)
		ignoreRegistry := exec.Command("kubectl", "patch", "configmap", "-n", "knative-serving", "config-deployment", "-p", configPatch)
		if err := runCommand(ignoreRegistry); err != nil {
			return fmt.Errorf("tag resolving configuration: %w", err)
		}
		fmt.Println("    Enabled local registry deployment...")
	}

	fmt.Println("    Finished installing Knative Serving")

	return nil
}

// Eventing installs Knative Eventing from Github YAML files
func Eventing() error {
	fmt.Println("🔥 Installing Knative Eventing v" + EventingVersion + " ... ")
	baseURL := "https://files.m.daocloud.io/github.com/knative/eventing/releases/download/knative-v" + EventingVersion

	if err := retryingApply(baseURL + "/eventing-crds.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if err := waitForCRDsEstablished(); err != nil {
		return fmt.Errorf("crds: %w", err)
	}
	fmt.Println("    CRDs installed...")

	if err := retryingApply(baseURL + "/eventing-core.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if err := waitForPodsReady("knative-eventing"); err != nil {
		return fmt.Errorf("core: %w", err)
	}
	fmt.Println("    Core installed...")

	if err := retryingApply(baseURL + "/in-memory-channel.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if err := waitForPodsReady("knative-eventing"); err != nil {
		return fmt.Errorf("channel: %w", err)
	}
	fmt.Println("    In-memory channel installed...")

	if err := retryingApply(baseURL + "/mt-channel-broker.yaml"); err != nil {
		return fmt.Errorf("wait: %w", err)
	}

	if err := waitForPodsReady("knative-eventing"); err != nil {
		return fmt.Errorf("broker: %w", err)
	}
	fmt.Println("    Mt-channel broker installed...")

	config := `apiVersion: eventing.knative.dev/v1
kind: broker
metadata:
 name: example-broker
 namespace: default`

	exampleBroker := exec.Command("kubectl", "apply", "-f", "-")
	exampleBroker.Stdin = strings.NewReader(config)
	if err := runCommand(exampleBroker); err != nil {
		return fmt.Errorf("example broker: %w", err)
	}

	fmt.Println("    Example broker installed...")
	fmt.Println("    Finished installing Knative Eventing")

	return nil
}

func CamelK(registryAddress string) error {
	fmt.Println("🐪 Installing Apache Camel K  ... ")

	if err := addHelmRepo(); err != nil {
		fmt.Printf("Error adding Helm repo: %v\n", err)
		return err
	}
	fmt.Println("Helm repo added successfully")

	// Run the Helm install command
	if err := runHelmInstall(registryAddress); err != nil {
		fmt.Printf("Error: %v\n", err)
		return err
	}

	if err := waitForCRDsEstablished(); err != nil {
		return fmt.Errorf("crds: %w", err)
	}
	fmt.Println("    CRDs installed...")

	if err := waitForPodsReady("default"); err != nil {
		return fmt.Errorf("core: %w", err)
	}
	fmt.Println("    Apache Camel K installed...")

	return nil
}

func runCommand(c *exec.Cmd) error {
	if out, err := c.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return err
	}
	return nil
}

// retryingApply retries a kubectl apply call with the given path 3 times, sleeping
// for 10s between each try.
func retryingApply(path string) error {
	fmt.Println(path)
	cmd := exec.Command("kubectl", "apply", "-f", path)
	var err error
	for i := 0; i < 3; i++ {
		err = runCommand(cmd)
		if err == nil {
			break
		}
		time.Sleep(10 * time.Second)
	}
	return err
}

// waitForCRDsEstablished waits for all CRDs to be established.
func waitForCRDsEstablished() error {
	return runCommand(exec.Command("kubectl", "wait", "--for=condition=Established", "--all", "crd"))
}

// waitForPodsReady waits for all pods in the given namespace to be ready.
func waitForPodsReady(ns string) error {
	return runCommand(exec.Command("kubectl", "wait", "pod", "--timeout=10m", "--for=condition=Ready", "-l", "!job-name", "-n", ns))
}

//nolint:gosec // avoid linter warnings
func runHelmInstall(registryAddress string) error {

	// Check if helm CLI is installed
	if _, err := exec.LookPath("helm"); err != nil {
		return fmt.Errorf("Please install helm CLI")
	}

	cmd := exec.Command("helm", "install",
		"--generate-name",
		"--set", fmt.Sprintf("platform.build.registry.address=%s", registryAddress),
		"--set", "platform.build.registry.insecure=true",
		"camel-k/camel-k")

	cmd.Stdout = cmd.Stderr

	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("failed to run helm install: %w", err)
	}
	return nil
}

func addHelmRepo() error {
	cmd := exec.Command("helm", "repo", "add", "camel-k", "https://apache.github.io/camel-k/charts/")
	cmd.Stdout = cmd.Stderr

	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Println(string(out))
		return fmt.Errorf("failed to add helm repo: %w", err)
	}
	return nil
}
