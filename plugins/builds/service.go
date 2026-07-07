package builds

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/openshift-online/rh-trex-ai/pkg/api"
	"github.com/openshift-online/rh-trex-ai/pkg/db"
	"github.com/openshift-online/rh-trex-ai/pkg/errors"
	"github.com/openshift-online/rh-trex-ai/pkg/logger"
	"github.com/openshift-online/rh-trex-ai/pkg/services"
	"github.com/openshift-online/rh-trex-ai/plugins/entityDefinitions"
	"github.com/openshift-online/rh-trex-ai/plugins/fieldDefinitions"
)

var validBuildStatuses = map[string]bool{
	"pending":   true,
	"building":  true,
	"succeeded": true,
	"failed":    true,
}

var allowedBuildTransitions = map[string][]string{
	"pending":  {"building"},
	"building": {"succeeded", "failed"},
}

func ValidateBuildStatusTransition(from, to string) *errors.ServiceError {
	if !validBuildStatuses[to] {
		return errors.BadRequest(fmt.Sprintf("invalid build status: %s", to))
	}
	if from == to {
		return nil
	}
	allowed, ok := allowedBuildTransitions[from]
	if !ok {
		return errors.BadRequest(fmt.Sprintf("invalid status transition: %s → %s (terminal state)", from, to))
	}
	for _, a := range allowed {
		if a == to {
			return nil
		}
	}
	return errors.BadRequest(fmt.Sprintf("invalid status transition: %s → %s", from, to))
}

const buildsLockType db.LockType = "builds"

type BuildService interface {
	Get(ctx context.Context, id string) (*Build, *errors.ServiceError)
	Create(ctx context.Context, build *Build) (*Build, *errors.ServiceError)
	Replace(ctx context.Context, build *Build) (*Build, *errors.ServiceError)
	Delete(ctx context.Context, id string) *errors.ServiceError
	All(ctx context.Context) (BuildList, *errors.ServiceError)

	FindByIDs(ctx context.Context, ids []string) (BuildList, *errors.ServiceError)

	OnUpsert(ctx context.Context, id string) error
	OnDelete(ctx context.Context, id string) error
}

func NewBuildService(
	lockFactory db.LockFactory,
	buildDao BuildDao,
	events services.EventService,
	entityDefDao entityDefinitions.EntityDefinitionDao,
	fieldDefDao fieldDefinitions.FieldDefinitionDao,
) BuildService {
	return &sqlBuildService{
		lockFactory:  lockFactory,
		buildDao:     buildDao,
		events:       events,
		entityDefDao: entityDefDao,
		fieldDefDao:  fieldDefDao,
	}
}

var _ BuildService = &sqlBuildService{}

type sqlBuildService struct {
	lockFactory  db.LockFactory
	buildDao     BuildDao
	events       services.EventService
	entityDefDao entityDefinitions.EntityDefinitionDao
	fieldDefDao  fieldDefinitions.FieldDefinitionDao
}

func (s *sqlBuildService) OnUpsert(ctx context.Context, id string) error {
	log := logger.NewLogger(ctx)

	build, err := s.buildDao.Get(ctx, id)
	if err != nil {
		return err
	}

	if build.Status != "pending" {
		log.Infof("Build %s is in status %s, skipping", build.ID, build.Status)
		return nil
	}

	build.Status = "building"
	build, err = s.buildDao.Replace(ctx, build)
	if err != nil {
		return fmt.Errorf("failed to transition build %s to building: %w", id, err)
	}

	buildLog, buildErr := s.executeBuild(ctx, build)

	now := time.Now()
	build.CompletedAt = &now
	build.BuildLog = &buildLog
	if buildErr != nil {
		build.Status = "failed"
		log.Error(fmt.Sprintf("Build %s failed: %s", build.ID, buildErr))
	} else {
		build.Status = "succeeded"
		log.Infof("Build %s succeeded", build.ID)
	}

	build, err = s.buildDao.Replace(ctx, build)
	if err != nil {
		return fmt.Errorf("failed to update build %s after execution: %w", id, err)
	}

	return nil
}

func (s *sqlBuildService) executeBuild(ctx context.Context, build *Build) (string, error) {
	entityDefs, err := s.entityDefDao.FindByProjectID(ctx, build.ProjectId)
	if err != nil {
		return "", fmt.Errorf("failed to fetch entity definitions: %w", err)
	}

	var logBuf strings.Builder

	for _, ed := range entityDefs {
		args := []string{"run", "./scripts/generator.go", "--kind", ed.KindName}

		if ed.PluralOverride != nil && *ed.PluralOverride != "" {
			args = append(args, "--plural", *ed.PluralOverride)
		}

		fieldDefs, err := s.fieldDefDao.FindByEntityDefinitionID(ctx, ed.ID)
		if err != nil {
			return logBuf.String(), fmt.Errorf("failed to fetch field definitions for %s: %w", ed.KindName, err)
		}

		if len(fieldDefs) > 0 {
			var fieldParts []string
			for _, fd := range fieldDefs {
				spec := fd.FieldName + ":" + fd.FieldType
				if fd.IsRequired != nil && *fd.IsRequired {
					spec += ":required"
				}
				fieldParts = append(fieldParts, spec)
			}
			args = append(args, "--fields", strings.Join(fieldParts, ","))
		}

		logBuf.WriteString(fmt.Sprintf("=== Generating %s ===\n", ed.KindName))
		logBuf.WriteString(fmt.Sprintf("Command: go %s\n", strings.Join(args, " ")))

		cmd := exec.CommandContext(ctx, "go", args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			logBuf.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout.String()))
			logBuf.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr.String()))
			logBuf.WriteString(fmt.Sprintf("ERROR: %s\n", err))
			return logBuf.String(), fmt.Errorf("generator failed for %s: %w", ed.KindName, err)
		}

		logBuf.WriteString(fmt.Sprintf("STDOUT:\n%s\n", stdout.String()))
		if stderr.Len() > 0 {
			logBuf.WriteString(fmt.Sprintf("STDERR:\n%s\n", stderr.String()))
		}
	}

	return logBuf.String(), nil
}

func (s *sqlBuildService) OnDelete(ctx context.Context, id string) error {
	logger := logger.NewLogger(ctx)
	logger.Infof("This build has been deleted: %s", id)
	return nil
}

func (s *sqlBuildService) Get(ctx context.Context, id string) (*Build, *errors.ServiceError) {
	build, err := s.buildDao.Get(ctx, id)
	if err != nil {
		return nil, services.HandleGetError("Build", "id", id, err)
	}
	return build, nil
}

func (s *sqlBuildService) Create(ctx context.Context, build *Build) (*Build, *errors.ServiceError) {
	if build.Status == "" {
		build.Status = "pending"
	}
	if build.Status != "pending" {
		return nil, errors.BadRequest("builds must be created with status 'pending'")
	}
	build, err := s.buildDao.Create(ctx, build)
	if err != nil {
		return nil, services.HandleCreateError("Build", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Builds",
		SourceID:  build.ID,
		EventType: api.CreateEventType,
	})
	if evErr != nil {
		return nil, services.HandleCreateError("Build", evErr)
	}

	return build, nil
}

func (s *sqlBuildService) Replace(ctx context.Context, build *Build) (*Build, *errors.ServiceError) {
	lockOwnerID, err := s.lockFactory.NewAdvisoryLock(ctx, build.ID, buildsLockType)
	if err != nil {
		return nil, errors.DatabaseAdvisoryLock(err)
	}
	defer s.lockFactory.Unlock(ctx, lockOwnerID)

	existing, err := s.buildDao.Get(ctx, build.ID)
	if err != nil {
		return nil, services.HandleGetError("Build", "id", build.ID, err)
	}
	if build.Status != existing.Status {
		if svcErr := ValidateBuildStatusTransition(existing.Status, build.Status); svcErr != nil {
			return nil, svcErr
		}
	}

	build, err = s.buildDao.Replace(ctx, build)
	if err != nil {
		return nil, services.HandleUpdateError("Build", err)
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Builds",
		SourceID:  build.ID,
		EventType: api.UpdateEventType,
	})
	if evErr != nil {
		return nil, services.HandleUpdateError("Build", evErr)
	}

	return build, nil
}

func (s *sqlBuildService) Delete(ctx context.Context, id string) *errors.ServiceError {
	if err := s.buildDao.Delete(ctx, id); err != nil {
		return services.HandleDeleteError("Build", errors.GeneralError("Unable to delete build: %s", err))
	}

	_, evErr := s.events.Create(ctx, &api.Event{
		Source:    "Builds",
		SourceID:  id,
		EventType: api.DeleteEventType,
	})
	if evErr != nil {
		return services.HandleDeleteError("Build", evErr)
	}

	return nil
}

func (s *sqlBuildService) FindByIDs(ctx context.Context, ids []string) (BuildList, *errors.ServiceError) {
	builds, err := s.buildDao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all builds: %s", err)
	}
	return builds, nil
}

func (s *sqlBuildService) All(ctx context.Context) (BuildList, *errors.ServiceError) {
	builds, err := s.buildDao.All(ctx)
	if err != nil {
		return nil, errors.GeneralError("Unable to get all builds: %s", err)
	}
	return builds, nil
}
