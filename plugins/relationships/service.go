package relationships

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

const relationshipsLockType db.LockType = "relationships"

type RelationshipService interface {
	Get(ctx context.Context, id string) (*Relationship, *errors.ServiceError)
	Create(ctx context.Context, relationship *Relationship) (*Relationship, *errors.ServiceError)
	Replace(ctx context.Context, relationship *Relationship) (*Relationship, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (RelationshipList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (RelationshipList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewRelationshipService(lockFactory db.LockFactory, relationshipDao RelationshipDao, events services.EventService) RelationshipService {
	return &sqlRelationshipService{
		lockFactory:     lockFactory,
		relationshipDao: relationshipDao,
		events:          events,
	}
}

var _ RelationshipService = &sqlRelationshipService{}

type sqlRelationshipService struct {
	lockFactory     db.LockFactory
	relationshipDao RelationshipDao
	events          services.EventService
}

func (s *sqlRelationshipService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)

	relationship, err := s.relationshipDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this relationship: %s", relationship.ID)

	return nil
}

func (s *sqlRelationshipService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
	logger.Infof("This relationship has been deleted: %s", id)
	return nil
}

func (s *sqlRelationshipService) Get(ctx context.Context, id string) (*Relationship, *errors.ServiceError) {
	relationship, err := s.relationshipDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Relationship", "id", id, err)
	}
	return relationship, nil
}

func (s *sqlRelationshipService) Create(ctx context.Context, relationship *Relationship) (*Relationship, *errors.ServiceError) {
	relationship, err := s.relationshipDao.Create(ctx, relationship)
	if err != nil {
		return nil, services.HandleCreateError("Relationship", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Relationships",
		SourceID:  relationship.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("Relationship", evErr)
	}

	return relationship, nil
}

func (s *sqlRelationshipService) Replace(ctx context.Context, relationship *Relationship) (*Relationship, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, relationship.ID, relationshipsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	relationship, err = s.relationshipDao.Replace(ctx, relationship)
	if err != nil {
		return nil, services.HandleUpdateError("Relationship", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Relationships",
		SourceID:  relationship.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("Relationship", evErr)
	}

	return relationship, nil
}

func (s *sqlRelationshipService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.relationshipDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("Relationship", errors.GeneralError("Unable to delete relationship: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Relationships",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("Relationship", evErr)
	}

	return nil
}

func (s *sqlRelationshipService) FindByIDs(ctx context.Context, ids []string) (RelationshipList, *errors.ServiceError) {
	relationships, err := s.relationshipDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all relationships: %s", err)
	}
	return relationships, nil
}

func (s *sqlRelationshipService) All(ctx context.Context) (RelationshipList, *errors.ServiceError) {
	relationships, err := s.relationshipDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all relationships: %s", err)
	}
	return relationships, nil
}
