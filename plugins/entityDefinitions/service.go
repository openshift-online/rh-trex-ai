package entityDefinitions

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
	"github.com/openshift-online/rh-trex-ai/plugins/fieldDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/relationships"
)

const entityDefinitionsLockType db.LockType = "entity_definitions"

type EntityDefinitionService interface {
	Get(ctx context.Context, id string) (*EntityDefinition, *errors.ServiceError)
	Create(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, *errors.ServiceError)
	Replace(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (EntityDefinitionList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (EntityDefinitionList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewEntityDefinitionService(
	lockFactory db.LockFactory,
	entityDefinitionDao EntityDefinitionDao,
	events services.EventService,
	fieldDefDao fieldDefinitions.FieldDefinitionDao,
	relDao relationships.RelationshipDao,
) EntityDefinitionService {
	return &sqlEntityDefinitionService{
		lockFactory:         lockFactory,
		entityDefinitionDao: entityDefinitionDao,
		events:              events,
		fieldDefDao:         fieldDefDao,
		relDao:              relDao,
	}
}

var _ EntityDefinitionService = &sqlEntityDefinitionService{}

type sqlEntityDefinitionService struct {
	lockFactory         db.LockFactory
	entityDefinitionDao EntityDefinitionDao
	events              services.EventService
	fieldDefDao         fieldDefinitions.FieldDefinitionDao
	relDao              relationships.RelationshipDao
}

func (s *sqlEntityDefinitionService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)

	entityDefinition, err := s.entityDefinitionDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this entityDefinition: %s", entityDefinition.ID)

	return nil
}

func (s *sqlEntityDefinitionService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
	logger.Infof("This entityDefinition has been deleted: %s", id)
	return nil
}

func (s *sqlEntityDefinitionService) Get(ctx context.Context, id string) (*EntityDefinition, *errors.ServiceError) {
	entityDefinition, err := s.entityDefinitionDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("EntityDefinition", "id", id, err)
	}
	return entityDefinition, nil
}

func (s *sqlEntityDefinitionService) Create(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, *errors.ServiceError) {
	entityDefinition, err := s.entityDefinitionDao.Create(ctx, entityDefinition)
	if err != nil {
		return nil, services.HandleCreateError("EntityDefinition", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "EntityDefinitions",
		SourceID:  entityDefinition.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("EntityDefinition", evErr)
	}

	return entityDefinition, nil
}

func (s *sqlEntityDefinitionService) Replace(ctx context.Context, entityDefinition *EntityDefinition) (*EntityDefinition, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, entityDefinition.ID, entityDefinitionsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	entityDefinition, err = s.entityDefinitionDao.Replace(ctx, entityDefinition)
	if err != nil {
		return nil, services.HandleUpdateError("EntityDefinition", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "EntityDefinitions",
		SourceID:  entityDefinition.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("EntityDefinition", evErr)
	}

	return entityDefinition, nil
}

func (s *sqlEntityDefinitionService) Delete(ctx context.Context, id string) *errors.ServiceError {
	fieldDefs, err := s.fieldDefDao.FindByEntityDefinitionID(ctx, id)
	if err != nil {
		return services.HandleDeleteError("EntityDefinition", errors.GeneralError("Unable to list field definitions: %s", err))
	}
	for _, fd := range fieldDefs {
		if err := s.fieldDefDao.Delete(ctx, fd.ID); err != nil {
			return services.HandleDeleteError("EntityDefinition", errors.GeneralError("Unable to delete field definition: %s", err))
		}
	}

	rels, err := s.relDao.FindByEntityID(ctx, id)
	if err != nil {
		return services.HandleDeleteError("EntityDefinition", errors.GeneralError("Unable to list relationships: %s", err))
	}
	for _, rel := range rels {
		if err := s.relDao.Delete(ctx, rel.ID); err != nil {
			return services.HandleDeleteError("EntityDefinition", errors.GeneralError("Unable to delete relationship: %s", err))
		}
	}

	if err := s.entityDefinitionDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("EntityDefinition", errors.GeneralError("Unable to delete entityDefinition: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "EntityDefinitions",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("EntityDefinition", evErr)
	}

	return nil
}

func (s *sqlEntityDefinitionService) FindByIDs(ctx context.Context, ids []string) (EntityDefinitionList, *errors.ServiceError) {
	entityDefinitions, err := s.entityDefinitionDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all entityDefinitions: %s", err)
	}
	return entityDefinitions, nil
}

func (s *sqlEntityDefinitionService) All(ctx context.Context) (EntityDefinitionList, *errors.ServiceError) {
	entityDefinitions, err := s.entityDefinitionDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all entityDefinitions: %s", err)
	}
	return entityDefinitions, nil
}
