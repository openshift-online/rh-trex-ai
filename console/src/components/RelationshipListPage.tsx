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

type RelationshipRow = Record<string, unknown>;

const PAGE_SIZE = 20;

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const RelationshipListPage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const projectId = params.get('project_id') || undefined;

  const [items, setItems] = React.useState<RelationshipRow[]>([]);
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
      const resp = await api.relationships.list({ page, size: PAGE_SIZE, search: search || undefined, projectId });
      setItems(resp.items);
      setTotal(resp.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [page, search, projectId]);

  React.useEffect(() => { fetchData(); }, [fetchData]);

  const createUrl = `/trex-console/relationships/create${projectId ? `?project_id=${projectId}` : ''}`;

  return (
    <>
      <PageSection variant="light">
        <Title headingLevel="h1">Relationships{projectId ? ' (Project-Scoped)' : ''}</Title>
      </PageSection>
      <PageSection>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Search relationships..."
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
                onClick={() => history.push(createUrl)}
              >
                Create Relationship
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
            <Title headingLevel="h4" size="lg">No relationships found</Title>
            <EmptyStateBody>
              Create a relationship to get started.
            </EmptyStateBody>
            <Button
              variant="primary"
              onClick={() => history.push(createUrl)}
            >
              Create Relationship
            </Button>
          </EmptyState>
        )}

        {items.length > 0 && (
          <Table aria-label="Relationships table" variant="compact">
            <Thead>
              <Tr>
                <Th>Relationship Type</Th>
                <Th>Source Entity</Th>
                <Th>Target Entity</Th>
                <Th>Foreign Key</Th>
                <Th>Created</Th>
              </Tr>
            </Thead>
            <Tbody>
              {items.map((row) => (
                <Tr key={String(row.id)}>
                  <Td dataLabel="Relationship Type">
                    <Link to={`/trex-console/relationships/${row.id}`}>
                      {String(row.relationship_type ?? '')}
                    </Link>
                  </Td>
                  <Td dataLabel="Source Entity">
                    {String(row.source_entity_id ?? '').substring(0, 8)}...
                  </Td>
                  <Td dataLabel="Target Entity">
                    {String(row.target_entity_id ?? '').substring(0, 8)}...
                  </Td>
                  <Td dataLabel="Foreign Key">
                    {String(row.foreign_key ?? '')}
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

export default RelationshipListPage;
