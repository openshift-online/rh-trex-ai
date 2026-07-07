package fieldDefinitions

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

const fieldDefinitionsLockType db.LockType = "field_definitions"

type FieldDefinitionService interface {
	Get(ctx context.Context, id string) (*FieldDefinition, *errors.ServiceError)
	Create(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, *errors.ServiceError)
	Replace(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (FieldDefinitionList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (FieldDefinitionList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewFieldDefinitionService(lockFactory db.LockFactory, fieldDefinitionDao FieldDefinitionDao, events services.EventService) FieldDefinitionService {
	return &sqlFieldDefinitionService{
		lockFactory:        lockFactory,
		fieldDefinitionDao: fieldDefinitionDao,
		events:             events,
	}
}

var _ FieldDefinitionService = &sqlFieldDefinitionService{}

type sqlFieldDefinitionService struct {
	lockFactory        db.LockFactory
	fieldDefinitionDao FieldDefinitionDao
	events             services.EventService
}

func (s *sqlFieldDefinitionService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)

	fieldDefinition, err := s.fieldDefinitionDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this fieldDefinition: %s", fieldDefinition.ID)

	return nil
}

func (s *sqlFieldDefinitionService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
	logger.Infof("This fieldDefinition has been deleted: %s", id)
	return nil
}

func (s *sqlFieldDefinitionService) Get(ctx context.Context, id string) (*FieldDefinition, *errors.ServiceError) {
	fieldDefinition, err := s.fieldDefinitionDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("FieldDefinition", "id", id, err)
	}
	return fieldDefinition, nil
}

func (s *sqlFieldDefinitionService) Create(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, *errors.ServiceError) {
	fieldDefinition, err := s.fieldDefinitionDao.Create(ctx, fieldDefinition)
	if err != nil {
		return nil, services.HandleCreateError("FieldDefinition", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "FieldDefinitions",
		SourceID:  fieldDefinition.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("FieldDefinition", evErr)
	}

	return fieldDefinition, nil
}

func (s *sqlFieldDefinitionService) Replace(ctx context.Context, fieldDefinition *FieldDefinition) (*FieldDefinition, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, fieldDefinition.ID, fieldDefinitionsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	fieldDefinition, err = s.fieldDefinitionDao.Replace(ctx, fieldDefinition)
	if err != nil {
		return nil, services.HandleUpdateError("FieldDefinition", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "FieldDefinitions",
		SourceID:  fieldDefinition.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("FieldDefinition", evErr)
	}

	return fieldDefinition, nil
}

func (s *sqlFieldDefinitionService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.fieldDefinitionDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("FieldDefinition", errors.GeneralError("Unable to delete fieldDefinition: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "FieldDefinitions",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("FieldDefinition", evErr)
	}

	return nil
}

func (s *sqlFieldDefinitionService) FindByIDs(ctx context.Context, ids []string) (FieldDefinitionList, *errors.ServiceError) {
	fieldDefinitions, err := s.fieldDefinitionDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all fieldDefinitions: %s", err)
	}
	return fieldDefinitions, nil
}

func (s *sqlFieldDefinitionService) All(ctx context.Context) (FieldDefinitionList, *errors.ServiceError) {
	fieldDefinitions, err := s.fieldDefinitionDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all fieldDefinitions: %s", err)
	}
	return fieldDefinitions, nil
}
