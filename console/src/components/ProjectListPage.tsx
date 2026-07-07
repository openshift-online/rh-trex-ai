import * as React from 'react';
import { Link, useHistory } from 'react-router-dom';
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

type ProjectRow = Record<string, unknown>;

const PAGE_SIZE = 20;

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const ProjectListPage: React.FC = () => {
  const history = useHistory();
  const [items, setItems] = React.useState<ProjectRow[]>([]);
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
      const resp = await api.projects.list({ page, size: PAGE_SIZE, search: search || undefined });
      setItems(resp.items);
      setTotal(resp.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [page, search]);

  React.useEffect(() => { fetchData(); }, [fetchData]);

  return (
    <>
      <PageSection variant="light">
        <Title headingLevel="h1">Projects</Title>
      </PageSection>
      <PageSection>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Search projects..."
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
                onClick={() => history.push('/trex-console/projects/create')}
              >
                Create Project
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
            <Title headingLevel="h4" size="lg">No projects found</Title>
            <EmptyStateBody>
              Create a project to get started.
            </EmptyStateBody>
            <Button
              variant="primary"
              onClick={() => history.push('/trex-console/projects/create')}
            >
              Create Project
            </Button>
          </EmptyState>
        )}

        {items.length > 0 && (
          <Table aria-label="Projects table" variant="compact">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Status</Th>
                <Th>Description</Th>
                <Th>Repository URL</Th>
                <Th>Created</Th>
              </Tr>
            </Thead>
            <Tbody>
              {items.map((row) => (
                <Tr key={String(row.id)}>
                  <Td dataLabel="Name">
                    <Link to={`/trex-console/projects/${row.id}`}>
                      {String(row.name ?? row.id)}
                    </Link>
                  </Td>
                  <Td dataLabel="Status">
                    <StatusBadge status={String(row.status ?? '')} type="project" />
                  </Td>
                  <Td dataLabel="Description">
                    {String(row.description ?? '')}
                  </Td>
                  <Td dataLabel="Repository URL">
                    {String(row.repository_url ?? '')}
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

export default ProjectListPage;
