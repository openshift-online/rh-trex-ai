import * as React from 'react';
import { Link, useHistory, useLocation } from 'react-router-dom';
import {
  Button,
  EmptyState,
  EmptyStateBody,
  EmptyStateIcon,
  PageSection,
  Pagination,
  SearchInput,
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
} from '@patternfly/react-core';
import { PlusCircleIcon, CubesIcon } from '@patternfly/react-icons';
import {
  Table,
  Thead,
  Tr,
  Th,
  Tbody,
  Td,
} from '@patternfly/react-table';
import { createAPIClient } from '../utils/api';
import StatusBadge from './StatusBadge';

type BuildRow = Record<string, unknown>;

const PAGE_SIZE = 20;

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const BuildListPage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const projectId = params.get('project_id') || undefined;

  const [items, setItems] = React.useState<BuildRow[]>([]);
  const [total, setTotal] = React.useState(0);
  const [page, setPage] = React.useState(1);
  const [loading, setLoading] = React.useState(true);
  const [error, setError] = React.useState<string | null>(null);
  const [search, setSearch] = React.useState('');

  const fetchData = React.useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const api = createAPIClient();
      const resp = await api.builds.list({ page, size: PAGE_SIZE, search: search || undefined, projectId });
      setItems(resp.items);
      setTotal(resp.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [page, search, projectId]);

  React.useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <>
      <PageSection variant="light">
        <Title headingLevel="h1">Builds{projectId ? ' (Project-Scoped)' : ''}</Title>
      </PageSection>
      <PageSection>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Search builds..."
                value={search}
                onChange={(_e, val) => setSearch(val)}
                onSearch={() => { setPage(1); fetchData(); }}
                onClear={() => { setSearch(''); setPage(1); }}
              />
            </ToolbarItem>
            <ToolbarItem>
              <Button
                variant="primary"
                icon={<PlusCircleIcon />}
                onClick={() => history.push(`/trex-console/builds/create${projectId ? `?project_id=${projectId}` : ''}`)}
              >
                Trigger Build
              </Button>
            </ToolbarItem>
            <ToolbarItem variant="pagination">
              <Pagination
                itemCount={total}
                perPage={PAGE_SIZE}
                page={page}
                onSetPage={(_e, p) => setPage(p)}
                isCompact
              />
            </ToolbarItem>
          </ToolbarContent>
        </Toolbar>

        {error && (
          <EmptyState>
            <EmptyStateBody>{error}</EmptyStateBody>
          </EmptyState>
        )}

        {!loading && !error && items.length === 0 && (
          <EmptyState>
            <EmptyStateIcon icon={CubesIcon} />
            <Title headingLevel="h4" size="lg">No builds found</Title>
            <EmptyStateBody>
              Trigger a build to get started.
            </EmptyStateBody>
            <Button
              variant="primary"
              onClick={() => history.push(`/trex-console/builds/create${projectId ? `?project_id=${projectId}` : ''}`)}
            >
              Trigger Build
            </Button>
          </EmptyState>
        )}

        {items.length > 0 && (
          <Table aria-label="Builds table" variant="compact">
            <Thead>
              <Tr>
                <Th>ID</Th>
                <Th>Status</Th>
                <Th>Project ID</Th>
                <Th>Triggered By</Th>
                <Th>Completed At</Th>
                <Th>Created</Th>
              </Tr>
            </Thead>
            <Tbody>
              {items.map((row) => (
                <Tr key={String(row.id)}>
                  <Td dataLabel="ID">
                    <Link to={`/trex-console/builds/${row.id}`}>
                      {String(row.id)}
                    </Link>
                  </Td>
                  <Td dataLabel="Status">
                    <StatusBadge status={String(row.status ?? '')} type="build" />
                  </Td>
                  <Td dataLabel="Project ID">
                    {String(row.project_id ?? '')}
                  </Td>
                  <Td dataLabel="Triggered By">
                    {String(row.triggered_by ?? '')}
                  </Td>
                  <Td dataLabel="Completed At">
                    {formatDate(row.completed_at)}
                  </Td>
                  <Td dataLabel="Created">
                    {formatDate(row.created_at)}
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        )}
      </PageSection>
    </>
  );
};

export default BuildListPage;
