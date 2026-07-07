import * as React from 'react';
import { useParams, useHistory } from 'react-router-dom';
import {
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Card,
  CardBody,
  CardTitle,
  Divider,
  Flex,
  FlexItem,
  Gallery,
  GalleryItem,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';
import StatusBadge from './StatusBadge';

const ProjectDashboard: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const history = useHistory();
  const [project, setProject] = React.useState<Record<string, unknown> | null>(null);
  const [entityCount, setEntityCount] = React.useState<number>(0);
  const [relationshipCount, setRelationshipCount] = React.useState<number>(0);
  const [latestBuild, setLatestBuild] = React.useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    setLoading(true);
    const api = createAPIClient();

    Promise.all([
      api.projects.get(id),
      api.entityDefinitions.list({ page: 1, size: 1, projectId: id }),
      api.relationships.list({ page: 1, size: 1, projectId: id }),
      api.builds.list({ page: 1, size: 1, projectId: id }),
    ])
      .then(([proj, entities, rels, builds]) => {
        setProject(proj);
        setEntityCount(entities.total);
        setRelationshipCount(rels.total);
        setLatestBuild(builds.items.length > 0 ? builds.items[0] : null);
        setError(null);
      })
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, [id]);

  if (loading) {
    return (
      <PageSection>
        <Spinner size="lg" />
      </PageSection>
    );
  }

  if (error) {
    return (
      <PageSection>
        <Title headingLevel="h2">Error</Title>
        <p>{error}</p>
      </PageSection>
    );
  }

  if (!project) return null;

  const projectName = String(project.name ?? project.id);

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/projects')}
            component="button"
          >
            Projects
          </BreadcrumbItem>
          <BreadcrumbItem
            onClick={() => history.push(`/trex-console/projects/${id}`)}
            component="button"
          >
            {projectName}
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Dashboard</BreadcrumbItem>
        </Breadcrumb>
        <Flex>
          <FlexItem>
            <Title headingLevel="h1">{projectName} Dashboard</Title>
          </FlexItem>
          <FlexItem align={{ default: 'alignRight' }}>
            <StatusBadge status={String(project.status ?? '')} type="project" />
          </FlexItem>
        </Flex>
      </PageSection>
      <Divider />
      <PageSection>
        <Gallery hasGutter minWidths={{ default: '250px' }}>
          <GalleryItem>
            <Card isCompact>
              <CardTitle>Entity Definitions</CardTitle>
              <CardBody>
                <Title headingLevel="h2" size="4xl">{entityCount}</Title>
                <Button
                  variant="link"
                  isInline
                  onClick={() => history.push(`/trex-console/entity-definitions?project_id=${id}`)}
                >
                  View all
                </Button>
              </CardBody>
            </Card>
          </GalleryItem>
          <GalleryItem>
            <Card isCompact>
              <CardTitle>Relationships</CardTitle>
              <CardBody>
                <Title headingLevel="h2" size="4xl">{relationshipCount}</Title>
                <Button
                  variant="link"
                  isInline
                  onClick={() => history.push(`/trex-console/relationships?project_id=${id}`)}
                >
                  View all
                </Button>
              </CardBody>
            </Card>
          </GalleryItem>
          <GalleryItem>
            <Card isCompact>
              <CardTitle>Latest Build</CardTitle>
              <CardBody>
                {latestBuild ? (
                  <>
                    <StatusBadge status={String(latestBuild.status ?? '')} type="build" />
                    <div style={{ marginTop: 8 }}>
                      <Button
                        variant="link"
                        isInline
                        onClick={() => history.push(`/trex-console/builds/${latestBuild.id}`)}
                      >
                        View build
                      </Button>
                    </div>
                  </>
                ) : (
                  <>
                    <p>No builds yet</p>
                    <Button
                      variant="link"
                      isInline
                      onClick={() => history.push(`/trex-console/builds/create?project_id=${id}`)}
                    >
                      Trigger build
                    </Button>
                  </>
                )}
              </CardBody>
            </Card>
          </GalleryItem>
        </Gallery>
      </PageSection>
    </>
  );
};

export default ProjectDashboard;
