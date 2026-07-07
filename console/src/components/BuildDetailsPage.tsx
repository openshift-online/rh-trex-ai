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

const POLL_INTERVAL = 3000;

const BuildDetailsPage: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const history = useHistory();
  const [item, setItem] = React.useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const logRef = React.useRef<HTMLPreElement>(null);

  const fetchBuild = React.useCallback(async () => {
    if (!id) return;
    try {
      const api = createAPIClient();
      const data = await api.builds.get(id);
      setItem(data);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [id]);

  React.useEffect(() => { fetchBuild(); }, [fetchBuild]);

  React.useEffect(() => {
    if (!item) return;
    const status = String(item.status ?? '');
    if (status === 'succeeded' || status === 'failed') return;

    const interval = setInterval(fetchBuild, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [item, fetchBuild]);

  React.useEffect(() => {
    if (logRef.current) {
      logRef.current.scrollTop = logRef.current.scrollHeight;
    }
  }, [item]);

  const handleDelete = async () => {
    if (!id) return;
    if (!window.confirm('Are you sure you want to delete this build?')) return;
    try {
      const api = createAPIClient();
      await api.builds.delete(id);
      history.push('/trex-console/builds');
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
  const buildLog = String(item.build_log ?? '');
  const isActive = status === 'pending' || status === 'building';

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/builds')}
            component="button"
          >
            Builds
          </BreadcrumbItem>
          <BreadcrumbItem isActive>{id}</BreadcrumbItem>
        </Breadcrumb>
        <Flex>
          <FlexItem>
            <Title headingLevel="h1">Build Details</Title>
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
            <DescriptionListTerm>Status</DescriptionListTerm>
            <DescriptionListDescription>
              <StatusBadge status={status} type="build" />
              {isActive && <Spinner size="sm" style={{ marginLeft: 8 }} />}
            </DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Project ID</DescriptionListTerm>
            <DescriptionListDescription>{String(item.project_id ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Triggered By</DescriptionListTerm>
            <DescriptionListDescription>{String(item.triggered_by ?? '')}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Completed At</DescriptionListTerm>
            <DescriptionListDescription>{formatDate(item.completed_at)}</DescriptionListDescription>
          </DescriptionListGroup>
          <DescriptionListGroup>
            <DescriptionListTerm>Created</DescriptionListTerm>
            <DescriptionListDescription>{formatDate(item.created_at)}</DescriptionListDescription>
          </DescriptionListGroup>
        </DescriptionList>
      </PageSection>
      {buildLog && (
        <>
          <Divider />
          <PageSection>
            <Title headingLevel="h3" style={{ marginBottom: 8 }}>Build Log</Title>
            <pre
              ref={logRef}
              style={{
                background: '#1e1e1e',
                color: '#d4d4d4',
                padding: 16,
                borderRadius: 4,
                maxHeight: 500,
                overflow: 'auto',
                fontFamily: 'monospace',
                fontSize: 13,
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
              }}
            >
              {buildLog}
            </pre>
          </PageSection>
        </>
      )}
    </>
  );
};

export default BuildDetailsPage;
