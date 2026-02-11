package dinosaurs

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
)

const dinosaursLockType db.LockType = "dinosaurs"

var (
	DisableAdvisoryLock     = false
	UseBlockingAdvisoryLock = true
)

type DinosaurService interface {
	Get(ctx context.Context, id string) (*Dinosaur, *errors.ServiceError)
	Create(ctx context.Context, dinosaur *Dinosaur) (*Dinosaur, *errors.ServiceError)
	Replace(ctx context.Context, dinosaur *Dinosaur) (*Dinosaur, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (DinosaurList, *errors.ServiceError)

	FindBySpecies(ctx context.Context, species string) (DinosaurList, *errors.ServiceError)
	FindByIDs(ctx context.Context, ids []string) (DinosaurList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewDinosaurService(lockFactory db.LockFactory, dinosaurDao DinosaurDao, events services.EventService) DinosaurService {
	return &sqlDinosaurService{
		lockFactory: lockFactory,
		dinosaurDao: dinosaurDao,
		events:      events,
	}
}

var _ DinosaurService = &sqlDinosaurService{}

type sqlDinosaurService struct {
	lockFactory db.LockFactory
	dinosaurDao DinosaurDao
	events      services.EventService
}

func (s *sqlDinosaurService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)

	dinosaur, err := s.dinosaurDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this dinosaur: %s", dinosaur.ID)

	return nil
}

func (s *sqlDinosaurService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewOCMLogger(ctx)
	logger.Infof("This dino didn't make it to the asteroid: %s", id)
	return nil
}

func (s *sqlDinosaurService) Get(ctx context.Context, id string) (*Dinosaur, *errors.ServiceError) {
	dinosaur, err := s.dinosaurDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Dinosaur", "id", id, err)
	}
	return dinosaur, nil
}

func (s *sqlDinosaurService) Create(ctx context.Context, dinosaur *Dinosaur) (*Dinosaur, *errors.ServiceError) {
	dinosaur, err := s.dinosaurDao.Create(ctx, dinosaur)
	if err != nil {
		return nil, services.HandleCreateError("Dinosaur", err)
	}

	_, eErr := s.events.Create(ctx, &api.Event{
		Source:    "Dinosaurs",
		SourceID:  dinosaur.ID,
		EventType: api.CreateEventType,
	})
	if eErr != nil {
		return nil, services.HandleCreateError("Dinosaur", eErr)
	}

	return dinosaur, nil
}

func (s *sqlDinosaurService) Replace(ctx context.Context, dinosaur *Dinosaur) (*Dinosaur, *errors.ServiceError) {
	if !DisableAdvisoryLock {
		if UseBlockingAdvisoryLock {
			lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, dinosaur.ID, dinosaursLockType)
			if err != nil {
				return nil, errors.DatabaseAdvisoryLock(err)
			}
			defer s.lockFactory.Unlock(ctx, lockOwnerID)

		} else {
			lockOwnerID, locked, err := s.lockFactory.NewNonBlockingLock(ctx, dinosaur.ID, dinosaursLockType)
			if err != nil {
				return nil, errors.DatabaseAdvisoryLock(err)
			}
			if !locked {
				return nil, services.HandleUpdateError("Dinosaur", errors.New(errors.ErrorConflict, "row locked"))
			}
			defer s.lockFactory.Unlock(ctx, lockOwnerID)
		}
	}

	found, err := s.dinosaurDao.Get(ctx, dinosaur.ID)
	if err != nil {
		return nil, services.HandleGetError("Dinosaur", "id", dinosaur.ID, err)
	}


	if found.Species == dinosaur.Species {
		return found, nil
	}

	found.Species = dinosaur.Species
	updated, err := s.dinosaurDao.Replace(ctx, found)
	if err != nil {
		return nil, services.HandleUpdateError("Dinosaur", err)
	}

	_, eErr := s.events.Create(ctx, &api.Event{
		Source:    "Dinosaurs",
		SourceID:  updated.ID,
		EventType: api.UpdateEventType,
	})
	if eErr != nil {
		return nil, services.HandleUpdateError("Dinosaur", eErr)
	}
	return updated, nil
}

func (s *sqlDinosaurService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.dinosaurDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("Dinosaur", errors.GeneralError("Unable to delete dinosaur: %s", err))
	}

	_, err := s.events.Create(ctx, &api.Event{
		Source:    "Dinosaurs",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if err != nil {
		return services.HandleDeleteError("Dinosaur", err)
	}

	return nil
}

func (s *sqlDinosaurService) FindByIDs(ctx context.Context, ids []string) (DinosaurList, *errors.ServiceError) {
	dinosaurs, err := s.dinosaurDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all dinosaurs: %s", err)
	}
	return dinosaurs, nil
}

func (s *sqlDinosaurService) FindBySpecies(ctx context.Context, species string) (DinosaurList, *errors.ServiceError) {
	dinosaurs, err := s.dinosaurDao.FindBySpecies(ctx, species)
	if err != nil {
		return nil, services.HandleGetError("Dinosaur", "species", species, err)
	}
	return dinosaurs, nil
}

func (s *sqlDinosaurService) All(ctx context.Context) (DinosaurList, *errors.ServiceError) {
	dinosaurs, err := s.dinosaurDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all dinosaurs: %s", err)
	}
	return dinosaurs, nil
}
