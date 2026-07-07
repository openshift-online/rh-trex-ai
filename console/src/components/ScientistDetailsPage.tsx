import * as React from 'react';
import { useParams, useHistory } from 'react-router-dom';
import {
  Breadcrumb,
  BreadcrumbItem,
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

const ScientistDetailsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const history = useHistory();
  const [item, setItem] = React.useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!id) return;
    setLoading(true);
    const api = createAPIClient();
    api.scientists.get(id)
      .then((data) => { setItem(data); setError(null); })
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

  if (!item) return null;

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/scientists')}
            component="button"
          >
            Scientists
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{id}</BreadcrumbItem>
        </Breadcrumb>
        <Flex>
          <FlexItem>
            <Title headingLevel="h1">Scientist Details</Title>
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
            <DescriptionListTerm>Field</DescriptionListTerm>
            <DescriptionListDescription>
              {String(item['field'] ?? '')}
            </DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Name</DescriptionListTerm>
            <DescriptionListDescription>
              {String(item['name'] ?? '')}
            </DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Created</DescriptionListTerm>
            <DescriptionListDescription>
              {formatDate(item['created_at'])}
            </DescriptionListDescription>
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

export default ScientistDetailsPage;
