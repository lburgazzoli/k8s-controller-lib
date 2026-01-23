package watch_test

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"

	"github.com/lburgazzoli/k8s-controller-lib/pkg/reconciler/watch"

	. "github.com/onsi/gomega"
)

func TestEnqueueRequestForOwnerOrLabel_OwnerReferences(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object (Pod as owner type)
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true, // only controller owner
	)

	// Create ConfigMap with OwnerReference
	controller := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-owner",
				Controller: &controller,
			}},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify request was enqueued
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("test-owner"))
	g.Expect(req.Namespace).To(Equal("default"))
}

func TestEnqueueRequestForOwnerOrLabel_LabelFallback(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object (Pod as owner type)
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with labels only (no OwnerReferences)
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				watch.LabelOwnerName:      "test-owner",
				watch.LabelOwnerNamespace: "default",
			},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify request was enqueued via labels
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("test-owner"))
	g.Expect(req.Namespace).To(Equal("default"))
}

func TestEnqueueRequestForOwnerOrLabel_OwnerReferencesPriority(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with BOTH OwnerReferences and labels
	controller := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "owner-from-ref",
				Controller: &controller,
			}},
			Labels: map[string]string{
				watch.LabelOwnerName:      "owner-from-label",
				watch.LabelOwnerNamespace: "default",
			},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify OwnerReference takes priority over labels
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("owner-from-ref"))
	g.Expect(req.Namespace).To(Equal("default"))
}

func TestEnqueueRequestForOwnerOrLabel_NoOwnerInfo(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap without owner info
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify no request was enqueued (graceful handling)
	g.Expect(q.Len()).To(Equal(0))
}

func TestEnqueueRequestForOwnerOrLabel_ControllerOwnerFilter(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler with isController=true
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true, // only controller owner
	)

	// Create ConfigMap with non-controller owner reference
	controller := false
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Pod",
				Name:       "test-owner",
				Controller: &controller, // Not a controller owner
			}},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify no request was enqueued (non-controller owner filtered out)
	g.Expect(q.Len()).To(Equal(0))
}

func TestEnqueueRequestForOwnerOrLabel_NamespaceDefaulting(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with label but NO namespace label
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "my-namespace",
			Labels: map[string]string{
				watch.LabelOwnerName: "test-owner",
				// LabelOwnerNamespace intentionally omitted
			},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify namespace defaults to object's namespace
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("test-owner"))
	g.Expect(req.Namespace).To(Equal("my-namespace"))
}

func TestEnqueueRequestForOwnerOrLabel_WrongGVK(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object (looking for Pod owners)
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with Service owner (wrong kind)
	controller := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Service", // Wrong kind
				Name:       "test-owner",
				Controller: &controller,
			}},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify no request was enqueued (wrong GVK)
	g.Expect(q.Len()).To(Equal(0))
}

func TestEnqueueRequestForOwnerOrLabel_UpdateEvent(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with labels
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				watch.LabelOwnerName: "test-owner",
			},
		},
	}

	// Create queue and trigger update event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedUpdateEvent[client.Object]{ObjectNew: cm}
	handler.Update(ctx, evt, q)

	// Verify request was enqueued
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("test-owner"))
}

func TestEnqueueRequestForOwnerOrLabel_DeleteEvent(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with labels
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				watch.LabelOwnerName: "test-owner",
			},
		},
	}

	// Create queue and trigger delete event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedDeleteEvent[client.Object]{Object: cm}
	handler.Delete(ctx, evt, q)

	// Verify request was enqueued
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("test-owner"))
}

func TestEnqueueRequestForOwnerOrLabel_GenericEvent(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with labels
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			Labels: map[string]string{
				watch.LabelOwnerName: "test-owner",
			},
		},
	}

	// Create queue and trigger generic event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedGenericEvent[client.Object]{Object: cm}
	handler.Generic(ctx, evt, q)

	// Verify request was enqueued
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("test-owner"))
}

func TestEnqueueRequestForOwnerOrLabel_NilObject(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create queue and trigger event with nil object
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: nil}
	handler.Create(ctx, evt, q)

	// Verify no request was enqueued (nil handled gracefully)
	g.Expect(q.Len()).To(Equal(0))
}

func TestEnqueueRequestForOwnerOrLabel_DifferentGroup(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object (looking for v1/Pod)
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with owner in different group
	controller := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "apps/v1", // Different group
				Kind:       "Pod",
				Name:       "test-owner",
				Controller: &controller,
			}},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify no request was enqueued (different group)
	g.Expect(q.Len()).To(Equal(0))
}

func TestEnqueueRequestForOwnerOrLabel_MultipleOwners(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create scheme and client
	s := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(s)).To(Succeed())

	// Create owner object (looking for Pod)
	owner := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
	}

	// Create handler
	handler := watch.EnqueueRequestForOwnerOrLabel(
		s,
		owner,
		true,
	)

	// Create ConfigMap with multiple owner references
	controller := true
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Service",
					Name:       "service-owner",
					Controller: &controller,
				},
				{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       "pod-owner", // This should match
					Controller: &controller,
				},
			},
		},
	}

	// Create queue and trigger event
	q := workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request]())
	defer q.ShutDown()

	evt := event.TypedCreateEvent[client.Object]{Object: cm}
	handler.Create(ctx, evt, q)

	// Verify the matching Pod owner was enqueued
	g.Expect(q.Len()).To(Equal(1))
	req, _ := q.Get()
	g.Expect(req.Name).To(Equal("pod-owner"))
}
