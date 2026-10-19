package fossils

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/api"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/db"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/components/api-server/pkg/services"
)

const fossilsLockType db.LockType = "fossils"

// FossilEventSource is the Source of every event about a fossil.
const FossilEventSource = "Fossils"

type FossilService interface {
	Get(ctx context.Context, id string) (*Fossil, *errors.ServiceError)
	Create(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError)
	Replace(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError)
	Patch(ctx context.Context, id string, patch *FossilPatchRequest) (*Fossil, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (FossilList, *errors.ServiceError)
	List(ctx context.Context, args *services.ListArguments) ([]Fossil, *api.PagingMeta, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (FossilList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewFossilService(lockFactory db.LockFactory, fossilDao FossilDao, events services.EventService, generic services.GenericService) FossilService {
	return &sqlFossilService{
		lockFactory: lockFactory,
		fossilDao:   fossilDao,
		events:      events,
		generic:     generic,
	}
}

var _ FossilService = &sqlFossilService{}

type sqlFossilService struct {
	lockFactory db.LockFactory
	fossilDao   FossilDao
	events      services.EventService
	generic     services.GenericService
}

// validateFossil checks the rules every stored fossil must satisfy.
// Create, Replace and Patch all call it, so REST and gRPC enforce the same rules.
func validateFossil(fossil *Fossil) *errors.ServiceError {
	if err := services.ValidateRequiredString("discovery_location", fossil.DiscoveryLocation); err != nil {
		return err
	}
	if err := services.ValidateOptionalString("fossil_type", fossil.FossilType); err != nil {
		return err
	}
	if err := services.ValidateOptionalString("excavator_name", fossil.ExcavatorName); err != nil {
		return err
	}
	return nil
}

// applyFossilPatch copies every field set in patch onto fossil.
func applyFossilPatch(fossil *Fossil, patch *FossilPatchRequest) {
	if patch.DiscoveryLocation != nil {
		fossil.DiscoveryLocation = *patch.DiscoveryLocation
	}
	if patch.EstimatedAge != nil {
		fossil.EstimatedAge = patch.EstimatedAge
	}
	if patch.FossilType != nil {
		fossil.FossilType = patch.FossilType
	}
	if patch.ExcavatorName != nil {
		fossil.ExcavatorName = patch.ExcavatorName
	}
}

func (s *sqlFossilService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)

	fossil, err := s.fossilDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this fossil: %s", fossil.ID)

	return nil
}

func (s *sqlFossilService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
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
	if svcErr := validateFossil(fossil); svcErr != nil {
		return nil, svcErr
	}

	fossil, err := s.fossilDao.Create(ctx, fossil)
	if err != nil {
		return nil, services.HandleCreateError("Fossil", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    FossilEventSource,
		SourceID:  fossil.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("Fossil", evErr)
	}

	return fossil, nil
}

func (s *sqlFossilService) Replace(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, fossil.ID, fossilsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	return s.update(ctx, fossil)
}

// Patch applies a partial update. The read, merge and write all happen under a
// row lock held until the request transaction commits, so concurrent patches to
// the same fossil cannot overwrite each other.
func (s *sqlFossilService) Patch(ctx context.Context, id string, patch *FossilPatchRequest) (*Fossil, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, id, fossilsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	fossil, err := s.fossilDao.GetForUpdate(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Fossil", "id", id, err)
	}

	applyFossilPatch(fossil, patch)
	return s.update(ctx, fossil)
}

// update validates and saves fossil, then records an update event.
// Callers must hold the fossil's lock.
func (s *sqlFossilService) update(ctx context.Context, fossil *Fossil) (*Fossil, *errors.ServiceError) {
	if svcErr := validateFossil(fossil); svcErr != nil {
		return nil, svcErr
	}

	fossil, err := s.fossilDao.Replace(ctx, fossil)
	if err != nil {
		return nil, services.HandleUpdateError("Fossil", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    FossilEventSource,
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
		Source:    FossilEventSource,
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

func (s *sqlFossilService) List(ctx context.Context, args *services.ListArguments) ([]Fossil, *api.PagingMeta, *errors.ServiceError) {
	var fossils []Fossil
	paging, err := s.generic.List(ctx, "id", args, &fossils)
	if err != nil {
		return nil, nil, err
	}
	return fossils, paging, nil
}
