package fossils

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

const fossilsLockType db.LockType = "fossils"

var (
	DisableAdvisoryLock     = false
	UseBlockingAdvisoryLock = true
)

type FossilService interface {
	Get(ctx context.Context, id string) (*Fossil, *errors.ServiceError)
	Create(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError)
	Replace(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (FossilList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (FossilList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewFossilService(lockFactory db.LockFactory, fossilDao FossilDao, events services.EventService) FossilService {
	return &sqlFossilService{
		lockFactory: lockFactory,
		fossilDao:   fossilDao,
		events:      events,
	}
}

var _ FossilService = &sqlFossilService{}

type sqlFossilService struct {
	lockFactory db.LockFactory
	fossilDao   FossilDao
	events      services.EventService
}

func (s *sqlFossilService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	fossil, err := s.fossilDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this fossil: %s", fossil.ID)

	return nil
}

func (s *sqlFossilService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)
	logger.Infof("This fossil has been deleted: %s", id)
	return nil
}

func (s *sqlFossilService) Get(ctx context.Context, id string) (*Fossil, *errors.ServiceError) {
	fossil, err := s.fossilDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Fossil", "id", id, err)
	}
	return fossil, nil
}

func (s *sqlFossilService) Create(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError) {
	fossil, err := s.fossilDao.Create(ctx, fossil)
	if err != nil {
		return nil, services.HandleCreateError("Fossil", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Fossils",
		SourceID:  fossil.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("Fossil", evErr)
	}

	return fossil, nil
}

func (s *sqlFossilService) Replace(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError) {
	if !DisableAdvisoryLock {
		if UseBlockingAdvisoryLock {
			lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, fossil.ID, fossilsLockType)
			if err != nil {
				return nil, errors.DatabaseAdvisoryLock(err)
			}
			defer s.lockFactory.Unlock(ctx, lockOwnerID)
		} else {
			lockOwnerID, locked, err := s.lockFactory.NewNonBlockingLock(ctx, fossil.ID, fossilsLockType)
			if err != nil {
				return nil, errors.DatabaseAdvisoryLock(err)
			}
			if !locked {
				return nil, services.HandleCreateError("Fossil", errors.New(errors.ErrorConflict, "row locked"))
			}
			defer s.lockFactory.Unlock(ctx, lockOwnerID)
		}
	}

	fossil, err := s.fossilDao.Replace(ctx, fossil)
	if err != nil {
		return nil, services.HandleUpdateError("Fossil", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Fossils",
		SourceID:  fossil.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("Fossil", evErr)
	}

	return fossil, nil
}

func (s *sqlFossilService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.fossilDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("Fossil", errors.GeneralError("Unable to delete fossil: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Fossils",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("Fossil", evErr)
	}

	return nil
}

func (s *sqlFossilService) FindByIDs(ctx context.Context, ids []string) (FossilList, *errors.ServiceError) {
	fossils, err := s.fossilDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all fossils: %s", err)
	}
	return fossils, nil
}

func (s *sqlFossilService) All(ctx context.Context) (FossilList, *errors.ServiceError) {
	fossils, err := s.fossilDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all fossils: %s", err)
	}
	return fossils, nil
}
