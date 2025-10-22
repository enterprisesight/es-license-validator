package nodes

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Counter counts Kubernetes nodes matching a label selector
type Counter struct {
	clientset      *kubernetes.Clientset
	nodeLabelKey   string
	nodeLabelValue string
}

// NewCounter creates a new node counter
func NewCounter(nodeLabelKey, nodeLabelValue string) (*Counter, error) {
	// Create in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &Counter{
		clientset:      clientset,
		nodeLabelKey:   nodeLabelKey,
		nodeLabelValue: nodeLabelValue,
	}, nil
}

// CountLabeledNodes counts the number of nodes with the specified label
func (c *Counter) CountLabeledNodes(ctx context.Context) (int, error) {
	labelSelector := fmt.Sprintf("%s=%s", c.nodeLabelKey, c.nodeLabelValue)

	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list nodes: %w", err)
	}

	return len(nodes.Items), nil
}

// CountAllNodes counts all nodes in the cluster
func (c *Counter) CountAllNodes(ctx context.Context) (int, error) {
	nodes, err := c.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return 0, fmt.Errorf("failed to list all nodes: %w", err)
	}

	return len(nodes.Items), nil
}
