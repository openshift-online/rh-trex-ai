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

type EntityDefinitionRow = Record<string, unknown>;

const PAGE_SIZE = 20;

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const EntityDefinitionListPage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const projectId = params.get('project_id') || undefined;

  const [items, setItems] = React.useState<EntityDefinitionRow[]>([]);
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
      const resp = await api.entityDefinitions.list({ page, size: PAGE_SIZE, search: search || undefined, projectId });
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
        <Title headingLevel="h1">Entity Definitions{projectId ? ' (Project-Scoped)' : ''}</Title>
      </PageSection>
      <PageSection>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Search entity definitions..."
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
                onClick={() => history.push(`/trex-console/entity-definitions/create${projectId ? `?project_id=${projectId}` : ''}`)}
              >
                Create Entity Definition
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
            <Title headingLevel="h4" size="lg">No entity definitions found</Title>
            <EmptyStateBody>
              Create an entity definition to get started.
            </EmptyStateBody>
            <Button
              variant="primary"
              onClick={() => history.push(`/trex-console/entity-definitions/create${projectId ? `?project_id=${projectId}` : ''}`)}
            >
              Create Entity Definition
            </Button>
          </EmptyState>
        )}

        {items.length > 0 && (
          <Table aria-label="Entity Definitions table" variant="compact">
            <Thead>
              <Tr>
                <Th>Kind Name</Th>
                <Th>Description</Th>
                <Th>Plural Override</Th>
                <Th>Project ID</Th>
                <Th>Created</Th>
              </Tr>
            </Thead>
            <Tbody>
              {items.map((row) => (
                <Tr key={String(row.id)}>
                  <Td dataLabel="Kind Name">
                    <Link to={`/trex-console/entity-definitions/${row.id}`}>
                      {String(row.kind_name ?? row.id)}
                    </Link>
                  </Td>
                  <Td dataLabel="Description">
                    {String(row.description ?? '')}
                  </Td>
                  <Td dataLabel="Plural Override">
                    {String(row.plural_override ?? '')}
                  </Td>
                  <Td dataLabel="Project ID">
                    {String(row.project_id ?? '')}
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

export default EntityDefinitionListPage;
