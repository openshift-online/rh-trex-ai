package scientists

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

const scientistsLockType db.LockType = "scientists"

// ScientistEventSource is the Source of every event about a scientist.
const ScientistEventSource = "Scientists"

type ScientistService interface {
	Get(ctx context.Context, id string) (*Scientist, *errors.ServiceError)
	Create(ctx context.Context, scientist *Scientist) (*Scientist, *errors.ServiceError)
	Replace(ctx context.Context, scientist *Scientist) (*Scientist, *errors.ServiceError)
	Patch(ctx context.Context, id string, patch *ScientistPatchRequest) (*Scientist, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (ScientistList, *errors.ServiceError)
	List(ctx context.Context, args *services.ListArguments) ([]Scientist, *api.PagingMeta, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (ScientistList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewScientistService(lockFactory db.LockFactory, scientistDao ScientistDao, events services.EventService, generic services.GenericService) ScientistService {
	return &sqlScientistService{
		lockFactory:  lockFactory,
		scientistDao: scientistDao,
		events:       events,
		generic:      generic,
	}
}

var _ ScientistService = &sqlScientistService{}

type sqlScientistService struct {
	lockFactory  db.LockFactory
	scientistDao ScientistDao
	events       services.EventService
	generic      services.GenericService
}

// validateScientist checks the rules every stored scientist must satisfy.
// Create, Replace and Patch all call it, so REST and gRPC enforce the same rules.
func validateScientist(scientist *Scientist) *errors.ServiceError {
	if err := services.ValidateRequiredString("name", scientist.Name); err != nil {
		return err
	}
	if err := services.ValidateRequiredString("field", scientist.Field); err != nil {
		return err
	}
	return nil
}

// applyScientistPatch copies every field set in patch onto scientist.
func applyScientistPatch(scientist *Scientist, patch *ScientistPatchRequest) {
	if patch.Name != nil {
		scientist.Name = *patch.Name
	}
	if patch.Field != nil {
		scientist.Field = *patch.Field
	}
}

func (s *sqlScientistService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)

	scientist, err := s.scientistDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this scientist: %s", scientist.ID)

	return nil
}

func (s *sqlScientistService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
	logger.Infof("This scientist has been deleted: %s", id)
	return nil
}

func (s *sqlScientistService) Get(ctx context.Context, id string) (*Scientist, *errors.ServiceError) {
	scientist, err := s.scientistDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Scientist", "id", id, err)
	}
	return scientist, nil
}

func (s *sqlScientistService) Create(ctx context.Context, scientist *Scientist) (*Scientist, *errors.ServiceError) {
	if svcErr := validateScientist(scientist); svcErr != nil {
		return nil, svcErr
	}

	scientist, err := s.scientistDao.Create(ctx, scientist)
	if err != nil {
		return nil, services.HandleCreateError("Scientist", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    ScientistEventSource,
		SourceID:  scientist.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("Scientist", evErr)
	}

	return scientist, nil
}

func (s *sqlScientistService) Replace(ctx context.Context, scientist *Scientist) (*Scientist, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, scientist.ID, scientistsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	return s.update(ctx, scientist)
}

// Patch applies a partial update. The read, merge and write all happen under a
// row lock held until the request transaction commits, so concurrent patches to
// the same scientist cannot overwrite each other.
func (s *sqlScientistService) Patch(ctx context.Context, id string, patch *ScientistPatchRequest) (*Scientist, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, id, scientistsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	scientist, err := s.scientistDao.GetForUpdate(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Scientist", "id", id, err)
	}

	applyScientistPatch(scientist, patch)
	return s.update(ctx, scientist)
}

// update validates and saves scientist, then records an update event.
// Callers must hold the scientist's lock.
func (s *sqlScientistService) update(ctx context.Context, scientist *Scientist) (*Scientist, *errors.ServiceError) {
	if svcErr := validateScientist(scientist); svcErr != nil {
		return nil, svcErr
	}

	scientist, err := s.scientistDao.Replace(ctx, scientist)
	if err != nil {
		return nil, services.HandleUpdateError("Scientist", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    ScientistEventSource,
		SourceID:  scientist.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("Scientist", evErr)
	}

	return scientist, nil
}

func (s *sqlScientistService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.scientistDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("Scientist", errors.GeneralError("Unable to delete scientist: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    ScientistEventSource,
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("Scientist", evErr)
	}

	return nil
}

func (s *sqlScientistService) FindByIDs(ctx context.Context, ids []string) (ScientistList, *errors.ServiceError) {
	scientists, err := s.scientistDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all scientists: %s", err)
	}
	return scientists, nil
}

func (s *sqlScientistService) All(ctx context.Context) (ScientistList, *errors.ServiceError) {
	scientists, err := s.scientistDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all scientists: %s", err)
	}
	return scientists, nil
}

func (s *sqlScientistService) List(ctx context.Context, args *services.ListArguments) ([]Scientist, *api.PagingMeta, *errors.ServiceError) {
	var scientists []Scientist
	paging, err := s.generic.List(ctx, "id", args, &scientists)
	if err != nil {
		return nil, nil, err
	}
	return scientists, paging, nil
}
