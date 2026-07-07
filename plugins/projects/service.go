package projects

import (
	"context"
	"fmt"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
	"github.com/openshift-online/rh-trex-ai/plugins/builds"
	"github.com/openshift-online/rh-trex-ai/plugins/entityDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/fieldDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/relationships"
)

var validProjectStatuses = map[string]bool{
	"draft":    true,
	"active":   true,
	"archived": true,
}

var allowedProjectTransitions = map[string]string{
	"draft":  "active",
	"active": "archived",
}

func ValidateProjectStatusTransition(from, to string) *errors.ServiceError {
	if !validProjectStatuses[to] {
		return errors.BadRequest(fmt.Sprintf("invalid project status: %s", to))
	}
	if from == to {
		return nil
	}
	if allowed, ok := allowedProjectTransitions[from]; !ok || allowed != to {
		return errors.BadRequest(fmt.Sprintf("invalid status transition: %s → %s", from, to))
	}
	return nil
}

const projectsLockType db.LockType = "projects"

type ProjectService interface {
	Get(ctx context.Context, id string) (*Project, *errors.ServiceError)
	Create(ctx context.Context, project *Project) (*Project, *errors.ServiceError)
	Replace(ctx context.Context, project *Project) (*Project, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (ProjectList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (ProjectList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewProjectService(
	lockFactory db.LockFactory,
	projectDao ProjectDao,
	events services.EventService,
	entityDefDao entityDefinitions.EntityDefinitionDao,
	fieldDefDao fieldDefinitions.FieldDefinitionDao,
	relDao relationships.RelationshipDao,
	buildDao builds.BuildDao,
) ProjectService {
	return &sqlProjectService{
		lockFactory:  lockFactory,
		projectDao:   projectDao,
		events:       events,
		entityDefDao: entityDefDao,
		fieldDefDao:  fieldDefDao,
		relDao:       relDao,
		buildDao:     buildDao,
	}
}

var _ ProjectService = &sqlProjectService{}

type sqlProjectService struct {
	lockFactory  db.LockFactory
	projectDao   ProjectDao
	events       services.EventService
	entityDefDao entityDefinitions.EntityDefinitionDao
	fieldDefDao  fieldDefinitions.FieldDefinitionDao
	relDao       relationships.RelationshipDao
	buildDao     builds.BuildDao
}

func (s *sqlProjectService) OnUpsert(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)

	project, err := s.projectDao.Get(ctx, id)
	if err != nil {
		return err
	}

	logger.Infof("Do idempotent somethings with this project: %s", project.ID)

	return nil
}

func (s *sqlProjectService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
	logger.Infof("This project has been deleted: %s", id)
	return nil
}

func (s *sqlProjectService) Get(ctx context.Context, id string) (*Project, *errors.ServiceError) {
	project, err := s.projectDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Project", "id", id, err)
	}
	return project, nil
}

func (s *sqlProjectService) Create(ctx context.Context, project *Project) (*Project, *errors.ServiceError) {
	if project.Status == "" {
		project.Status = "draft"
	}
	if !validProjectStatuses[project.Status] {
		return nil, errors.BadRequest(fmt.Sprintf("invalid project status: %s", project.Status))
	}
	project, err := s.projectDao.Create(ctx, project)
	if err != nil {
		return nil, services.HandleCreateError("Project", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Projects",
		SourceID:  project.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("Project", evErr)
	}

	return project, nil
}

func (s *sqlProjectService) Replace(ctx context.Context, project *Project) (*Project, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, project.ID, projectsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	existing, err := s.projectDao.Get(ctx, project.ID)
	if err != nil {
		return nil, services.HandleGetError("Project", "id", project.ID, err)
	}
	if project.Status != existing.Status {
		if svcErr := ValidateProjectStatusTransition(existing.Status, project.Status); svcErr != nil {
			return nil, svcErr
		}
	}

	project, err = s.projectDao.Replace(ctx, project)
	if err != nil {
		return nil, services.HandleUpdateError("Project", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Projects",
		SourceID:  project.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("Project", evErr)
	}

	return project, nil
}

func (s *sqlProjectService) Delete(ctx context.Context, id string) *errors.ServiceError {
	entityDefs, err := s.entityDefDao.FindByProjectID(ctx, id)
	if err != nil {
		return services.HandleDeleteError("Project", errors.GeneralError("Unable to list entity definitions for project: %s", err))
	}
	for _, ed := range entityDefs {
		fieldDefs, err := s.fieldDefDao.FindByEntityDefinitionID(ctx, ed.ID)
		if err != nil {
			return services.HandleDeleteError("Project", errors.GeneralError("Unable to list field definitions: %s", err))
		}
		for _, fd := range fieldDefs {
			if err := s.fieldDefDao.Delete(ctx, fd.ID); err != nil {
				return services.HandleDeleteError("Project", errors.GeneralError("Unable to delete field definition: %s", err))
			}
		}
		if err := s.entityDefDao.Delete(ctx, ed.ID); err != nil {
			return services.HandleDeleteError("Project", errors.GeneralError("Unable to delete entity definition: %s", err))
		}
	}

	rels, err := s.relDao.FindByProjectID(ctx, id)
	if err != nil {
		return services.HandleDeleteError("Project", errors.GeneralError("Unable to list relationships for project: %s", err))
	}
	for _, rel := range rels {
		if err := s.relDao.Delete(ctx, rel.ID); err != nil {
			return services.HandleDeleteError("Project", errors.GeneralError("Unable to delete relationship: %s", err))
		}
	}

	projectBuilds, err := s.buildDao.FindByProjectID(ctx, id)
	if err != nil {
		return services.HandleDeleteError("Project", errors.GeneralError("Unable to list builds for project: %s", err))
	}
	for _, b := range projectBuilds {
		if err := s.buildDao.Delete(ctx, b.ID); err != nil {
			return services.HandleDeleteError("Project", errors.GeneralError("Unable to delete build: %s", err))
		}
	}

	if err := s.projectDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("Project", errors.GeneralError("Unable to delete project: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Projects",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("Project", evErr)
	}

	return nil
}

func (s *sqlProjectService) FindByIDs(ctx context.Context, ids []string) (ProjectList, *errors.ServiceError) {
	projects, err := s.projectDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all projects: %s", err)
	}
	return projects, nil
}

func (s *sqlProjectService) All(ctx context.Context) (ProjectList, *errors.ServiceError) {
	projects, err := s.projectDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all projects: %s", err)
	}
	return projects, nil
}
