//nolint:testpackage // White-box tests verify option copy behavior.
package hierarchical

import (
	"testing"

	"github.com/go-logr/logr/testr"
	"sigs.k8s.io/controller-runtime/pkg/cache"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/gomega"
)

func TestClusterOptions(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	logger := testr.New(t)
	selector := labels.SelectorFromSet(labels.Set{"app": "test"})
	configMap := &corev1.ConfigMap{}

	opts := (&ClusterOptions{}).ApplyOptions([]ClusterOption{
		&ClusterOptions{Name: "preset", Namespaces: []string{"one"}},
		WithName("child"),
		WithScheme(scheme),
		WithLogger(logger),
		WithNamespaces("two", "three"),
		WithByObject(configMap, cache.ByObject{Label: selector}),
	})

	g.Expect(opts.Name).To(Equal("child"))
	g.Expect(opts.Scheme).To(BeIdenticalTo(scheme))
	g.Expect(opts.Logger.GetSink()).ToNot(BeNil())
	g.Expect(opts.Namespaces).To(Equal([]string{"one", "two", "three"}))
	g.Expect(opts.ByObject).To(HaveLen(1))
	g.Expect(opts.ByObject[configMap].Label.String()).To(Equal("app=test"))
}

func TestClusterOptionsCloneByObjectNamespaces(t *testing.T) {
	g := NewWithT(t)
	namespaces := map[string]cache.Config{"one": {}}
	byObject := cache.ByObject{Namespaces: namespaces}
	opts := &ClusterOptions{}
	configMap := &corev1.ConfigMap{}

	WithByObject(configMap, byObject).ApplyTo(opts)
	namespaces["two"] = cache.Config{}

	g.Expect(opts.ByObject[configMap].Namespaces).To(HaveKey("one"))
	g.Expect(opts.ByObject[configMap].Namespaces).ToNot(HaveKey("two"))
}

func TestNewClusterRequiresParent(t *testing.T) {
	g := NewWithT(t)

	_, err := NewCluster(nil, WithName("child"))
	g.Expect(err).To(MatchError("parent manager is required"))
}

func TestClusterOptionsValidate(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()

	g.Expect((*ClusterOptions)(nil).Validate()).To(MatchError("child cluster options are required"))
	g.Expect((&ClusterOptions{Scheme: scheme}).Validate()).To(MatchError("child cluster name is required"))
	g.Expect((&ClusterOptions{Name: "child"}).Validate()).To(MatchError("child cluster scheme is required"))
	g.Expect((&ClusterOptions{
		Name: "child", Scheme: scheme, Namespaces: []string{"valid", ""},
	}).Validate()).To(MatchError("child cache namespace must not be empty"))
	g.Expect((&ClusterOptions{Name: "child", Scheme: scheme}).Validate()).To(Succeed())
}
