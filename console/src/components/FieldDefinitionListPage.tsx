import * as React from 'react';
import { Link, useHistory, useLocation } from 'react-router-dom';
import {
  Button,
  EmptyState,
  EmptyStateBody,
  EmptyStateIcon,
  Label,
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

type FieldDefinitionRow = Record<string, unknown>;

const PAGE_SIZE = 20;

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

const FieldDefinitionListPage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const entityDefinitionId = params.get('entity_definition_id') || undefined;

  const [items, setItems] = React.useState<FieldDefinitionRow[]>([]);
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
      const resp = await api.fieldDefinitions.list({ page, size: PAGE_SIZE, search: search || undefined, entityDefinitionId });
      setItems(resp.items);
      setTotal(resp.total);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, [page, search, entityDefinitionId]);

  React.useEffect(() => { fetchData(); }, [fetchData]);

  const createUrl = `/trex-console/field-definitions/create${entityDefinitionId ? `?entity_definition_id=${entityDefinitionId}` : ''}`;

  return (
    <>
      <PageSection variant="light">
        <Title headingLevel="h1">Field Definitions{entityDefinitionId ? ' (Entity-Scoped)' : ''}</Title>
      </PageSection>
      <PageSection>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Search field definitions..."
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
                Add Field
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
            <Title headingLevel="h4" size="lg">No field definitions found</Title>
            <EmptyStateBody>
              Add a field definition to get started.
            </EmptyStateBody>
            <Button
              variant="primary"
              onClick={() => history.push(createUrl)}
            >
              Add Field
            </Button>
          </EmptyState>
        )}

        {items.length > 0 && (
          <Table aria-label="Field Definitions table" variant="compact">
            <Thead>
              <Tr>
                <Th>Field Name</Th>
                <Th>Field Type</Th>
                <Th>Required</Th>
                <Th>Entity Definition</Th>
                <Th>Created</Th>
              </Tr>
            </Thead>
            <Tbody>
              {items.map((row) => (
                <Tr key={String(row.id)}>
                  <Td dataLabel="Field Name">
                    <Link to={`/trex-console/field-definitions/${row.id}`}>
                      {String(row.field_name ?? row.id)}
                    </Link>
                  </Td>
                  <Td dataLabel="Field Type">
                    <Label color="blue">{String(row.field_type ?? '')}</Label>
                  </Td>
                  <Td dataLabel="Required">
                    {row.is_required ? 'Yes' : 'No'}
                  </Td>
                  <Td dataLabel="Entity Definition">
                    {String(row.entity_definition_id ?? '').substring(0, 8)}...
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

export default FieldDefinitionListPage;
