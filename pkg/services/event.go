package services

import (
	"context"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/dao"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
)

type EventService interface {
	Get(ctx context.Context, id string) (*api.Event, *errors.ServiceError)
	Create(ctx context.Context, event *api.Event) (*api.Event, *errors.ServiceError)
	Replace(ctx context.Context, event *api.Event) (*api.Event, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (api.EventList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (api.EventList, *errors.ServiceError)
}

func NewEventService(eventDao dao.EventDao) EventService {
	return &sqlEventService{
		eventDao: eventDao,
	}
}

var _ EventService = &sqlEventService{}

type sqlEventService struct {
	eventDao dao.EventDao
}

func (s *sqlEventService) Get(ctx context.Context, id string) (*api.Event, *errors.ServiceError) {
	event, err := s.eventDao.Get(ctx, id)
	if err != nil {
		return nil, HandleGetError("Event", "id", id, err)
	}
	return event, nil
}

func (s *sqlEventService) Create(ctx context.Context, event *api.Event) (*api.Event, *errors.ServiceError) {
	event, err := s.eventDao.Create(ctx, event)
	if err != nil {
		return nil, HandleCreateError("Event", err)
	}
	return event, nil
}

func (s *sqlEventService) Replace(ctx context.Context, event *api.Event) (*api.Event, *errors.ServiceError) {
	event, err := s.eventDao.Replace(ctx, event)
	if err != nil {
		return nil, HandleUpdateError("Event", err)
	}
	return event, nil
}

func (s *sqlEventService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.eventDao.Delete(ctx, id); err != nil {
		return HandleDeleteError("Event", errors.GeneralError("Unable to delete event: %s", err))
	}
	return nil
}

func (s *sqlEventService) FindByIDs(ctx context.Context, ids []string) (api.EventList, *errors.ServiceError) {
	events, err := s.eventDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all events: %s", err)
	}
	return events, nil
}

func (s *sqlEventService) All(ctx context.Context) (api.EventList, *errors.ServiceError) {
	events, err := s.eventDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all events: %s", err)
	}
	return events, nil
}
