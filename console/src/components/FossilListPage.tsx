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

type FossilRow = Record<string, unknown>;

const PAGE_SIZE = 20;

function formatDate(value: unknown): string {
  if (!value) return '';
  const d = new Date(String(value));
  return isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}

function cellValue(row: FossilRow, key: string, fieldType: string): string {
  const val = row[key];
  if (val === null || val === undefined) return '';
  if (fieldType === 'date-time') return formatDate(val);
  return String(val);
}

const FossilListPage: React.FC = () => {
  const history = useHistory();
  const [items, setItems] = React.useState<FossilRow[]>([]);
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
      const resp = await api.fossils.list({ page, size: PAGE_SIZE, search: search || undefined });
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
        <Title headingLevel="h1">Fossils</Title>
      </PageSection>
      <PageSection>
        <Toolbar>
          <ToolbarContent>
            <ToolbarItem>
              <SearchInput
                placeholder="Search fossils..."
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
                onClick={() => history.push('/trex-console/fossils/create')}
              >
                Create Fossil
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
            <Title headingLevel="h4" size="lg">No fossils found</Title>
            <EmptyStateBody>
              Create a fossil to get started.
            </EmptyStateBody>
            <Button
              variant="primary"
              onClick={() => history.push('/trex-console/fossils/create')}
            >
              Create Fossil
            </Button>
          </EmptyState>
        )}

        {items.length > 0 && (
          <Table aria-label="Fossils table" variant="compact">
            <Thead>
              <Tr>
                <Th>ID</Th>
                <Th>Discovery Location</Th>
                <Th>Estimated Age</Th>
                <Th>Excavator Name</Th>
                <Th>Fossil Type</Th>
                <Th>Created</Th>
              </Tr>
            </Thead>
            <Tbody>
              {items.map((row) => (
                <Tr key={String(row.id)}>
                  <Td dataLabel="ID">
                    <Link to={`/trex-console/fossils/${row.id}`}>
                      {String(row.id)}
                    </Link>
                  </Td>
                  <Td dataLabel="Discovery Location">
                    {cellValue(row, 'discovery_location', 'string')}
                  </Td>
                  <Td dataLabel="Estimated Age">
                    {cellValue(row, 'estimated_age', 'int32')}
                  </Td>
                  <Td dataLabel="Excavator Name">
                    {cellValue(row, 'excavator_name', 'string')}
                  </Td>
                  <Td dataLabel="Fossil Type">
                    {cellValue(row, 'fossil_type', 'string')}
                  </Td>
                  <Td dataLabel="Created">
                    {cellValue(row, 'created_at', 'date-time')}
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

export default FossilListPage;
