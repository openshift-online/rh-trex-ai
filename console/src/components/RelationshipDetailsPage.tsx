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

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const RelationshipDetailsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const history = useHistory();
  const [item, setItem] = React.useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    setLoading(true);
    const api = createAPIClient();
    api.relationships.get(id)
      .then((data) => { setItem(data); setError(null); })
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, [id]);

  const handleDelete = async () => {
    if (!id) return;
    if (!window.confirm('Delete this relationship?')) return;
    try {
      const api = createAPIClient();
      await api.relationships.delete(id);
      history.push('/trex-console/relationships');
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

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/relationships')}
            component="button"
          >
            Relationships
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{String(item.relationship_type ?? id)}</BreadcrumbItem>
        </Breadcrumb>
        <Flex>
          <FlexItem>
            <Title headingLevel="h1">Relationship Details</Title>
          </FlexItem>
          <FlexItem align={{ default: 'alignRight' }}>
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
            <DescriptionListTerm>Relationship Type</DescriptionListTerm>
            <DescriptionListDescription>{String(item.relationship_type ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Source Entity ID</DescriptionListTerm>
            <DescriptionListDescription>{String(item.source_entity_id ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Target Entity ID</DescriptionListTerm>
            <DescriptionListDescription>{String(item.target_entity_id ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Foreign Key</DescriptionListTerm>
            <DescriptionListDescription>{String(item.foreign_key ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Project ID</DescriptionListTerm>
            <DescriptionListDescription>{String(item.project_id ?? '')}</DescriptionListDescription>
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
    </>
  );
};

export default RelationshipDetailsPage;
