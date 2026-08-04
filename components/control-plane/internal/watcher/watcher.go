package watcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	pb "github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api/grpc/rh_trex/v1"
	"google.golang.org/grpc"
)

type EventType int

const (
	EventCreated EventType = iota
	EventUpdated
	EventDeleted
)

type Event[T any] struct {
	Type       EventType
	ResourceID string
	Resource   T
}

type Handler[T any] interface {
	Handle(ctx context.Context, event Event[T]) error
}

func toEventType(t pb.EventType) EventType {
	switch t {
	case pb.EventType_EVENT_TYPE_CREATED:
		return EventCreated
	case pb.EventType_EVENT_TYPE_UPDATED:
		return EventUpdated
	case pb.EventType_EVENT_TYPE_DELETED:
		return EventDeleted
	default:
		return EventCreated
	}
}

func WatchDinosaurs(ctx context.Context, conn *grpc.ClientConn, handler Handler[*pb.Dinosaur]) error {
	client := pb.NewDinosaurServiceClient(conn)
	return watchLoop(ctx, "Dinosaur", func(ctx context.Context) error {
		stream, err := client.WatchDinosaurs(ctx, &pb.WatchDinosaursRequest{})
		if err != nil {
			return fmt.Errorf("starting dinosaur watch: %w", err)
		}
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("receiving dinosaur event: %w", err)
			}
			if err := handler.Handle(ctx, Event[*pb.Dinosaur]{
				Type:       toEventType(event.Type),
				ResourceID: event.ResourceId,
				Resource:   event.Dinosaur,
			}); err != nil {
				log.Printf("ERROR handling dinosaur %s: %v", event.ResourceId, err)
			}
		}
	})
}

func WatchFossils(ctx context.Context, conn *grpc.ClientConn, handler Handler[*pb.Fossil]) error {
	client := pb.NewFossilServiceClient(conn)
	return watchLoop(ctx, "Fossil", func(ctx context.Context) error {
		stream, err := client.WatchFossils(ctx, &pb.WatchFossilsRequest{})
		if err != nil {
			return fmt.Errorf("starting fossil watch: %w", err)
		}
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("receiving fossil event: %w", err)
			}
			if err := handler.Handle(ctx, Event[*pb.Fossil]{
				Type:       toEventType(event.Type),
				ResourceID: event.ResourceId,
				Resource:   event.Fossil,
			}); err != nil {
				log.Printf("ERROR handling fossil %s: %v", event.ResourceId, err)
			}
		}
	})
}

func WatchScientists(ctx context.Context, conn *grpc.ClientConn, handler Handler[*pb.Scientist]) error {
	client := pb.NewScientistServiceClient(conn)
	return watchLoop(ctx, "Scientist", func(ctx context.Context) error {
		stream, err := client.WatchScientists(ctx, &pb.WatchScientistsRequest{})
		if err != nil {
			return fmt.Errorf("starting scientist watch: %w", err)
		}
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("receiving scientist event: %w", err)
			}
			if err := handler.Handle(ctx, Event[*pb.Scientist]{
				Type:       toEventType(event.Type),
				ResourceID: event.ResourceId,
				Resource:   event.Scientist,
			}); err != nil {
				log.Printf("ERROR handling scientist %s: %v", event.ResourceId, err)
			}
		}
	})
}

func watchLoop(ctx context.Context, kind string, connectAndRecv func(ctx context.Context) error) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		log.Printf("INFO connecting %s watch stream...", kind)
		err := connectAndRecv(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		log.Printf("WARN %s watch stream disconnected: %v; reconnecting in %v", kind, err, backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}
