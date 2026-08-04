package reconciler

import (
	"context"
	"log"
	"sync"

	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"github.com/openshift-online/rh-trex-ai/components/control-plane/internal/watcher"
)

type DinosaurReconciler struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func NewDinosaurReconciler() *DinosaurReconciler {
	return &DinosaurReconciler{active: make(map[string]struct{})}
}

func (r *DinosaurReconciler) Handle(ctx context.Context, event watcher.Event[*pb.Dinosaur]) error {
	r.mu.Lock()
	if _, ok := r.active[event.ResourceID]; ok {
		r.mu.Unlock()
		return nil
	}
	r.active[event.ResourceID] = struct{}{}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.active, event.ResourceID)
		r.mu.Unlock()
	}()

	log.Printf("INFO reconciling Dinosaur %s (event=%d)", event.ResourceID, event.Type)
	return nil
}

type FossilReconciler struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func NewFossilReconciler() *FossilReconciler {
	return &FossilReconciler{active: make(map[string]struct{})}
}

func (r *FossilReconciler) Handle(ctx context.Context, event watcher.Event[*pb.Fossil]) error {
	r.mu.Lock()
	if _, ok := r.active[event.ResourceID]; ok {
		r.mu.Unlock()
		return nil
	}
	r.active[event.ResourceID] = struct{}{}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.active, event.ResourceID)
		r.mu.Unlock()
	}()

	log.Printf("INFO reconciling Fossil %s (event=%d)", event.ResourceID, event.Type)
	return nil
}

type ScientistReconciler struct {
	mu     sync.Mutex
	active map[string]struct{}
}

func NewScientistReconciler() *ScientistReconciler {
	return &ScientistReconciler{active: make(map[string]struct{})}
}

func (r *ScientistReconciler) Handle(ctx context.Context, event watcher.Event[*pb.Scientist]) error {
	r.mu.Lock()
	if _, ok := r.active[event.ResourceID]; ok {
		r.mu.Unlock()
		return nil
	}
	r.active[event.ResourceID] = struct{}{}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.active, event.ResourceID)
		r.mu.Unlock()
	}()

	log.Printf("INFO reconciling Scientist %s (event=%d)", event.ResourceID, event.Type)
	return nil
}
