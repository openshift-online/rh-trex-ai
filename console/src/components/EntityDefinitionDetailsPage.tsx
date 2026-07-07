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

const EntityDefinitionDetailsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const history = useHistory();
  const [item, setItem] = React.useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    setLoading(true);
    const api = createAPIClient();
    api.entityDefinitions.get(id)
      .then((data) => { setItem(data); setError(null); })
      .catch((err) => setError(err instanceof Error ? err.message : String(err)))
      .finally(() => setLoading(false));
  }, [id]);

  const handleDelete = async () => {
    if (!id) return;
    if (!window.confirm('Delete this entity definition and all its field definitions?')) return;
    try {
      const api = createAPIClient();
      await api.entityDefinitions.delete(id);
      history.push('/trex-console/entity-definitions');
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
            onClick={() => history.push('/trex-console/entity-definitions')}
            component="button"
          >
            Entity Definitions
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{String(item.kind_name ?? id)}</BreadcrumbItem>
        </Breadcrumb>
        <Flex>
          <FlexItem>
            <Title headingLevel="h1">{String(item.kind_name ?? 'Entity Definition')}</Title>
          </FlexItem>
          <FlexItem align={{ default: 'alignRight' }}>
            <Button
              variant="secondary"
              onClick={() => history.push(`/trex-console/field-definitions?entity_definition_id=${id}`)}
              style={{ marginRight: 8 }}
            >
              View Fields
            </Button>
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
            <DescriptionListTerm>Kind Name</DescriptionListTerm>
            <DescriptionListDescription>{String(item.kind_name ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Description</DescriptionListTerm>
            <DescriptionListDescription>{String(item.description ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Plural Override</DescriptionListTerm>
            <DescriptionListDescription>{String(item.plural_override ?? '')}</DescriptionListDescription>
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

export default EntityDefinitionDetailsPage;
