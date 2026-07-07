import * as React from 'react';
import { useParams, useHistory } from 'react-router-dom';
import {
  Breadcrumb,
  BreadcrumbItem,
  Button,
  DescriptionList,
  DescriptionListDescription,
  DescriptionListGroup,
  DescriptionListTerm,
  Divider,
  Flex,
  FlexItem,
  PageSection,
  Spinner,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';
import StatusBadge from './StatusBadge';

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const ProjectDetailsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const history = useHistory();
  const [item, setItem] = React.useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    setLoading(true);
    const api = createAPIClient();
    api.projects.get(id)
      .then((data) => { setItem(data); setError(null); })
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, [id]);

  const handleDelete = async () => {
    if (!id) return;
    if (!window.confirm('Are you sure you want to delete this project? This will cascade delete all children.')) return;
    try {
      const api = createAPIClient();
      await api.projects.delete(id);
      history.push('/trex-console/projects');
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

  const handleActivate = async () => {
    if (!id) return;
    try {
      const api = createAPIClient();
      const updated = await api.projects.update(id, { status: 'active' });
      setItem(updated);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    }
  };

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

  if (!item) return null;

  const status = String(item.status ?? '');

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
          <BreadcrumbItem isActive>{String(item.name ?? id)}</BreadcrumbItem>
        </Breadcrumb>
        <Flex>
          <FlexItem>
            <Title headingLevel="h1">{String(item.name ?? 'Project Details')}</Title>
          </FlexItem>
          <FlexItem align={{ default: 'alignRight' }}>
            {status === 'draft' && (
              <Button variant="primary" onClick={handleActivate} style={{ marginRight: 8 }}>
                Activate
              </Button>
            )}
            {status === 'active' && (
              <Button
                variant="secondary"
                onClick={() => history.push(`/trex-console/builds/create?project_id=${id}`)}
                style={{ marginRight: 8 }}
              >
                Trigger Build
              </Button>
            )}
            <Button variant="danger" onClick={handleDelete}>Delete</Button>
          </FlexItem>
        </Flex>
      </PageSection>
      <Divider />
      <PageSection>
        <DescriptionList isHorizontal>
          <DescriptionListGroup>
            <DescriptionListTerm>ID</DescriptionListTerm>
            <DescriptionListDescription>{String(item.id)}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Name</DescriptionListTerm>
            <DescriptionListDescription>{String(item.name ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Status</DescriptionListTerm>
            <DescriptionListDescription>
              <StatusBadge status={status} type="project" />
            </DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Description</DescriptionListTerm>
            <DescriptionListDescription>{String(item.description ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Repository URL</DescriptionListTerm>
            <DescriptionListDescription>{String(item.repository_url ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Created</DescriptionListTerm>
            <DescriptionListDescription>{formatDate(item.created_at)}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Updated</DescriptionListTerm>
            <DescriptionListDescription>{formatDate(item.updated_at)}</DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </PageSection>
      <Divider />
      <PageSection>
        <Title headingLevel="h3" style={{ marginBottom: 16 }}>Resources</Title>
        <Flex>
          <FlexItem>
            <Button variant="secondary" onClick={() => history.push(`/trex-console/entity-definitions?project_id=${id}`)}>
              Entity Definitions
            </Button>
          </FlexItem>
          <FlexItem>
            <Button variant="secondary" onClick={() => history.push(`/trex-console/relationships?project_id=${id}`)}>
              Relationships
            </Button>
          </FlexItem>
          <FlexItem>
            <Button variant="secondary" onClick={() => history.push(`/trex-console/builds?project_id=${id}`)}>
              Builds
            </Button>
          </FlexItem>
        </Flex>
      </PageSection>
    </>
  );
};

export default ProjectDetailsPage;
